package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/notify"
	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type paneFocus int

const (
	paneLists paneFocus = iota
	paneTasks
)

type appView int

const (
	viewTasks appView = iota
	viewCalendar
	viewPomodoro
	viewHabits
)

type mode int

const (
	modeNormal mode = iota
	modeAddTask
	modeEditTask
	modeAddPomoForm
	modeFilter
	modeRenameList
	modeRenameTask
	modeRenamePomo
	modeRenameHabit
	modeFocusPicker
	modeListPicker
	modeAddList
	modeAddFolder
)

// Run starts the interactive TickTick browser.
func Run(client *ticktick.Client) error {
	m := newModel(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type listRow struct {
	node       ticktick.ProjectTreeNode
	selectable bool
}

type model struct {
	client *ticktick.Client

	width, height int
	view          appView
	showHelp      bool
	showKeyHints  bool
	helpCursor    int
	helpFilter    string

	paneFocus paneFocus
	mode      mode

	tree       []ticktick.ProjectTreeNode
	listRows   []listRow
	listCursor int

	tasks         []ticktick.Task
	taskCursor    int
	taskMarked    map[string]struct{}
	showCompleted bool
	showDeleted   bool

	projectID   string
	projectName string
	inboxID     string

	addInput    textinput.Model
	taskTitleInput textinput.Model
	addTaskDueInput textinput.Model
	addTaskTimeInput textinput.Model
	addTaskDurationInput textinput.Model
	addTaskNotesInput    textarea.Model
	addTaskField int
	addTaskReminderIdx int
	addTaskPriorityIdx int
	editTaskID         string
	filterInput textinput.Model
	renameInput textinput.Model

	addListFolder string // folder context when creating a list

	listPickerPurpose listPickerPurpose
	listPickerRows []listPickerRow
	listPickerCursor int
	listPickerTaskIDs     []string
	listPickerFromProject string
	listPickerListRef string

	calDate       time.Time
	calMode       calMode
	calTaskCursor int
	calGridCursor int
	calTasks      []ticktick.Task

	focusStats *ticktick.FocusStats
	taskFocusByID    map[string]ticktick.TaskFocusSummary
	taskFocusByTitle map[string]ticktick.TaskFocusSummary
	pomoCursor int
	pomoGridCursor int
	pomoLegendCursor int
	pomoScrollToNow  bool
	pomoFollowNow    bool
	pomoNowTick      time.Time
	pomoViewDate     time.Time
	habits     []ticktick.Habit
	habitCursor int
	habitCheckedToday map[string]bool
	todayPomos  int
	todayCompleted []ticktick.Task

	focusNotifySent      bool
	focusNotifyEscalated bool
	focusPlannedLogged   bool
	showFocusAlert       bool
	focusAlertDismissed  bool
	focusAlertTitle      string
	focusAlertChord      bool
	focusAlertSince      time.Time

	focusPickerTasks        []ticktick.Task
	focusPickerProjectNames map[string]string
	focusPickerFilter       string
	focusPickerCursor       int
	focusPickerMinutes      int
	focusPickerLoading      bool
	focusPickerEditDuration bool
	focusPickerDurationBuf  string
	focusPickerSwitch       bool

	addPomoField            int
	addPomoLogDate          time.Time
	addPomoStartUnset       bool
	addPomoStartMinutes     int
	addPomoEditPause        bool
	addPomoPauseInput       textinput.Model

	taskSortMode TaskSortMode
	uiSettings   uiSettings

	focusTrackedStart time.Time

	loading bool
	toast   string
	errMsg  string
}

func newModel(client *ticktick.Client) model {
	add := textinput.New()
	add.Placeholder = "new task title…"
	add.CharLimit = 500
	add.Prompt = iconAdd + " "
	add.PromptStyle = inputPromptStyle
	add.TextStyle = inputStyle

	filter := textinput.New()
	filter.Placeholder = "filter tasks…"
	filter.CharLimit = 80
	filter.Prompt = iconFilter + " "
	filter.PromptStyle = inputPromptStyle
	filter.TextStyle = inputStyle

	rename := textinput.New()
	rename.CharLimit = 120
	rename.Prompt = iconEdit + " "
	rename.PromptStyle = inputPromptStyle
	rename.TextStyle = inputStyle

	addDue := newAddTaskFieldInput("YYYY-MM-DD")
	addTime := newAddTaskFieldInput("HH:MM")
	addDur := newAddTaskFieldInput("minutes")
	addNotes := newAddTaskNotesInput()
	taskTitle := newAddTaskFieldInput("task title…")
	taskTitle.CharLimit = 500
	addPomoPause := newAddTaskFieldInput("minutes · empty = none")

	now := time.Now()
	settings := loadUISettings()
	startView := parseAppView(settings.LastView)
	if settings.TaskDetailLayout == "" {
		settings.TaskDetailLayout = TaskDetailBottom
	}
	m := model{
		client:      client,
		view:        startView,
		paneFocus:   paneLists,
		uiSettings:  settings,
		addInput:    add,
		taskTitleInput: taskTitle,
		addTaskDueInput: addDue,
		addTaskTimeInput: addTime,
		addTaskDurationInput: addDur,
		addTaskNotesInput:    addNotes,
		addPomoPauseInput: addPomoPause,
		filterInput: filter,
		renameInput: rename,
		calDate:     dateOnly(now),
		calMode:     calModeMonth,
		pomoNowTick: now,
		pomoViewDate: dateOnly(now),
		loading:     startView != viewTasks,
	}
	if sess, err := focus.Load(); err == nil && sess.Active() {
		m.focusTrackedStart = sess.StartedAt
	}
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		loadTreeCmd(m.client),
		loadTodayPomoCmd(m.client),
		tickCmd(),
		focusAlertCheckCmd(),
	}
	switch m.view {
	case viewCalendar:
		cmds = append(cmds, loadCalCmd(m.client))
	case viewPomodoro:
		cmds = append(cmds, loadPomoCmd(m.client, m.pomoViewDate))
	case viewHabits:
		cmds = append(cmds, loadHabitsCmd(m.client))
	}
	return tea.Batch(cmds...)
}

// ---- messages ----

type treeLoadedMsg struct {
	groups   []ticktick.ProjectGroup
	projects []ticktick.Project
	inboxID  string
	err      error
}

type tasksLoadedMsg struct {
	projectID   string
	projectName string
	tasks       []ticktick.Task
	err         error
}

type taskDoneMsg struct {
	count int
	err   error
}

type taskReopenedMsg struct {
	count int
	err   error
}

type taskAddedMsg struct {
	title string
	err   error
}

type taskMovedMsg struct {
	destName string
	count    int
	err      error
}

type taskDeletedMsg struct {
	count int
	err   error
}

type listMovedMsg struct {
	folder string
	err    error
}

type listAddedMsg struct {
	name string
	err  error
}

type folderAddedMsg struct {
	name string
	err  error
}

type listDeletedMsg struct {
	kind string
	name string
	err  error
}

type listRenamedMsg struct {
	kind    string
	oldName string
	newName string
	err     error
}

type taskUpdatedMsg struct {
	title string
	err   error
}

type taskRenamedMsg struct {
	taskID   string
	newTitle string
	err      error
}

type calLoadedMsg struct {
	tasks []ticktick.Task
	err   error
}

type pomoLoadedMsg struct {
	stats            *ticktick.FocusStats
	completed        []ticktick.Task
	taskFocusByID    map[string]ticktick.TaskFocusSummary
	taskFocusByTitle map[string]ticktick.TaskFocusSummary
	viewDate         time.Time
	todayFullCount   int
	err              error
}

type pomoChangedMsg struct {
	err error
	op  string // add, update, delete
}

type habitsLoadedMsg struct {
	habits   []ticktick.Habit
	checkins map[string]ticktick.HabitCheckin
	err      error
}

type habitChangedMsg struct {
	err error
}

type focusActionMsg struct {
	action    string
	taskTitle string
	duration  time.Duration
	err       error
}

type focusNotifyMsg struct {
	escalated bool
	err       error
}

type focusDismissNotifyMsg struct {
	err error
}

type focusAlertCheckMsg struct {
	show  bool
	title string
}

type focusPickerLoadedMsg struct {
	tasks []ticktick.Task
	names map[string]string
	err   error
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadTreeCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		groups, err := c.ListProjectGroups()
		if err != nil {
			return treeLoadedMsg{err: err}
		}
		projects, err := c.ListProjects()
		if err != nil {
			return treeLoadedMsg{err: err}
		}
		projects = ticktick.OpenProjects(projects, false)
		inboxID, _ := c.InboxID()
		return treeLoadedMsg{groups: groups, projects: projects, inboxID: inboxID}
	}
}

func loadTasksCmd(c *ticktick.Client, projectID, projectName string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := c.ProjectTasks(projectID)
		if err != nil {
			return tasksLoadedMsg{projectID: projectID, projectName: projectName, err: err}
		}
		return tasksLoadedMsg{projectID: projectID, projectName: projectName, tasks: tasks}
	}
}

func inboxID(c *ticktick.Client) string {
	id, err := c.InboxID()
	if err != nil {
		return ""
	}
	return id
}

func loadCalCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		tasks, err := c.AllOpenTasks()
		return calLoadedMsg{tasks: tasks, err: err}
	}
}

func loadPomoCmd(c *ticktick.Client, day time.Time) tea.Cmd {
	day = dateOnly(day)
	return func() tea.Msg {
		stats, err := c.FocusForDay(day)
		if err != nil {
			return pomoLoadedMsg{viewDate: day, err: err}
		}
		completed, cerr := c.TasksCompletedOn(day)
		if cerr != nil {
			completed = nil
		}
		todayStats, _ := c.FocusForDay(time.Now())
		todayFull := 0
		if todayStats != nil {
			todayFull = todayStats.FullPomoCount
		}
		now := time.Now()
		historyStart := now.AddDate(0, 0, -ticktick.FocusHistoryDays())
		historyEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999000000, now.Location())
		history, herr := c.FocusForRange(historyStart, historyEnd)
		var byID map[string]ticktick.TaskFocusSummary
		var byTitle map[string]ticktick.TaskFocusSummary
		if herr == nil {
			idx := ticktick.AggregateTaskFocus(history)
			byID = idx.ByID
			byTitle = idx.ByTitle
		}
		return pomoLoadedMsg{
			stats:            stats,
			completed:        completed,
			taskFocusByID:    byID,
			taskFocusByTitle: byTitle,
			viewDate:         day,
			todayFullCount:   todayFull,
		}
	}
}

func loadTodayPomoCmd(c *ticktick.Client) tea.Cmd {
	return loadPomoCmd(c, time.Now())
}

func loadFocusPickerCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		tasks, err := c.AllOpenTasks()
		if err != nil {
			return focusPickerLoadedMsg{err: err}
		}
		projects, _ := c.ListProjects()
		names := make(map[string]string, len(projects))
		for _, p := range projects {
			names[p.ID] = p.Name
		}
		return focusPickerLoadedMsg{
			tasks: filterFocusPickerTasks(tasks),
			names: names,
		}
	}
}

func reloadPomoCmd(c *ticktick.Client, day time.Time) tea.Cmd {
	return loadPomoCmd(c, day)
}

func deletePomodoroCmd(c *ticktick.Client, id string) tea.Cmd {
	return func() tea.Msg {
		return pomoChangedMsg{op: "delete", err: c.DeletePomodoro(id)}
	}
}

func renamePomodoroCmd(c *ticktick.Client, record ticktick.FocusRecord, title string) tea.Cmd {
	return func() tea.Msg {
		return pomoChangedMsg{op: "update", err: c.UpdatePomodoroTitle(record, title)}
	}
}

func addPomodoroCmd(c *ticktick.Client, in ticktick.LogPomodoroInput) tea.Cmd {
	return func() tea.Msg {
		_, err := c.LogPomodoro(in)
		return pomoChangedMsg{op: "add", err: err}
	}
}

func loadHabitsCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		habits, err := c.ListHabits()
		if err != nil {
			return habitsLoadedMsg{err: err}
		}
		ids := make([]string, len(habits))
		for i, h := range habits {
			ids[i] = h.ID
		}
		checkins, cerr := c.HabitCheckinsForDay(ids, time.Now())
		if checkins == nil {
			checkins = map[string]ticktick.HabitCheckin{}
		}
		msg := habitsLoadedMsg{habits: habits, checkins: checkins}
		if cerr != nil {
			msg.err = cerr
		}
		return msg
	}
}

func habitCheckinCmd(c *ticktick.Client, habit ticktick.Habit, done bool) tea.Cmd {
	return func() tea.Msg {
		goal := habit.GoalValue()
		return habitChangedMsg{err: c.UpsertHabitCheckin(habit.ID, time.Now(), done, goal)}
	}
}

func renameHabitCmd(c *ticktick.Client, id, name string) tea.Cmd {
	return func() tea.Msg {
		return habitChangedMsg{err: c.UpdateHabitName(id, name)}
	}
}

func deleteHabitCmd(c *ticktick.Client, id string) tea.Cmd {
	return func() tea.Msg {
		return habitChangedMsg{err: c.DeleteHabit(id)}
	}
}

func completeTasksCmd(c *ticktick.Client, projectID string, taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		n := 0
		var lastErr error
		for _, id := range taskIDs {
			if err := c.CompleteTask(projectID, id); err != nil {
				lastErr = err
				continue
			}
			n++
		}
		if n == 0 && lastErr != nil {
			return taskDoneMsg{err: lastErr}
		}
		if lastErr != nil {
			return taskDoneMsg{count: n, err: fmt.Errorf("completed %d/%d: %w", n, len(taskIDs), lastErr)}
		}
		return taskDoneMsg{count: n}
	}
}

func reopenTasksCmd(c *ticktick.Client, projectID string, taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		n := 0
		var lastErr error
		for _, id := range taskIDs {
			if err := c.ReopenTask(projectID, id); err != nil {
				lastErr = err
				continue
			}
			n++
		}
		if n == 0 && lastErr != nil {
			return taskReopenedMsg{err: lastErr}
		}
		if lastErr != nil {
			return taskReopenedMsg{count: n, err: fmt.Errorf("reopened %d/%d: %w", n, len(taskIDs), lastErr)}
		}
		return taskReopenedMsg{count: n}
	}
}

func deleteTasksCmd(c *ticktick.Client, projectID string, taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		n := 0
		var lastErr error
		for _, id := range taskIDs {
			if err := c.DeleteTask(projectID, id); err != nil {
				lastErr = err
				continue
			}
			n++
		}
		if n == 0 && lastErr != nil {
			return taskDeletedMsg{err: lastErr}
		}
		if lastErr != nil {
			return taskDeletedMsg{count: n, err: fmt.Errorf("deleted %d/%d: %w", n, len(taskIDs), lastErr)}
		}
		return taskDeletedMsg{count: n}
	}
}

func renameListCmd(c *ticktick.Client, kind, ref, newName string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch kind {
		case "folder":
			err = c.RenameProjectGroup(ref, newName)
		default:
			err = c.RenameProject(ref, newName)
		}
		return listRenamedMsg{kind: kind, oldName: ref, newName: newName, err: err}
	}
}

func renameTaskCmd(c *ticktick.Client, taskID, newTitle string) tea.Cmd {
	return func() tea.Msg {
		title := newTitle
		err := c.EditTask(taskID, ticktick.TaskEdit{Title: &title})
		return taskRenamedMsg{taskID: taskID, newTitle: newTitle, err: err}
	}
}

func focusStartCmd(duration time.Duration, task ticktick.Task, projectName string) tea.Cmd {
	return func() tea.Msg {
		_, err := focus.Start(duration, task.ID, task.Title, task.ProjectID, projectName)
		return focusActionMsg{action: "start", err: err}
	}
}

func focusSwitchTaskCmd(c *ticktick.Client, task ticktick.Task, projectName string) tea.Cmd {
	return func() tea.Msg {
		_, err := focus.SwitchTask(c, task.ID, task.Title, task.ProjectID, projectName)
		return focusActionMsg{action: "switch", taskTitle: task.Title, err: err}
	}
}

func focusPauseCmd() tea.Cmd {
	return func() tea.Msg {
		s, err := focus.Load()
		if err != nil {
			return focusActionMsg{action: "pause", err: err}
		}
		if s.State == focus.StatePaused {
			err = s.Resume()
			return focusActionMsg{action: "resume", err: err}
		}
		err = s.Pause()
		return focusActionMsg{action: "pause", err: err}
	}
}

func focusStopCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		notify.CancelOverlay()
		_ = notify.Dismiss()
		s, err := focus.Load()
		if err != nil {
			return focusActionMsg{action: "stop", err: err}
		}
		if s.State == focus.StateAwaitingDismiss {
			if err := focus.FinalizeDismiss(c); err != nil && !errors.Is(err, focus.ErrNoSession) {
				return focusActionMsg{action: "stop", err: err}
			}
			return focusActionMsg{action: "stop", err: nil}
		}
		segStart := s.SegmentLogStart()
		segElapsed := s.CurrentSegmentElapsed()
		taskID := s.TaskID
		taskTitle := s.TaskTitle
		projectName := s.ProjectName
		_, err = s.Finish()
		if err != nil {
			return focusActionMsg{action: "stop", err: err}
		}
		if segElapsed >= focus.MinLogDuration && taskID != "" {
			pauseTotal := s.SegmentPauseTotal(time.Now().UTC())
			_, logErr := c.LogPomodoro(ticktick.LogPomodoroInput{
				TaskID:        taskID,
				TaskTitle:     taskTitle,
				ProjectName:   projectName,
				StartedAt:     segStart,
				Elapsed:       segElapsed,
				PauseDuration: pauseTotal,
			})
			if logErr != nil {
				return focusActionMsg{action: "stop", err: logErr}
			}
		}
		return focusActionMsg{action: "stop", err: nil}
	}
}

func focusCompleteSessionCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		s, err := focus.Load()
		if err != nil {
			return focusActionMsg{action: "complete", err: err}
		}
		if !s.Finished() && s.State != focus.StateAwaitingDismiss {
			return focusActionMsg{action: "complete", err: nil}
		}
		if s.State != focus.StateAwaitingDismiss {
			if err := s.EnterAwaitingDismiss(); err != nil {
				return focusActionMsg{action: "complete", err: err}
			}
		}
		if err := focus.LogPlannedIfNeeded(c, s); err != nil {
			return focusActionMsg{action: "complete", err: err}
		}
		return focusActionMsg{action: "complete", err: nil}
	}
}

func focusFinalizeDismissCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		if err := focus.FinalizeDismiss(c); err != nil && !errors.Is(err, focus.ErrNoSession) {
			return focusActionMsg{action: "finalize", err: err}
		}
		return focusActionMsg{action: "finalize", err: nil}
	}
}

func focusRepeatCmd(c *ticktick.Client) tea.Cmd {
	return func() tea.Msg {
		s, err := focus.RepeatAfterDismiss(c)
		msg := focusActionMsg{action: "repeat", err: err}
		if s != nil {
			msg.taskTitle = s.TaskTitle
			msg.duration = s.Duration
		}
		return msg
	}
}

func focusNotifyCmd(taskTitle string, escalated bool) tea.Cmd {
	return func() tea.Msg {
		return focusNotifyMsg{escalated: escalated, err: notify.FocusDone(taskTitle, escalated)}
	}
}

func focusDismissNotifyCmd() tea.Cmd {
	return func() tea.Msg {
		notify.CancelOverlay()
		err := notify.Dismiss()
		if errors.Is(err, notify.ErrNoActive) {
			err = nil
		}
		return focusDismissNotifyMsg{err: err}
	}
}

func focusAlertCheckCmd() tea.Cmd {
	return func() tea.Msg {
		sess, err := focus.Load()
		if err != nil || !sess.Finished() {
			return focusAlertCheckMsg{}
		}
		return focusAlertCheckMsg{show: true, title: sess.TaskTitle}
	}
}

// ---- update ----

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showFocusAlert {
		return m.updateFocusAlert(msg)
	}
	if m.showHelp {
		return m.updateHelp(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewPomodoro && m.pomoFollowNow {
			m.centerPomoTimelineOnNow()
		}
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeAddTask || m.mode == modeEditTask {
			return m.updateAddTaskForm(msg)
		}
		if m.mode == modeAddList || m.mode == modeAddFolder {
			return m.updateAddListOrFolder(msg)
		}
		if m.mode == modeAddPomoForm {
			return m.updateAddPomoForm(msg)
		}
		if m.mode == modeFilter {
			return m.updateFilter(msg)
		}
		if m.mode == modeRenameList || m.mode == modeRenamePomo || m.mode == modeRenameHabit {
			return m.updateRename(msg)
		}
		if m.mode == modeFocusPicker {
			return m.updateFocusPicker(msg)
		}
		if m.mode == modeListPicker {
			return m.updateListPicker(msg)
		}
		return m.updateKey(msg)

	case treeLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.tree = ticktick.ProjectTreeWithInbox(msg.inboxID, msg.groups, msg.projects)
		m.inboxID = msg.inboxID
		m.listRows = buildListRows(m.tree)
		if m.listCursor >= len(m.listRows) {
			m.listCursor = m.firstListCursor()
		}
		if m.view == viewTasks && m.projectID == "" && len(m.selectableLists()) > 0 {
			m.listCursor = m.firstListCursor()
			m.loading = true
			return m, m.loadCurrentList()
		}
		return m, nil

	case tasksLoadedMsg:
		row, ok := m.currentListRow()
		if !ok || row.node.ID != msg.projectID {
			// Ignore responses from a previous list selection or in-flight refresh.
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.projectID = msg.projectID
		m.projectName = msg.projectName
		m.tasks = dedupeTasks(msg.tasks)
		m.taskSortMode = m.uiSettings.sortForProject(msg.projectID, m.inboxID)
		sortTasksForProject(m.tasks, m.taskSortMode)
		m.applyFilter()
		if m.taskCursor >= len(m.visibleTasks()) {
			m.taskCursor = max(len(m.visibleTasks())-1, 0)
		}
		m.toast = ""
		return m, nil

	case taskReopenedMsg:
		m.clearTaskMarks()
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s reopened %d", iconCheck, msg.count)
			}
			m.errMsg = msg.err.Error()
			return m, loadTasksCmd(m.client, m.projectID, m.projectName)
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s reopened %d tasks", iconCheck, msg.count)
		} else {
			m.toast = iconCheck + " reopened"
		}
		return m, loadTasksCmd(m.client, m.projectID, m.projectName)

	case taskDoneMsg:
		m.clearTaskMarks()
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s completed %d", iconCheck, msg.count)
			}
			m.errMsg = msg.err.Error()
			return m, loadTasksCmd(m.client, m.projectID, m.projectName)
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s completed %d tasks", iconCheck, msg.count)
		} else {
			m.toast = iconCheck + " completed"
		}
		return m, loadTasksCmd(m.client, m.projectID, m.projectName)

	case taskAddedMsg:
		m.mode = modeNormal
		m.editTaskID = ""
		m.blurTaskFormInputs()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = fmt.Sprintf("added %q", msg.title)
		return m, loadTasksCmd(m.client, m.projectID, m.projectName)

	case taskMovedMsg:
		m.mode = modeNormal
		m.clearTaskMarks()
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s moved %d to %s (some failed)", iconCheck, msg.count, msg.destName)
			} else {
				m.toast = iconOverdue + " move failed"
			}
			m.errMsg = msg.err.Error()
			return m, tea.Batch(loadTreeCmd(m.client), loadTasksCmd(m.client, m.projectID, m.projectName))
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s moved %d tasks to %s", iconCheck, msg.count, msg.destName)
		} else {
			m.toast = iconCheck + " moved to " + msg.destName
		}
		return m, tea.Batch(loadTreeCmd(m.client), loadTasksCmd(m.client, m.projectID, m.projectName))

	case taskDeletedMsg:
		m.clearTaskMarks()
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s deleted %d", iconCheck, msg.count)
			}
			m.errMsg = msg.err.Error()
			return m, loadTasksCmd(m.client, m.projectID, m.projectName)
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s deleted %d tasks", iconCheck, msg.count)
		} else {
			m.toast = iconCheck + " task deleted"
		}
		return m, loadTasksCmd(m.client, m.projectID, m.projectName)

	case listMovedMsg:
		m.mode = modeNormal
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = iconCheck + " list moved"
		return m, loadTreeCmd(m.client)

	case listAddedMsg:
		m.mode = modeNormal
		m.addInput.Blur()
		m.addListFolder = ""
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = iconCheck + " list " + msg.name
		return m, loadTreeCmd(m.client)

	case folderAddedMsg:
		m.mode = modeNormal
		m.addInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = iconCheck + " folder " + msg.name
		return m, loadTreeCmd(m.client)

	case listDeletedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = iconCheck + " deleted " + msg.name
		if msg.kind == "list" && strings.EqualFold(m.projectName, msg.name) {
			m.projectID = ""
			m.projectName = ""
			m.tasks = nil
		}
		return m, tea.Batch(loadTreeCmd(m.client), loadTasksCmd(m.client, m.projectID, m.projectName))

	case listRenamedMsg:
		m.mode = modeNormal
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = fmt.Sprintf("renamed list → %q", msg.newName)
		if msg.kind == "list" && m.projectID != "" && strings.EqualFold(m.projectName, msg.oldName) {
			m.projectName = msg.newName
		}
		return m, tea.Batch(loadTreeCmd(m.client), loadTasksCmd(m.client, m.projectID, m.projectName))

	case taskUpdatedMsg:
		m.mode = modeNormal
		m.editTaskID = ""
		m.blurTaskFormInputs()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = fmt.Sprintf("%s updated %q", iconCheck, msg.title)
		return m, loadTasksCmd(m.client, m.projectID, m.projectName)

	case taskRenamedMsg:
		m.mode = modeNormal
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = fmt.Sprintf("renamed task → %q", msg.newTitle)
		return m, loadTasksCmd(m.client, m.projectID, m.projectName)

	case calLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.calTasks = msg.tasks
		return m, nil

	case pomoLoadedMsg:
		if !msg.viewDate.IsZero() && dateKey(msg.viewDate) != dateKey(m.pomoViewDate) {
			return m, nil
		}
		if msg.err != nil {
			if m.view == viewPomodoro {
				m.loading = false
				m.errMsg = msg.err.Error()
			}
			return m, nil
		}
		m.focusStats = msg.stats
		m.todayCompleted = msg.completed
		if !msg.viewDate.IsZero() {
			m.pomoViewDate = dateOnly(msg.viewDate)
		}
		if msg.taskFocusByID != nil {
			m.taskFocusByID = msg.taskFocusByID
		}
		if msg.taskFocusByTitle != nil {
			m.taskFocusByTitle = msg.taskFocusByTitle
		}
		m.todayPomos = msg.todayFullCount
		if msg.stats != nil && msg.viewDate.IsZero() {
			m.todayPomos = msg.stats.FullPomoCount
		}
		if m.pomoCursor >= len(m.focusStatsRecords()) {
			m.pomoCursor = max(len(m.focusStatsRecords())-1, 0)
		}
		slices := aggregateFocusByTask(m.focusStatsRecords())
		if m.pomoLegendCursor >= len(slices) {
			m.pomoLegendCursor = max(len(slices)-1, 0)
		}
		if m.pomoScrollToNow {
			m.centerPomoTimelineOnNow()
			m.pomoScrollToNow = false
		} else if m.pomoFollowNow {
			m.centerPomoTimelineOnNow()
		} else {
			m.syncPomoGridCursor()
			grid := m.pomoDayGrid()
			if len(grid) > 0 && (m.pomoGridCursor < 0 || m.pomoGridCursor >= len(grid)) {
				m.pomoGridCursor = 0
			}
			if len(grid) > 0 && pomoGridHasSessions(grid) {
				if !isSelectablePomoGridRow(grid[m.pomoGridCursor]) {
					if idx := firstSelectablePomoGridRow(grid); idx >= 0 {
						m.pomoGridCursor = idx
						m.syncPomoCursorFromGrid(grid)
					}
				}
			} else if m.pomoViewIsToday() {
				if idx := gridRowForNow(grid); idx >= 0 {
					m.pomoGridCursor = idx
				}
			}
		}
		if m.view == viewPomodoro {
			m.loading = false
		}
		return m, nil

	case pomoChangedMsg:
		m.mode = modeNormal
		m.addInput.Blur()
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		switch msg.op {
		case "delete":
			m.toast = iconCheck + " pomodoro deleted"
		case "delete-pause":
			m.toast = iconCheck + " pause removed"
		case "add":
			m.toast = iconCheck + " pomodoro logged"
		default:
			m.toast = iconCheck + " pomodoro saved"
		}
		return m, reloadPomoCmd(m.client, m.pomoViewDate)

	case habitsLoadedMsg:
		m.loading = false
		m.habits = msg.habits
		m.habitCheckedToday = map[string]bool{}
		for id := range msg.checkins {
			m.habitCheckedToday[id] = true
		}
		if m.habitCursor >= len(m.habits) {
			m.habitCursor = max(len(m.habits)-1, 0)
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		}
		return m, nil

	case habitChangedMsg:
		m.mode = modeNormal
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.toast = iconCheck + " habit updated"
		return m, loadHabitsCmd(m.client)

	case focusActionMsg:
		syncWarn := ""
		if msg.err != nil {
			switch msg.action {
			case "stop", "finalize":
				if sess, loadErr := focus.Load(); loadErr != nil || sess.Active() {
					m.errMsg = msg.err.Error()
					return m, nil
				}
				syncWarn = msg.err.Error()
			case "repeat":
				syncWarn = msg.err.Error()
			default:
				m.errMsg = msg.err.Error()
				return m, nil
			}
		}
		switch msg.action {
		case "start":
			m.focusNotifySent = false
			m.focusNotifyEscalated = false
			m.focusPlannedLogged = false
			m.showFocusAlert = false
			m.focusAlertDismissed = false
			m.focusAlertTitle = ""
			m.focusAlertSince = time.Time{}
			if sess, err := focus.Load(); err == nil {
				m.focusTrackedStart = sess.StartedAt
			}
		case "complete":
			m.focusPlannedLogged = true
		case "finalize":
			m.focusPlannedLogged = false
			m.focusNotifySent = false
			m.focusNotifyEscalated = false
			if syncWarn != "" {
				m.toast = "dismissed · TickTick sync failed"
			}
			return m, tea.Batch(loadPomoCmd(m.client, m.pomoViewDate), focusDismissNotifyCmd())
		case "stop":
			m.focusNotifySent = false
			m.focusNotifyEscalated = false
			m.focusPlannedLogged = false
			m.showFocusAlert = false
			m.focusAlertDismissed = false
			m.focusAlertTitle = ""
			m.focusAlertChord = false
			m.focusAlertSince = time.Time{}
			if syncWarn != "" {
				m.toast = "focus stopped · TickTick sync failed"
			} else {
				m.toast = "focus stopped"
			}
			return m, tea.Batch(loadPomoCmd(m.client, m.pomoViewDate), focusDismissNotifyCmd())
		case "repeat":
			m.focusNotifySent = false
			m.focusNotifyEscalated = false
			m.focusPlannedLogged = false
			m.showFocusAlert = false
			m.focusAlertDismissed = false
			m.focusAlertTitle = ""
			m.focusAlertChord = false
			m.focusAlertSince = time.Time{}
			if sess, err := focus.Load(); err == nil {
				m.focusTrackedStart = sess.StartedAt
			}
			if syncWarn != "" {
				if msg.taskTitle != "" && msg.duration > 0 {
					m.toast = fmt.Sprintf("repeated · %dm · %s · sync failed", int(msg.duration.Minutes()), msg.taskTitle)
				} else {
					m.toast = "focus repeated · TickTick sync failed"
				}
			} else if msg.taskTitle != "" && msg.duration > 0 {
				m.toast = fmt.Sprintf("repeated · %dm · %s", int(msg.duration.Minutes()), msg.taskTitle)
			} else {
				m.toast = "focus repeated"
			}
			return m, tea.Batch(loadPomoCmd(m.client, m.pomoViewDate), loadTodayPomoCmd(m.client), focusDismissNotifyCmd())
		case "switch":
			if msg.taskTitle != "" {
				m.toast = fmt.Sprintf("switched to %q · timer running", msg.taskTitle)
			} else {
				m.toast = "task switched · timer running"
			}
			return m, tea.Batch(loadPomoCmd(m.client, m.pomoViewDate), loadTodayPomoCmd(m.client))
		}
		m.toast = "focus " + msg.action
		return m, loadPomoCmd(m.client, m.pomoViewDate)

	case focusNotifyMsg:
		if msg.err != nil {
			sessionlog.Appendf("focus_notify_result", "escalated=%v err=%v", msg.escalated, msg.err)
			m.errMsg = "notify: " + msg.err.Error()
		} else {
			sessionlog.Appendf("focus_notify_result", "escalated=%v ok", msg.escalated)
			if msg.escalated {
				m.toast = iconPomodoro + " focus overdue — dismiss notification"
			}
		}
		return m, nil

	case focusDismissNotifyMsg:
		if msg.err != nil {
			sessionlog.Appendf("focus_dismiss_result", "err=%v", msg.err)
			m.errMsg = msg.err.Error()
		} else {
			sessionlog.Append("focus_dismiss_result", "ok")
			m.toast = "notification dismissed"
		}
		return m, nil

	case focusAlertCheckMsg:
		if msg.show {
			sessionlog.Appendf("focus_alert_startup", "task=%q", msg.title)
			m.showFocusAlert = true
			m.focusAlertTitle = msg.title
			m.focusAlertSince = time.Now()
			if sess, err := focus.Load(); err == nil && sess.PlannedLogged {
				m.focusPlannedLogged = true
			}
			return m.handleFocusTick()
		}
		return m, nil

	case focusPickerLoadedMsg:
		m.focusPickerLoading = false
		if msg.err != nil {
			m.mode = modeNormal
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.focusPickerTasks = msg.tasks
		m.focusPickerProjectNames = msg.names
		m.focusPickerFilter = ""
		m.focusPickerCursor = m.focusPickerInitialCursor(msg.tasks)
		return m, nil

	case tickMsg:
		m.pomoNowTick = time.Now()
		return m.handleFocusTick()
	}

	return m, nil
}

func (m model) updateHelp(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "?", "q":
			m.showHelp = false
			m.helpFilter = ""
			return m, nil
		case "ctrl+u":
			m.helpFilter = ""
			m.helpCursor = 0
			return m, nil
		case "backspace", "ctrl+h":
			m.helpFilter = trimLastRune(m.helpFilter)
			m.helpCursor = m.clampHelpCursor(m.helpCursor)
			return m, nil
		case "j", "down":
			indices := m.helpFilteredIndices()
			nav := helpNavigableIndices(indices)
			if len(nav) > 0 {
				m.helpCursor = min(m.helpCursor+1, len(nav)-1)
			}
		case "k", "up":
			if len(helpNavigableIndices(m.helpFilteredIndices())) > 0 {
				m.helpCursor = max(m.helpCursor-1, 0)
			}
		case "g":
			m.helpCursor = 0
		case "G":
			n := len(helpNavigableIndices(m.helpFilteredIndices()))
			if n > 0 {
				m.helpCursor = n - 1
			}
		default:
			if key, ok := helpFilterKey(msg); ok {
				m.helpFilter += key
				if len(helpNavigableIndices(m.helpFilteredIndices())) > 0 {
					m.helpCursor = 0
				}
			}
		}
	}
	return m, nil
}

func (m model) clampHelpCursor(cur int) int {
	indices := m.helpFilteredIndices()
	n := len(helpNavigableIndices(indices))
	if n == 0 {
		return 0
	}
	if cur >= n {
		return n - 1
	}
	if cur < 0 {
		return 0
	}
	return cur
}

func (m model) updateKey(msg tea.KeyMsg) (model, tea.Cmd) {
	m.toast = ""
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "T":
		if out, cmd, ok := m.tryOpenFocusPickerSwitch(); ok {
			return out, cmd
		}
	case "?":
		m.showHelp = true
		m.helpCursor = 0
		m.helpFilter = ""
		return m, nil
	case "1":
		return m.switchView(viewTasks)
	case "2":
		return m.switchView(viewCalendar)
	case "3":
		return m.switchView(viewPomodoro)
	case "4":
		return m.switchView(viewHabits)
	case "tab":
		return m.switchView(appView((int(m.view) + 1) % 4))
	case "shift+tab":
		v := int(m.view) - 1
		if v < 0 {
			v = 3
		}
		return m.switchView(appView(v))
	case "r", "R":
		if msg.String() == "R" {
			if out, cmd, ok := m.tryRepeatFocusSession(); ok {
				return out, cmd
			}
		}
		return m.refreshView()
	case ".":
		m.showKeyHints = !m.showKeyHints
		if m.showKeyHints {
			m.toast = "key hints on · . to hide"
		} else {
			m.toast = "key hints hidden · . to show"
		}
		return m, nil
	}

	if m.view == viewTasks {
		return m.updateTasksKey(msg)
	}
	if m.view == viewCalendar {
		return m.updateCalKey(msg)
	}
	if m.view == viewPomodoro {
		return m.updatePomoKey(msg)
	}
	return m.updateHabitsKey(msg)
}

func (m model) switchView(v appView) (model, tea.Cmd) {
	if m.view == v {
		return m, nil
	}
	m.view = v
	m.mode = modeNormal
	m.toast = ""
	m.errMsg = ""
	m.addInput.Blur()
	m.blurTaskFormInputs()
	m.filterInput.Blur()
	m.renameInput.Blur()

	m.uiSettings.LastView = appViewName(v)
	_ = saveUISettings(m.uiSettings)

	var load tea.Cmd
	switch v {
	case viewCalendar:
		m.loading = true
		load = loadCalCmd(m.client)
	case viewPomodoro:
		m.loading = true
		m.pomoCursor = 0
		m.pomoGridCursor = 0
		m.pomoLegendCursor = 0
		m.pomoScrollToNow = true
		m.pomoFollowNow = true
		if m.pomoViewDate.IsZero() {
			m.pomoViewDate = dateOnly(time.Now())
		}
		load = loadPomoCmd(m.client, m.pomoViewDate)
	case viewHabits:
		m.loading = true
		load = loadHabitsCmd(m.client)
	default:
		m.loading = false
		load = loadTodayPomoCmd(m.client)
	}
	// Reset cursor home + clear clutter when pane layout changes (e.g. calendar ↔ tasks).
	clear := func() tea.Msg { return tea.ClearScreen() }
	if load != nil {
		return m, tea.Batch(load, clear)
	}
	return m, clear
}

func (m model) refreshView() (model, tea.Cmd) {
	m.loading = true
	m.errMsg = ""
	switch m.view {
	case viewCalendar:
		return m, loadCalCmd(m.client)
	case viewPomodoro:
		return m, loadPomoCmd(m.client, m.pomoViewDate)
	case viewHabits:
		return m, loadHabitsCmd(m.client)
	default:
		return m, tea.Batch(loadTreeCmd(m.client), loadTasksCmd(m.client, m.projectID, m.projectName), loadTodayPomoCmd(m.client))
	}
}

func (m model) updateTasksKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "l":
		m.paneFocus = paneTasks
		return m, nil
	case "h", "shift+tab":
		m.paneFocus = paneLists
		return m, nil
	case "e":
		if m.paneFocus == paneTasks {
			t, ok := m.selectedTask()
			if !ok {
				m.errMsg = "select a task to edit"
				return m, nil
			}
			if t.Trashed() {
				m.errMsg = "task is in trash"
				return m, nil
			}
			m.openEditTaskForm(t)
			return m, textinput.Blink
		}
		if m.paneFocus != paneLists {
			return m, nil
		}
		row, ok := m.currentListRowForRename()
		if !ok {
			m.errMsg = "select a list or folder to rename"
			return m, nil
		}
		m.mode = modeRenameList
		m.renameInput.SetValue(row.node.Name)
		m.renameInput.Focus()
		return m, textinput.Blink
	case "n":
		if m.paneFocus == paneLists {
			m.addInput.SetValue("")
			m.addListFolder = ""
			m.openAddListFolderPicker()
			return m, nil
		}
		if m.projectID == "" {
			m.errMsg = "select a list first"
			return m, nil
		}
		m.openAddTaskForm()
		return m, textinput.Blink
	case "N":
		if m.paneFocus != paneLists {
			return m, nil
		}
		m.mode = modeAddFolder
		m.addInput.SetValue("")
		m.addInput.Placeholder = "new folder name…"
		m.addInput.Focus()
		return m, textinput.Blink
	case "m":
		if m.paneFocus == paneTasks {
			if len(m.tasksToMove()) == 0 {
				m.errMsg = "select a task to move"
				return m, nil
			}
			m.openListPicker(pickerMoveTask)
			return m, nil
		}
		if m.paneFocus == paneLists {
			row, ok := m.currentListRowForRename()
			if !ok || row.node.Kind != "list" {
				m.errMsg = "select a list to move"
				return m, nil
			}
			m.openListPicker(pickerMoveList)
			return m, nil
		}
		return m, nil
	case " ":
		if m.paneFocus == paneTasks {
			return m.toggleTaskMarkAndAdvance(), nil
		}
		return m, nil
	case "a":
		if m.paneFocus == paneTasks {
			return m.markAllVisibleTasks(), nil
		}
		return m, nil
	case "A":
		if m.paneFocus == paneTasks {
			return m.markAllVisibleDoneTasks(), nil
		}
		return m, nil
	case "u":
		if m.paneFocus == paneTasks && m.markedTaskCount() > 0 {
			return m.clearTaskMarks(), nil
		}
		return m, nil
	case "x", "backspace":
		if m.paneFocus == paneTasks {
			toDelete := m.tasksToDelete()
			if len(toDelete) == 0 {
				return m, nil
			}
			ids := make([]string, len(toDelete))
			for i, t := range toDelete {
				ids[i] = t.ID
			}
			return m, deleteTasksCmd(m.client, m.projectID, ids)
		}
		if m.paneFocus == paneLists {
			row, ok := m.currentListRowForRename()
			if !ok {
				m.errMsg = "select a list or folder to delete"
				return m, nil
			}
			ref := row.node.Name
			if row.node.ID != "" {
				ref = row.node.ID
			}
			return m, deleteListOrFolderCmd(m.client, row.node.Kind, ref)
		}
		return m, nil
	case "/":
		m.mode = modeFilter
		m.filterInput.Focus()
		return m, textinput.Blink
	case "c":
		if m.paneFocus == paneTasks {
			m.showCompleted = !m.showCompleted
			m.applyFilter()
			open, done, _ := m.taskCounts()
			if m.showCompleted {
				m.toast = fmt.Sprintf("%d open · %d done", open, done)
			} else {
				m.toast = fmt.Sprintf("%d open", open)
			}
		}
		return m, nil
	case "C":
		if m.paneFocus == paneTasks {
			m.showDeleted = !m.showDeleted
			m.applyFilter()
			open, done, trashed := m.taskCounts()
			if m.showDeleted {
				m.toast = fmt.Sprintf("%d open · %d done · %d trashed", open, done, trashed)
			} else {
				m.toast = fmt.Sprintf("%d open · %d done", open, done)
			}
		}
		return m, nil
	case "o", "O":
		if m.paneFocus == paneTasks && m.projectID != "" {
			m.taskSortMode = m.taskSortMode.Next()
			m.uiSettings.setSortForProject(m.projectID, m.taskSortMode)
			if err := saveUISettings(m.uiSettings); err != nil {
				m.errMsg = "settings: " + err.Error()
			} else {
				m.toast = "sort: " + m.taskSortMode.Label()
			}
			sortTasksForProject(m.tasks, m.taskSortMode)
			m.applyFilter()
		}
		return m, nil
	case "z", "Z":
		if m.paneFocus == paneTasks {
			next := m.taskDetailLayoutPref().Toggle()
			m.uiSettings.TaskDetailLayout = next
			if err := saveUISettings(m.uiSettings); err != nil {
				m.errMsg = "settings: " + err.Error()
			} else {
				m.toast = "detail: " + next.Label()
			}
		}
		return m, nil
	case "d", "D", "enter":
		if m.paneFocus != paneTasks {
			return m, nil
		}
		if toReopen := m.tasksToReopen(); len(toReopen) > 0 {
			ids := make([]string, len(toReopen))
			for i, t := range toReopen {
				ids[i] = t.ID
			}
			return m, reopenTasksCmd(m.client, m.projectID, ids)
		}
		toComplete := m.tasksToComplete()
		if len(toComplete) == 0 {
			return m, nil
		}
		ids := make([]string, len(toComplete))
		for i, t := range toComplete {
			ids[i] = t.ID
		}
		return m, completeTasksCmd(m.client, m.projectID, ids)
	case "j", "down":
		if m.paneFocus == paneLists {
			m.listCursor = m.nextListCursor(1)
			return m.beginListLoad()
		}
		m.taskCursor = min(m.taskCursor+1, len(m.visibleTasks())-1)
		return m, nil
	case "k", "up":
		if m.paneFocus == paneLists {
			m.listCursor = m.nextListCursor(-1)
			return m.beginListLoad()
		}
		m.taskCursor = max(m.taskCursor-1, 0)
		return m, nil
	case "pgdown", "ctrl+d":
		page := m.scrollPageSize()
		if m.paneFocus == paneLists {
			for i := 0; i < page; i++ {
				next := m.nextListCursor(1)
				if next == m.listCursor {
					break
				}
				m.listCursor = next
			}
			return m.beginListLoad()
		}
		tasks := m.visibleTasks()
		if len(tasks) > 0 {
			m.taskCursor = min(m.taskCursor+page, len(tasks)-1)
		}
		return m, nil
	case "pgup", "ctrl+u":
		page := m.scrollPageSize()
		if m.paneFocus == paneLists {
			for i := 0; i < page; i++ {
				next := m.nextListCursor(-1)
				if next == m.listCursor {
					break
				}
				m.listCursor = next
			}
			return m.beginListLoad()
		}
		m.taskCursor = max(m.taskCursor-page, 0)
		return m, nil
	case "g":
		if m.paneFocus == paneLists {
			m.listCursor = m.firstListCursor()
			return m.beginListLoad()
		}
		m.taskCursor = 0
		return m, nil
	case "G":
		if m.paneFocus == paneTasks && len(m.visibleTasks()) > 0 {
			m.taskCursor = len(m.visibleTasks()) - 1
		}
		return m, nil
	}
	return m, nil
}

func (m model) updateCalKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "d":
		m.calMode = calModeDay
		m.calDate = dateOnly(time.Now())
		m.calTaskCursor = 0
		m.calGridCursor = 0
		return m, nil
	case "w":
		m.calMode = calModeWeek
		m.calTaskCursor = 0
		m.calGridCursor = 0
		return m, nil
	case "m":
		m.calMode = calModeMonth
		m.calTaskCursor = 0
		m.calGridCursor = 0
		return m, nil
	case "y":
		m.calMode = calModeYear
		m.calTaskCursor = 0
		m.calGridCursor = 0
		return m, nil
	case "t":
		m.calDate = dateOnly(time.Now())
		m.calTaskCursor = 0
		m.calGridCursor = 0
		return m, nil
	case "enter":
		return m.calDrillDown(), nil
	case "[", "left":
		return m.calNavPrev(), nil
	case "]", "right":
		return m.calNavNext(), nil
	case "h":
		return m.calMoveHoriz(-1), nil
	case "l":
		return m.calMoveHoriz(1), nil
	case "j", "down":
		return m.calMoveVert(1), nil
	case "k", "up":
		return m.calMoveVert(-1), nil
	}
	return m, nil
}

func (m model) updatePomoKey(msg tea.KeyMsg) (model, tea.Cmd) {
	grid := m.pomoDayGrid()
	switch msg.String() {
	case "[":
		return m.pomoNavDay(-1)
	case "]":
		return m.pomoNavDay(1)
	case "t":
		return m.pomoJumpToday()
	case "j", "down":
		if len(grid) > 0 {
			m.pomoFollowNow = false
			m.pomoGridCursor = nextPomoGridCursor(grid, m.pomoGridCursor, 1)
			m.syncPomoCursorFromGrid(grid)
		}
		return m, nil
	case "k", "up":
		if len(grid) > 0 {
			m.pomoFollowNow = false
			m.pomoGridCursor = nextPomoGridCursor(grid, m.pomoGridCursor, -1)
			m.syncPomoCursorFromGrid(grid)
		}
		return m, nil
	case "h", "left":
		if slices := m.pomoLegendSlices(); len(slices) > 0 {
			m.pomoLegendCursor = max(m.pomoLegendCursor-1, 0)
			m.syncPomoSelectionFromLegend()
		}
		return m, nil
	case "l", "right":
		if slices := m.pomoLegendSlices(); len(slices) > 0 {
			m.pomoLegendCursor = min(m.pomoLegendCursor+1, len(slices)-1)
			m.syncPomoSelectionFromLegend()
		}
		return m, nil
	case "e":
		r, ok := m.selectedPomoRecord()
		if !ok {
			m.errMsg = "no pomodoro selected"
			return m, nil
		}
		m.mode = modeRenamePomo
		m.renameInput.SetValue(r.TaskTitle())
		m.renameInput.Focus()
		return m, textinput.Blink
	case "n":
		return m.openAddPomoForm()
	case "x", "backspace":
		cmd := m.pomoDeleteCmd()
		if cmd == nil {
			m.errMsg = "select a session on the timeline (j/k) or legend (h/l), then x"
			return m, nil
		}
		return m, cmd
	case "s":
		return m.openFocusPicker(25)
	case "f":
		return m.openFocusPicker(5)
	case "p":
		return m, focusPauseCmd()
	case "S":
		return m, focusStopCmd(m.client)
	case "R":
		if out, cmd, ok := m.tryRepeatFocusSession(); ok {
			return out, cmd
		}
		return m, nil
	case "T":
		if out, cmd, ok := m.tryOpenFocusPickerSwitch(); ok {
			return out, cmd
		}
		return m, nil
	case "D":
		return m, focusDismissNotifyCmd()
	case "z", "Z":
		d := m.pomoTimelineDensity().Toggle()
		m.uiSettings.PomoTimelineDensity = d
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else {
			m.toast = "timeline " + d.Label()
		}
		m.centerPomoTimelineOnNow()
		m.syncPomoGridCursor()
		return m, nil
	}
	return m, nil
}

func (m *model) syncPomoGridCursor() {
	grid := m.pomoDayGrid()
	if len(grid) == 0 {
		m.pomoGridCursor = 0
		return
	}
	m.pomoGridCursor = gridRowForRecord(grid, m.pomoCursor)
	if m.pomoGridCursor >= len(grid) {
		m.pomoGridCursor = len(grid) - 1
	}
}

func (m *model) syncPomoCursorFromGrid(grid []dayGridRow) {
	if m.pomoGridCursor < 0 || m.pomoGridCursor >= len(grid) {
		return
	}
	row := grid[m.pomoGridCursor]
	if row.kind == "pomo" {
		m.pomoCursor = row.recIdx
	}
}

func (m model) pomoLegendSlices() []focusSlice {
	if m.focusStats == nil {
		return nil
	}
	anchor := m.pomoTimelineAnchor()
	var live *focus.Session
	if m.pomoViewIsToday() {
		live, _ = focus.Load()
	}
	pauses, _ := focus.PauseSpellsForTimeline(m.pomoViewDate, live, anchor)
	return buildLegendSlices(m.focusStatsRecords(), pauses, anchor)
}

func (m model) focusStatsRecords() []ticktick.FocusRecord {
	if m.focusStats == nil {
		return nil
	}
	return m.focusStats.Records
}

func (m model) selectedPomoRecord() (ticktick.FocusRecord, bool) {
	recs := m.focusStatsRecords()
	if len(recs) == 0 || m.pomoCursor < 0 || m.pomoCursor >= len(recs) {
		return ticktick.FocusRecord{}, false
	}
	return recs[m.pomoCursor], true
}

func (m model) openFocusPicker(defaultMinutes int) (model, tea.Cmd) {
	m.mode = modeFocusPicker
	m.focusPickerSwitch = false
	m.focusPickerMinutes = clampFocusMinutes(defaultMinutes)
	m.focusPickerCursor = 0
	m.focusPickerTasks = nil
	m.focusPickerProjectNames = nil
	m.focusPickerLoading = true
	m.focusPickerEditDuration = false
	m.focusPickerDurationBuf = ""
	m.focusPickerFilter = ""
	return m, loadFocusPickerCmd(m.client)
}

func (m model) tryOpenFocusPickerSwitch() (model, tea.Cmd, bool) {
	sess, err := focus.Load()
	if err != nil || !sess.Active() || sess.State == focus.StateAwaitingDismiss {
		return m, nil, false
	}
	out, cmd := m.openFocusPickerSwitch()
	return out, cmd, true
}

func (m model) openFocusPickerSwitch() (model, tea.Cmd) {
	m.mode = modeFocusPicker
	m.focusPickerSwitch = true
	m.focusPickerCursor = 0
	m.focusPickerTasks = nil
	m.focusPickerProjectNames = nil
	m.focusPickerLoading = true
	m.focusPickerEditDuration = false
	m.focusPickerDurationBuf = ""
	m.focusPickerFilter = ""
	return m, loadFocusPickerCmd(m.client)
}

func (m model) tryRepeatFocusSession() (model, tea.Cmd, bool) {
	sess, err := focus.Load()
	if err != nil || !sess.Active() {
		return m, nil, false
	}
	out, cmd := m.repeatFocusSession()
	return out, cmd, true
}

func (m model) updateFocusPicker(msg tea.KeyMsg) (model, tea.Cmd) {
	if !m.focusPickerSwitch && m.focusPickerEditDuration {
		switch msg.String() {
		case "esc":
			m.focusPickerEditDuration = false
			m.focusPickerDurationBuf = ""
			return m, nil
		case "enter":
			if v, ok := parseFocusDurationInput(m.focusPickerDurationBuf); ok {
				m.focusPickerMinutes = v
			}
			m.focusPickerEditDuration = false
			m.focusPickerDurationBuf = ""
			return m, nil
		case "backspace", "ctrl+h":
			if len(m.focusPickerDurationBuf) > 0 {
				m.focusPickerDurationBuf = m.focusPickerDurationBuf[:len(m.focusPickerDurationBuf)-1]
			}
			return m, nil
		default:
			if len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9' {
				if len(m.focusPickerDurationBuf) < 3 {
					m.focusPickerDurationBuf += string(msg.Runes[0])
				}
				return m, nil
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.focusPickerFilter) != "" {
			m.focusPickerFilter = ""
			m.focusPickerCursor = m.clampFocusPickerCursor()
			return m, nil
		}
		m.mode = modeNormal
		m.focusPickerSwitch = false
		return m, nil
	case "ctrl+u":
		m.focusPickerFilter = ""
		m.focusPickerCursor = m.clampFocusPickerCursor()
		return m, nil
	case "backspace", "ctrl+h":
		if m.focusPickerFilter != "" {
			m.focusPickerFilter = trimLastRune(m.focusPickerFilter)
			m.focusPickerCursor = m.clampFocusPickerCursor()
		}
		return m, nil
	case "t":
		if m.focusPickerSwitch {
			break
		}
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerEditDuration = true
			m.focusPickerDurationBuf = ""
			return m, nil
		}
	case "j", "down":
		visible := m.focusPickerVisibleTasks()
		if len(visible) > 0 {
			m.focusPickerCursor = min(m.focusPickerCursor+1, len(visible)-1)
		}
		return m, nil
	case "k", "up":
		if len(m.focusPickerVisibleTasks()) > 0 {
			m.focusPickerCursor = max(m.focusPickerCursor-1, 0)
		}
		return m, nil
	case "[", "-":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = clampFocusMinutes(m.focusPickerMinutes - 5)
			return m, nil
		}
	case "]", "+", "=":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = clampFocusMinutes(m.focusPickerMinutes + 5)
			return m, nil
		}
	case "1":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = 5
			return m, nil
		}
	case "2":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = 15
			return m, nil
		}
	case "3":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = 25
			return m, nil
		}
	case "4":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = 45
			return m, nil
		}
	case "5":
		if strings.TrimSpace(m.focusPickerFilter) == "" {
			m.focusPickerMinutes = 60
			return m, nil
		}
	case "enter":
		task, ok := m.selectedFocusPickerTask()
		if !ok {
			m.errMsg = "no task selected"
			return m, nil
		}
		switchMode := m.focusPickerSwitch
		m.mode = modeNormal
		m.focusPickerSwitch = false
		projectName := m.focusPickerProjectNames[task.ProjectID]
		if switchMode {
			return m, focusSwitchTaskCmd(m.client, task, projectName)
		}
		d := time.Duration(m.focusPickerMinutes) * time.Minute
		return m, focusStartCmd(d, task, projectName)
	}

	if key, ok := helpFilterKey(msg); ok {
		m.focusPickerFilter += key
		if len(m.focusPickerVisibleTasks()) > 0 {
			m.focusPickerCursor = 0
		} else {
			m.focusPickerCursor = 0
		}
	}
	return m, nil
}

func (m model) clampFocusPickerCursor() int {
	n := len(m.focusPickerVisibleTasks())
	if n == 0 {
		return 0
	}
	if m.focusPickerCursor >= n {
		return n - 1
	}
	if m.focusPickerCursor < 0 {
		return 0
	}
	return m.focusPickerCursor
}

func (m model) updateHabitsKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.habitCursor = min(m.habitCursor+1, max(len(m.habits)-1, 0))
	case "k", "up":
		m.habitCursor = max(m.habitCursor-1, 0)
	case " ", "enter":
		if len(m.habits) == 0 || m.habitCursor >= len(m.habits) {
			return m, nil
		}
		h := m.habits[m.habitCursor]
		done := !m.habitCheckedToday[h.ID]
		return m, habitCheckinCmd(m.client, h, done)
	case "e":
		if len(m.habits) == 0 || m.habitCursor >= len(m.habits) {
			m.errMsg = "no habit selected"
			return m, nil
		}
		h := m.habits[m.habitCursor]
		m.mode = modeRenameHabit
		m.renameInput.SetValue(h.Name)
		m.renameInput.Focus()
		return m, textinput.Blink
	case "x", "backspace":
		if len(m.habits) == 0 || m.habitCursor >= len(m.habits) {
			m.errMsg = "no habit selected"
			return m, nil
		}
		h := m.habits[m.habitCursor]
		return m, deleteHabitCmd(m.client, h.ID)
	}
	return m, nil
}

func (m model) updateAddListOrFolder(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.addInput.Blur()
		m.addInput.Placeholder = "new task title…"
		m.addListFolder = ""
		return m, nil
	case "ctrl+f":
		if m.mode == modeAddList {
			m.addInput.Blur()
			m.openAddListFolderPicker()
			return m, nil
		}
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.addInput.Value())
		if name == "" {
			return m, nil
		}
		if m.mode == modeAddFolder {
			return m, createFolderCmd(m.client, name)
		}
		return m, createListCmd(m.client, name, m.addListFolder)
	}
	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

func (m model) updateRename(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.renameInput.Blur()
		return m, nil
	case "enter":
		newName := strings.TrimSpace(m.renameInput.Value())
		if newName == "" {
			return m, nil
		}
		if m.mode == modeRenamePomo {
			r, ok := m.selectedPomoRecord()
			if !ok {
				m.mode = modeNormal
				m.renameInput.Blur()
				return m, nil
			}
			if newName == r.TaskTitle() {
				m.mode = modeNormal
				m.renameInput.Blur()
				return m, nil
			}
			return m, renamePomodoroCmd(m.client, r, newName)
		}
		if m.mode == modeRenameHabit {
			if len(m.habits) == 0 || m.habitCursor >= len(m.habits) {
				m.mode = modeNormal
				m.renameInput.Blur()
				return m, nil
			}
			h := m.habits[m.habitCursor]
			if newName == h.Name {
				m.mode = modeNormal
				m.renameInput.Blur()
				return m, nil
			}
			return m, renameHabitCmd(m.client, h.ID, newName)
		}
		row, ok := m.currentListRowForRename()
		if !ok {
			m.mode = modeNormal
			m.renameInput.Blur()
			return m, nil
		}
		if newName == row.node.Name {
			m.mode = modeNormal
			m.renameInput.Blur()
			return m, nil
		}
		return m, renameListCmd(m.client, row.node.Kind, row.node.Name, newName)
	}
	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m model) updateFilter(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.filterInput.Blur()
		m.filterInput.SetValue("")
		m.applyFilter()
		return m, nil
	case "enter":
		m.mode = modeNormal
		m.filterInput.Blur()
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *model) applyFilter() {
	n := len(m.visibleTasks())
	if m.taskCursor >= n {
		m.taskCursor = max(n-1, 0)
	}
}

func (m model) taskCounts() (open, done, trashed int) {
	for _, t := range m.tasks {
		if t.IsSubtask() {
			continue
		}
		if t.Trashed() {
			trashed++
			continue
		}
		if t.Done() {
			done++
		} else {
			open++
		}
	}
	return open, done, trashed
}

func (m model) visibleTaskRows() []taskListRow {
	return buildVisibleTaskRows(m.tasks, m.taskSortMode, m.showCompleted, m.showDeleted, m.filterInput.Value())
}

func (m model) visibleTasks() []ticktick.Task {
	rows := m.visibleTaskRows()
	out := make([]ticktick.Task, len(rows))
	for i, r := range rows {
		out[i] = r.Task
	}
	return out
}

func (m model) scrollPageSize() int {
	l := m.layout()
	detailBudget := 0
	if m.showTaskDetail() {
		detailBudget = m.taskDetailLineBudget(l.innerLines)
	}
	n := paneScrollRows(l.innerLines, detailBudget)
	if n > 1 {
		return n - 1
	}
	return 1
}

func (m model) beginListLoad() (model, tea.Cmd) {
	m.loading = true
	m.tasks = nil
	m.taskCursor = 0
	m.taskMarked = nil
	m.errMsg = ""
	if row, ok := m.currentListRow(); ok {
		m.projectID = row.node.ID
		m.projectName = row.node.Name
	}
	cmds := []tea.Cmd{func() tea.Msg { return tea.ClearScreen() }}
	if load := m.loadCurrentList(); load != nil {
		cmds = append(cmds, load)
	}
	return m, tea.Batch(cmds...)
}

func (m model) loadCurrentList() tea.Cmd {
	row, ok := m.currentListRow()
	if !ok {
		return nil
	}
	return loadTasksCmd(m.client, row.node.ID, row.node.Name)
}

func buildListRows(tree []ticktick.ProjectTreeNode) []listRow {
	out := make([]listRow, len(tree))
	for i, n := range tree {
		out[i] = listRow{node: n, selectable: n.Kind == "list"}
	}
	return out
}

func (m model) selectableLists() []listRow {
	var out []listRow
	for _, r := range m.listRows {
		if r.selectable {
			out = append(out, r)
		}
	}
	return out
}

func (m model) currentListRow() (listRow, bool) {
	if m.listCursor < 0 || m.listCursor >= len(m.listRows) {
		return listRow{}, false
	}
	r := m.listRows[m.listCursor]
	if !r.selectable {
		return listRow{}, false
	}
	return r, true
}

func (m model) currentListRowForRename() (listRow, bool) {
	if m.listCursor < 0 || m.listCursor >= len(m.listRows) {
		return listRow{}, false
	}
	r := m.listRows[m.listCursor]
	if r.node.Kind != "list" && r.node.Kind != "folder" {
		return listRow{}, false
	}
	return r, true
}

func (m model) firstListCursor() int {
	for i, r := range m.listRows {
		if r.selectable {
			return i
		}
	}
	return 0
}

func (m model) nextListCursor(delta int) int {
	if len(m.listRows) == 0 {
		return 0
	}
	i := m.listCursor
	for tries := 0; tries < len(m.listRows); tries++ {
		i += delta
		if i < 0 {
			i = len(m.listRows) - 1
		}
		if i >= len(m.listRows) {
			i = 0
		}
		if m.listRows[i].selectable {
			return i
		}
	}
	return m.listCursor
}

// ---- view ----

func (m model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	if m.showFocusAlert {
		return m.renderFocusAlertOverlay()
	}
	if m.showHelp {
		return m.renderHelpOverlay()
	}
	if m.mode == modeFocusPicker {
		return m.renderFocusPickerOverlay()
	}
	if m.mode == modeAddPomoForm {
		return m.renderAddPomoOverlay()
	}
	if m.mode == modeListPicker {
		return m.renderListPickerOverlay()
	}

	l := m.layout()
	header := m.renderHeader()
	body := m.renderBody(l)
	footer := m.renderFooter()
	return composeView(header, body, footer, l.termW, l.termH, l.bodyLines)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
