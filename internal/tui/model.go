package tui

import (
	"encoding/json"
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
	"github.com/j4y-w4lk3r/ttcli/internal/taskarchive"
	"github.com/j4y-w4lk3r/ttcli/internal/taskcheckin"
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
	modeConfirmDelete
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
	repo   *ticktick.Repository

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

	tasks                  []ticktick.Task
	taskCursor             int
	taskMarked             map[string]struct{}
	showCompleted          bool
	showDeleted            bool
	taskScope              TaskScope
	archiveStore           *taskarchive.Store
	archiveRecords         []taskarchive.Record
	pendingPermanentDelete []ticktick.Task

	projectID   string
	projectName string
	inboxID     string

	addInput             textinput.Model
	taskTitleInput       textinput.Model
	addTaskDueInput      textinput.Model
	addTaskTimeInput     textinput.Model
	addTaskDurationInput textinput.Model
	addTaskFocusInput    textinput.Model
	addTaskRepeatInput   textinput.Model
	addTaskNotesInput    textarea.Model
	addTaskField         int
	addTaskReminderIdx   int
	addTaskPriorityIdx   int
	addTaskRepeatIdx     int
	addTaskRepeatFromIdx int
	editTaskID           string
	filterInput          textinput.Model
	renameInput          textinput.Model

	addListFolder string // folder context when creating a list

	listPickerPurpose     listPickerPurpose
	listPickerRows        []listPickerRow
	listPickerCursor      int
	listPickerTaskIDs     []string
	listPickerFromProject string
	listPickerListRef     string

	calDate         time.Time
	calMode         calMode
	calTaskCursor   int
	calGridCursor   int
	calDayCenterNow bool
	calWeekViewport int
	calTasks        []ticktick.Task
	calCompleted    []ticktick.Task
	calFocusByDate  map[string]*ticktick.FocusStats
	taskCheckins    []taskcheckin.Record
	checkinStore    *taskcheckin.Store
	pendingCheckin  string

	focusStats        *ticktick.FocusStats
	taskFocusByID     map[string]ticktick.TaskFocusSummary
	taskFocusByTitle  map[string]ticktick.TaskFocusSummary
	pomoCursor        int
	pomoGridCursor    int
	pomoViewport      int
	pomoLegendCursor  int
	pomoScrollToNow   bool
	pomoFollowNow     bool
	pomoNowTick       time.Time
	pomoViewDate      time.Time
	habits            []ticktick.Habit
	habitCursor       int
	habitCheckedToday map[string]bool
	todayPomos        int
	todayCompleted    []ticktick.Task

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

	addPomoField        int
	addPomoLogDate      time.Time
	addPomoStartUnset   bool
	addPomoStartMinutes int
	addPomoEditPause    bool
	addPomoPauseInput   textinput.Model

	taskSortMode TaskSortMode
	uiSettings   uiSettings

	focusTrackedStart time.Time

	loading bool
	toast   string
	errMsg  string

	cacheStale   bool
	cacheSavedAt time.Time
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
	addFocus := newAddTaskFieldInput("minutes or pomos, e.g. 75m or 3p")
	addRepeat := newAddTaskFieldInput("RRULE:… or ERULE:…")
	addNotes := newAddTaskNotesInput()
	taskTitle := newAddTaskFieldInput("task title…")
	taskTitle.CharLimit = 500
	addPomoPause := newAddTaskFieldInput("minutes · empty = none")

	now := time.Now()
	settings := loadUISettings()
	settings.applyPlanningDefaults()
	settings.applyPomoDefaults()
	startView := parseAppView(settings.LastView)
	if settings.TaskDetailLayout == "" {
		settings.TaskDetailLayout = TaskDetailBottom
	}
	var repo *ticktick.Repository
	if client != nil {
		repo = ticktick.NewRepository(client)
	}
	var checkinStore *taskcheckin.Store
	var taskCheckins []taskcheckin.Record
	var archiveStore *taskarchive.Store
	var archiveRecords []taskarchive.Record
	if client != nil {
		checkinStore = taskcheckin.NewStore()
		taskCheckins, _ = checkinStore.Records()
		archiveStore = taskarchive.NewStore()
		archiveRecords, _ = archiveStore.Records()
	}
	m := model{
		client:               client,
		repo:                 repo,
		checkinStore:         checkinStore,
		taskCheckins:         taskCheckins,
		archiveStore:         archiveStore,
		archiveRecords:       archiveRecords,
		taskScope:            normalizeTaskScope(settings.TaskScope),
		calFocusByDate:       make(map[string]*ticktick.FocusStats),
		view:                 startView,
		paneFocus:            paneLists,
		uiSettings:           settings,
		addInput:             add,
		taskTitleInput:       taskTitle,
		addTaskDueInput:      addDue,
		addTaskTimeInput:     addTime,
		addTaskDurationInput: addDur,
		addTaskFocusInput:    addFocus,
		addTaskRepeatInput:   addRepeat,
		addTaskNotesInput:    addNotes,
		addPomoPauseInput:    addPomoPause,
		filterInput:          filter,
		renameInput:          rename,
		calDate:              dateOnly(now),
		calMode:              calModeMonth,
		pomoNowTick:          now,
		pomoViewDate:         dateOnly(now),
		loading:              startView != viewTasks,
	}
	m.hydrateCachedData()
	if sess, err := focus.Load(); err == nil && sess.Active() {
		m.focusTrackedStart = sess.StartedAt
	}
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		loadTreeCmd(m.repo, true),
		loadTodayPomoCmd(m.repo, true),
		loadPomoHistoryCmd(m.repo, false),
		tickCmd(),
		focusAlertCheckCmd(),
	}
	switch m.view {
	case viewCalendar:
		cmds = append(cmds, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, true, m.uiSettings.weekStartsMonday()))
	case viewPomodoro:
		cmds = append(cmds,
			loadPomoCmd(m.repo, m.pomoViewDate, true),
			loadPomoHistoryCmd(m.repo, false),
		)
	case viewHabits:
		cmds = append(cmds, loadHabitsCmd(m.repo, true))
	}
	return tea.Batch(cmds...)
}

func (m *model) hydrateCachedData() {
	if m.repo == nil {
		return
	}
	if tree, meta, ok := m.repo.CachedTree(); ok {
		projects := ticktick.OpenProjects(tree.Projects, false)
		m.tree = ticktick.ProjectTreeWithInbox(tree.InboxID, tree.Groups, projects)
		m.inboxID = tree.InboxID
		m.listRows = buildListRows(m.tree)
		m.listCursor = m.firstListCursor()
		if row, ok := m.currentListRow(); ok {
			m.projectID = row.node.ID
			m.projectName = row.node.Name
			if tasks, taskMeta, found := m.repo.CachedProjectTasks(row.node.ID); found {
				m.tasks = dedupeTasks(tasks)
				m.taskSortMode = m.uiSettings.sortForProject(row.node.ID, m.inboxID)
				sortTasksForProject(m.tasks, m.taskSortMode)
				m.noteCache(taskMeta, nil)
				m.loading = false
			}
		}
		m.noteCache(meta, nil)
	}
	if tasks, meta, ok := m.repo.CachedOpenTasks(); ok {
		m.calTasks = tasks
		m.noteCache(meta, nil)
		if m.view == viewCalendar {
			m.loading = false
		}
	}
	today := dateOnly(time.Now())
	if stats, meta, ok := m.repo.CachedFocusDay(today); ok {
		m.todayPomos = stats.FullPomoCount
		m.calFocusByDate[dateKey(today)] = stats
		m.noteCache(meta, nil)
		if m.view == viewPomodoro && dateKey(m.pomoViewDate) == dateKey(today) {
			m.focusStats = stats
			m.loading = false
		}
	}
	if records, meta, ok := m.repo.CachedFocusHistory(); ok {
		idx := ticktick.AggregateTaskFocus(records)
		m.taskFocusByID = idx.ByID
		m.taskFocusByTitle = idx.ByTitle
		m.noteCache(meta, nil)
	}
	if habits, meta, ok := m.repo.CachedHabits(today); ok {
		m.habits = habits.Habits
		m.habitCheckedToday = map[string]bool{}
		for id := range habits.Checkins {
			m.habitCheckedToday[id] = true
		}
		m.noteCache(meta, nil)
		if m.view == viewHabits {
			m.loading = false
		}
	}
}

func (m *model) noteCache(meta ticktick.CacheMeta, err error) bool {
	if meta.FromCache {
		if meta.SavedAt.After(m.cacheSavedAt) {
			m.cacheSavedAt = meta.SavedAt
		}
		if meta.Stale || err != nil {
			m.cacheStale = true
		}
		if err != nil {
			m.toast = "offline · showing cached data"
		}
		return true
	}
	if err == nil {
		m.cacheStale = false
		m.cacheSavedAt = meta.SavedAt
	}
	return false
}

// ---- messages ----

type treeLoadedMsg struct {
	groups   []ticktick.ProjectGroup
	projects []ticktick.Project
	inboxID  string
	cache    ticktick.CacheMeta
	err      error
}

type tasksLoadedMsg struct {
	projectID   string
	projectName string
	tasks       []ticktick.Task
	cache       ticktick.CacheMeta
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
	count          int
	op             string
	archiveRecords []taskarchive.Record
	err            error
}

type taskRecreatedMsg struct {
	archiveRecords []taskarchive.Record
	taskID         string
	err            error
}

type taskCheckinMsg struct {
	records       []taskcheckin.Record
	record        taskcheckin.Record
	checked       bool
	remoteChanged bool
	err           error
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
	tasks     []ticktick.Task
	completed []ticktick.Task
	cache     ticktick.CacheMeta
	err       error
}

type calFocusLoadedMsg struct {
	day   time.Time
	stats *ticktick.FocusStats
	cache ticktick.CacheMeta
	err   error
}

type calFocusRangeLoadedMsg struct {
	days  map[string]*ticktick.FocusStats
	cache ticktick.CacheMeta
	err   error
}

type pomoLoadedMsg struct {
	stats            *ticktick.FocusStats
	completed        []ticktick.Task
	taskFocusByID    map[string]ticktick.TaskFocusSummary
	taskFocusByTitle map[string]ticktick.TaskFocusSummary
	viewDate         time.Time
	todayFullCount   int
	cache            ticktick.CacheMeta
	err              error
}

type pomoHistoryLoadedMsg struct {
	taskFocusByID    map[string]ticktick.TaskFocusSummary
	taskFocusByTitle map[string]ticktick.TaskFocusSummary
	cache            ticktick.CacheMeta
	err              error
}

type todayPomoLoadedMsg struct {
	count int
	cache ticktick.CacheMeta
	err   error
}

type pomoChangedMsg struct {
	err error
	op  string // add, update, delete
}

type habitsLoadedMsg struct {
	habits   []ticktick.Habit
	checkins map[string]ticktick.HabitCheckin
	cache    ticktick.CacheMeta
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
	cache ticktick.CacheMeta
	err   error
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadTreeCmd(repo *ticktick.Repository, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return treeLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		tree, cache, err := repo.Tree(force)
		projects := ticktick.OpenProjects(tree.Projects, false)
		return treeLoadedMsg{
			groups: tree.Groups, projects: projects, inboxID: tree.InboxID,
			cache: cache, err: err,
		}
	}
}

func loadTasksCmd(repo *ticktick.Repository, projectID, projectName string, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return tasksLoadedMsg{projectID: projectID, projectName: projectName, err: fmt.Errorf("TickTick data repository unavailable")}
		}
		tasks, cache, err := repo.ProjectTasks(projectID, force)
		return tasksLoadedMsg{
			projectID: projectID, projectName: projectName, tasks: tasks,
			cache: cache, err: err,
		}
	}
}

func inboxID(c *ticktick.Client) string {
	id, err := c.InboxID()
	if err != nil {
		return ""
	}
	return id
}

func loadCalCmd(repo *ticktick.Repository, day time.Time, mode calMode, force bool, monday ...bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return calLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		tasks, cache, err := repo.OpenTasks(force)
		if err != nil && tasks == nil {
			return calLoadedMsg{cache: cache, err: err}
		}
		weekStartsMonday := true
		if len(monday) > 0 {
			weekStartsMonday = monday[0]
		}
		start, end := calendarRangeFor(day, mode, weekStartsMonday)
		completed, completedCache, completedErr := repo.CompletedBetween(start, end, force)
		if completedErr != nil {
			if err == nil {
				err = completedErr
			}
			if cache.SavedAt.IsZero() || completedCache.SavedAt.Before(cache.SavedAt) {
				cache = completedCache
			}
		}
		return calLoadedMsg{tasks: tasks, completed: completed, cache: cache, err: err}
	}
}

func loadCalFocusCmd(repo *ticktick.Repository, day time.Time, force bool) tea.Cmd {
	day = dateOnly(day)
	return func() tea.Msg {
		if repo == nil {
			return calFocusLoadedMsg{day: day, err: fmt.Errorf("TickTick data repository unavailable")}
		}
		stats, cache, err := repo.FocusDay(day, force)
		return calFocusLoadedMsg{day: day, stats: stats, cache: cache, err: err}
	}
}

func loadCalFocusRangeCmd(repo *ticktick.Repository, start, end time.Time, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return calFocusRangeLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		days, cache, err := repo.FocusBetween(start, end, force)
		return calFocusRangeLoadedMsg{days: days, cache: cache, err: err}
	}
}

func loadCalendarViewCmd(repo *ticktick.Repository, day time.Time, mode calMode, force, monday bool) tea.Cmd {
	cmds := []tea.Cmd{loadCalCmd(repo, day, mode, force, monday)}
	if mode == calModeDay {
		cmds = append(cmds, loadCalFocusCmd(repo, day, force))
	} else {
		start, end := calendarRangeFor(day, mode, true)
		cmds = append(cmds, loadCalFocusRangeCmd(repo, start, end, force))
	}
	return tea.Batch(cmds...)
}

func (m model) loadVisibleCalendarFocusCmd(force bool) tea.Cmd {
	if m.view != viewCalendar {
		return nil
	}
	if m.calMode != calModeDay {
		start, end := calendarRangeFor(m.calDate, m.calMode, true)
		return loadCalFocusRangeCmd(m.repo, start, end, force)
	}
	return loadCalFocusCmd(m.repo, m.calDate, force)
}

func (m model) loadVisibleTaskFocusCmd(force bool) tea.Cmd {
	if m.view != viewTasks {
		return nil
	}
	return loadPomoHistoryCmd(m.repo, force)
}

func loadPomoCmd(repo *ticktick.Repository, day time.Time, force bool) tea.Cmd {
	day = dateOnly(day)
	return func() tea.Msg {
		if repo == nil {
			return pomoLoadedMsg{viewDate: day, err: fmt.Errorf("TickTick data repository unavailable")}
		}
		stats, cache, err := repo.FocusDay(day, force)
		if err != nil {
			if stats == nil {
				return pomoLoadedMsg{viewDate: day, cache: cache, err: err}
			}
		}
		completed, completedCache, cerr := repo.CompletedOn(day, force)
		if cerr != nil {
			if completed == nil {
				completed = nil
			}
			if cache.SavedAt.IsZero() || completedCache.SavedAt.Before(cache.SavedAt) {
				cache = completedCache
			}
			if err == nil {
				err = cerr
			}
		}
		todayFull := 0
		if dateKey(day) == dateKey(time.Now()) {
			todayFull = stats.FullPomoCount
		} else {
			todayStats, todayCache, _ := repo.FocusDay(time.Now(), false)
			if todayStats != nil {
				todayFull = todayStats.FullPomoCount
			}
			if cache.SavedAt.IsZero() {
				cache = todayCache
			}
		}
		return pomoLoadedMsg{
			stats:          stats,
			completed:      completed,
			viewDate:       day,
			todayFullCount: todayFull,
			cache:          cache,
			err:            err,
		}
	}
}

func loadTodayPomoCmd(repo *ticktick.Repository, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return todayPomoLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		stats, cache, err := repo.FocusDay(time.Now(), force)
		count := 0
		if stats != nil {
			count = stats.FullPomoCount
		}
		return todayPomoLoadedMsg{count: count, cache: cache, err: err}
	}
}

func loadPomoHistoryCmd(repo *ticktick.Repository, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return pomoHistoryLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		history, cache, err := repo.FocusHistory(force)
		if err != nil && history == nil {
			return pomoHistoryLoadedMsg{cache: cache, err: err}
		}
		idx := ticktick.AggregateTaskFocus(history)
		return pomoHistoryLoadedMsg{
			taskFocusByID: idx.ByID, taskFocusByTitle: idx.ByTitle,
			cache: cache, err: err,
		}
	}
}

func loadFocusPickerCmd(repo *ticktick.Repository) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return focusPickerLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		tasks, cache, err := repo.OpenTasks(false)
		if err != nil && tasks == nil {
			return focusPickerLoadedMsg{cache: cache, err: err}
		}
		tree, _, _ := repo.Tree(false)
		names := make(map[string]string, len(tree.Projects))
		for _, p := range tree.Projects {
			names[p.ID] = p.Name
		}
		return focusPickerLoadedMsg{
			tasks: filterFocusPickerTasks(tasks),
			names: names,
			cache: cache,
			err:   err,
		}
	}
}

func reloadPomoCmd(repo *ticktick.Repository, day time.Time) tea.Cmd {
	return loadPomoCmd(repo, day, false)
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

func loadHabitsCmd(repo *ticktick.Repository, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return habitsLoadedMsg{err: fmt.Errorf("TickTick data repository unavailable")}
		}
		snapshot, cache, err := repo.HabitsForDay(time.Now(), force)
		return habitsLoadedMsg{
			habits: snapshot.Habits, checkins: snapshot.Checkins,
			cache: cache, err: err,
		}
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

func completeTasksCmd(c *ticktick.Client, tasks []ticktick.Task, fallbackProjectID string) tea.Cmd {
	return func() tea.Msg {
		count := 0
		for projectID, taskIDs := range groupTasksByProject(tasks, fallbackProjectID) {
			if err := c.CompleteTasks(projectID, taskIDs); err != nil {
				return taskDoneMsg{count: count, err: err}
			}
			count += len(taskIDs)
		}
		return taskDoneMsg{count: count}
	}
}

func reopenTasksCmd(c *ticktick.Client, tasks []ticktick.Task, fallbackProjectID string) tea.Cmd {
	return func() tea.Msg {
		count := 0
		for projectID, taskIDs := range groupTasksByProject(tasks, fallbackProjectID) {
			if err := c.ReopenTasks(projectID, taskIDs); err != nil {
				return taskReopenedMsg{count: count, err: err}
			}
			count += len(taskIDs)
		}
		return taskReopenedMsg{count: count}
	}
}

func trashTasksCmd(c *ticktick.Client, tasks []ticktick.Task, fallbackProjectID string) tea.Cmd {
	return func() tea.Msg {
		count := 0
		for projectID, taskIDs := range groupTasksByProject(tasks, fallbackProjectID) {
			if err := c.TrashTasks(projectID, taskIDs); err != nil {
				return taskDeletedMsg{count: count, op: "trash", err: err}
			}
			count += len(taskIDs)
		}
		return taskDeletedMsg{count: count, op: "trash"}
	}
}

func restoreTasksCmd(c *ticktick.Client, tasks []ticktick.Task, fallbackProjectID string) tea.Cmd {
	return func() tea.Msg {
		count := 0
		for projectID, taskIDs := range groupTasksByProject(tasks, fallbackProjectID) {
			if err := c.RestoreTasks(projectID, taskIDs); err != nil {
				return taskDeletedMsg{count: count, op: "restore", err: err}
			}
			count += len(taskIDs)
		}
		return taskDeletedMsg{count: count, op: "restore"}
	}
}

type permanentTaskDeleteClient interface {
	TaskSnapshot(projectID, taskID string) (json.RawMessage, error)
	DeleteTasks(projectID string, taskIDs []string) error
}

func permanentlyDeleteTasksCmd(
	c permanentTaskDeleteClient,
	store *taskarchive.Store,
	tasks []ticktick.Task,
	fallbackProjectID string,
) tea.Cmd {
	return func() tea.Msg {
		if c == nil || store == nil {
			return taskDeletedMsg{op: "permanent", err: fmt.Errorf("task archive or TickTick client unavailable")}
		}
		for i := range tasks {
			if tasks[i].ProjectID == "" {
				tasks[i].ProjectID = fallbackProjectID
			}
			raw, err := c.TaskSnapshot(tasks[i].ProjectID, tasks[i].ID)
			if err != nil {
				return taskDeletedMsg{op: "permanent", err: fmt.Errorf("snapshot %q: %w", tasks[i].Title, err)}
			}
			if err := store.Put(taskarchive.Record{Task: tasks[i], RawTask: raw}); err != nil {
				return taskDeletedMsg{op: "permanent", err: fmt.Errorf("archive %q: %w", tasks[i].Title, err)}
			}
		}
		records, err := store.Records()
		if err != nil {
			return taskDeletedMsg{op: "permanent", err: fmt.Errorf("read task archive: %w", err)}
		}
		count := 0
		for projectID, taskIDs := range groupTasksByProject(tasks, fallbackProjectID) {
			if err := c.DeleteTasks(projectID, taskIDs); err != nil {
				return taskDeletedMsg{count: count, op: "permanent", archiveRecords: records, err: err}
			}
			count += len(taskIDs)
		}
		return taskDeletedMsg{count: count, op: "permanent", archiveRecords: records}
	}
}

func recreateArchivedTaskCmd(c *ticktick.Client, store *taskarchive.Store, record taskarchive.Record) tea.Cmd {
	return func() tea.Msg {
		if c == nil || store == nil {
			return taskRecreatedMsg{err: fmt.Errorf("task archive or TickTick client unavailable")}
		}
		taskID, err := c.RecreateTaskSnapshot(record.RawTask, record.Task.ProjectID)
		if err != nil {
			return taskRecreatedMsg{err: err}
		}
		if err := store.MarkRecreated(record.ArchiveID, taskID); err != nil {
			return taskRecreatedMsg{taskID: taskID, err: fmt.Errorf("task recreated but archive update failed: %w", err)}
		}
		records, err := store.Records()
		return taskRecreatedMsg{archiveRecords: records, taskID: taskID, err: err}
	}
}

func taskCheckinKey(seriesID string, day time.Time) string {
	return seriesID + "\x00" + dateKey(day)
}

func taskCheckinRecord(task ticktick.Task, day time.Time, native bool) taskcheckin.Record {
	return taskcheckin.Record{
		TaskID:      task.ID,
		SeriesID:    task.SeriesID(),
		ProjectID:   task.ProjectID,
		Title:       task.Title,
		Date:        dateKey(day),
		CompletedAt: time.Now(),
		Native:      native,
	}
}

func toggleTaskCheckinCmd(
	c *ticktick.Client,
	store *taskcheckin.Store,
	task ticktick.Task,
	day time.Time,
	currentlyDone bool,
	native bool,
) tea.Cmd {
	return func() tea.Msg {
		record := taskCheckinRecord(task, day, native)
		if store == nil {
			return taskCheckinMsg{record: record, err: fmt.Errorf("task check-in store unavailable")}
		}
		remoteChanged := false
		if native {
			if c == nil {
				return taskCheckinMsg{record: record, err: fmt.Errorf("TickTick client unavailable")}
			}
			var err error
			if currentlyDone {
				err = c.ReopenTask(task.ProjectID, task.ID)
			} else {
				err = c.CompleteTaskOccurrence(task.ProjectID, task.ID)
			}
			if err != nil {
				return taskCheckinMsg{record: record, checked: currentlyDone, err: err}
			}
			remoteChanged = true
		}
		checked := !currentlyDone
		var err error
		if checked {
			err = store.Put(record)
		} else {
			err = store.Remove(record.SeriesID, record.Date)
		}
		records, loadErr := store.Records()
		if records == nil {
			records = []taskcheckin.Record{}
		}
		if err == nil {
			err = loadErr
		}
		return taskCheckinMsg{
			records: records, record: record, checked: checked,
			remoteChanged: remoteChanged, err: err,
		}
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
		if m.mode == modeConfirmDelete {
			return m.updatePermanentDeleteConfirmation(msg)
		}
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
		if msg.err != nil && !m.noteCache(msg.cache, msg.err) {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
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
		if msg.err != nil && !m.noteCache(msg.cache, msg.err) {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
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
		if m.repo != nil && (msg.err == nil || msg.count > 0) {
			m.repo.InvalidateTasks(m.projectID)
		}
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s reopened %d", iconCheck, msg.count)
			}
			m.errMsg = msg.err.Error()
			return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s reopened %d tasks", iconCheck, msg.count)
		} else {
			m.toast = iconCheck + " reopened"
		}
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case taskDoneMsg:
		m.clearTaskMarks()
		if m.repo != nil && (msg.err == nil || msg.count > 0) {
			m.repo.InvalidateTasks(m.projectID)
		}
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s completed %d", iconCheck, msg.count)
			}
			m.errMsg = msg.err.Error()
			return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s completed %d tasks", iconCheck, msg.count)
		} else {
			m.toast = iconCheck + " completed"
		}
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case taskAddedMsg:
		m.mode = modeNormal
		m.editTaskID = ""
		m.blurTaskFormInputs()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTasks(m.projectID)
		}
		m.toast = fmt.Sprintf("added %q", msg.title)
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case taskMovedMsg:
		m.mode = modeNormal
		m.clearTaskMarks()
		if m.repo != nil && (msg.err == nil || msg.count > 0) {
			m.repo.InvalidateTasks(m.projectID)
			m.repo.InvalidateTree()
		}
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s moved %d to %s (some failed)", iconCheck, msg.count, msg.destName)
			} else {
				m.toast = iconOverdue + " move failed"
			}
			m.errMsg = msg.err.Error()
			return m, tea.Batch(loadTreeCmd(m.repo, false), loadTasksCmd(m.repo, m.projectID, m.projectName, false))
		}
		if msg.count > 1 {
			m.toast = fmt.Sprintf("%s moved %d tasks to %s", iconCheck, msg.count, msg.destName)
		} else {
			m.toast = iconCheck + " moved to " + msg.destName
		}
		return m, tea.Batch(loadTreeCmd(m.repo, false), loadTasksCmd(m.repo, m.projectID, m.projectName, false))

	case taskDeletedMsg:
		m.clearTaskMarks()
		m.mode = modeNormal
		m.pendingPermanentDelete = nil
		if msg.archiveRecords != nil {
			m.archiveRecords = msg.archiveRecords
		}
		if m.repo != nil && (msg.err == nil || msg.count > 0) {
			m.repo.InvalidateTasks(m.projectID)
		}
		if msg.err != nil {
			if msg.count > 0 {
				m.toast = fmt.Sprintf("%s %s %d task(s); remaining operation failed", iconCheck, msg.op, msg.count)
			}
			m.errMsg = msg.err.Error()
			return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)
		}
		switch msg.op {
		case "restore":
			m.toast = fmt.Sprintf("%s restored %d task(s)", iconCheck, msg.count)
		case "permanent":
			m.toast = fmt.Sprintf("%s permanently deleted %d task(s) · snapshot archived", iconCheck, msg.count)
		default:
			m.toast = fmt.Sprintf("%s moved %d task(s) to Trash", iconCheck, msg.count)
		}
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case taskRecreatedMsg:
		if msg.archiveRecords != nil {
			m.archiveRecords = msg.archiveRecords
		}
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			if msg.taskID != "" {
				m.toast = iconCheck + " task recreated; archive status update failed"
			}
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTasks(m.projectID)
		}
		m.toast = iconCheck + " archived task recreated as a new task"
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case taskCheckinMsg:
		m.pendingCheckin = ""
		if msg.records != nil {
			m.taskCheckins = msg.records
		}
		if msg.remoteChanged && m.repo != nil {
			m.repo.InvalidateTasks(msg.record.ProjectID)
		}
		if msg.err != nil {
			if msg.remoteChanged {
				m.toast = "TickTick updated · local history failed"
			}
			m.errMsg = msg.err.Error()
		} else if msg.checked {
			if msg.record.Native {
				m.toast = iconCheck + " recurring occurrence completed"
			} else {
				m.toast = iconCheck + " checked in locally"
			}
		} else {
			m.toast = iconCheck + " check-in undone"
		}
		if !msg.remoteChanged {
			return m, nil
		}
		return m, tea.Batch(
			loadTasksCmd(m.repo, m.projectID, m.projectName, false),
			loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday()),
		)

	case listMovedMsg:
		m.mode = modeNormal
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTree()
		}
		m.toast = iconCheck + " list moved"
		return m, loadTreeCmd(m.repo, false)

	case listAddedMsg:
		m.mode = modeNormal
		m.addInput.Blur()
		m.addListFolder = ""
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTree()
		}
		m.toast = iconCheck + " list " + msg.name
		return m, loadTreeCmd(m.repo, false)

	case folderAddedMsg:
		m.mode = modeNormal
		m.addInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTree()
		}
		m.toast = iconCheck + " folder " + msg.name
		return m, loadTreeCmd(m.repo, false)

	case listDeletedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTree()
		}
		m.toast = iconCheck + " deleted " + msg.name
		if msg.kind == "list" && strings.EqualFold(m.projectName, msg.name) {
			m.projectID = ""
			m.projectName = ""
			m.tasks = nil
		}
		return m, tea.Batch(loadTreeCmd(m.repo, false), loadTasksCmd(m.repo, m.projectID, m.projectName, false))

	case listRenamedMsg:
		m.mode = modeNormal
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTree()
		}
		m.toast = fmt.Sprintf("renamed list → %q", msg.newName)
		if msg.kind == "list" && m.projectID != "" && strings.EqualFold(m.projectName, msg.oldName) {
			m.projectName = msg.newName
		}
		return m, tea.Batch(loadTreeCmd(m.repo, false), loadTasksCmd(m.repo, m.projectID, m.projectName, false))

	case taskUpdatedMsg:
		m.mode = modeNormal
		m.editTaskID = ""
		m.blurTaskFormInputs()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTasks(m.projectID)
		}
		m.toast = fmt.Sprintf("%s updated %q", iconCheck, msg.title)
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case taskRenamedMsg:
		m.mode = modeNormal
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateTasks(m.projectID)
		}
		m.toast = fmt.Sprintf("renamed task → %q", msg.newTitle)
		return m, loadTasksCmd(m.repo, m.projectID, m.projectName, false)

	case calLoadedMsg:
		m.loading = false
		if msg.err != nil && !m.noteCache(msg.cache, msg.err) {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		m.calTasks = msg.tasks
		m.calCompleted = msg.completed
		return m, nil

	case calFocusLoadedMsg:
		if msg.err != nil && msg.stats == nil {
			if m.view == viewCalendar && m.calMode == calModeDay && dateKey(msg.day) == dateKey(m.calDate) {
				if !m.noteCache(msg.cache, msg.err) {
					m.errMsg = msg.err.Error()
				}
			}
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		if m.calFocusByDate == nil {
			m.calFocusByDate = make(map[string]*ticktick.FocusStats)
		}
		m.calFocusByDate[dateKey(msg.day)] = msg.stats
		return m, nil

	case calFocusRangeLoadedMsg:
		if msg.err != nil && len(msg.days) == 0 {
			if m.view == viewCalendar && !m.noteCache(msg.cache, msg.err) {
				m.errMsg = msg.err.Error()
			}
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		if m.calFocusByDate == nil {
			m.calFocusByDate = make(map[string]*ticktick.FocusStats)
		}
		for key, stats := range msg.days {
			m.calFocusByDate[key] = stats
		}
		return m, nil

	case pomoLoadedMsg:
		if !msg.viewDate.IsZero() && dateKey(msg.viewDate) != dateKey(m.pomoViewDate) {
			return m, nil
		}
		if msg.err != nil && msg.stats == nil {
			if m.view == viewPomodoro {
				m.loading = false
				if !m.noteCache(msg.cache, msg.err) {
					m.errMsg = msg.err.Error()
				}
			}
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
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

	case pomoHistoryLoadedMsg:
		if msg.err != nil && msg.taskFocusByID == nil {
			if !m.noteCache(msg.cache, msg.err) {
				m.errMsg = msg.err.Error()
			}
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		m.taskFocusByID = msg.taskFocusByID
		m.taskFocusByTitle = msg.taskFocusByTitle
		return m, nil

	case todayPomoLoadedMsg:
		if msg.err != nil && !m.noteCache(msg.cache, msg.err) {
			if m.view == viewPomodoro {
				m.errMsg = msg.err.Error()
			}
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		m.todayPomos = msg.count
		return m, nil

	case pomoChangedMsg:
		m.mode = modeNormal
		m.addInput.Blur()
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateFocus(m.pomoViewDate)
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
		return m, tea.Batch(
			reloadPomoCmd(m.repo, m.pomoViewDate),
			loadPomoHistoryCmd(m.repo, false),
		)

	case habitsLoadedMsg:
		m.loading = false
		if msg.err != nil && msg.habits == nil && !m.noteCache(msg.cache, msg.err) {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		m.habits = msg.habits
		m.habitCheckedToday = map[string]bool{}
		for id := range msg.checkins {
			m.habitCheckedToday[id] = true
		}
		if m.habitCursor >= len(m.habits) {
			m.habitCursor = max(len(m.habits)-1, 0)
		}
		return m, nil

	case habitChangedMsg:
		m.mode = modeNormal
		m.renameInput.Blur()
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			m.repo.InvalidateHabits()
		}
		m.toast = iconCheck + " habit updated"
		return m, loadHabitsCmd(m.repo, false)

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
		if m.repo != nil && msg.err == nil {
			switch msg.action {
			case "finalize", "stop", "repeat":
				m.repo.InvalidateFocus(m.pomoViewDate)
				m.repo.InvalidateFocus(time.Now())
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
			return m, tea.Batch(loadPomoCmd(m.repo, m.pomoViewDate, false), m.loadVisibleCalendarFocusCmd(false), m.loadVisibleTaskFocusCmd(false), focusDismissNotifyCmd())
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
			return m, tea.Batch(loadPomoCmd(m.repo, m.pomoViewDate, false), m.loadVisibleCalendarFocusCmd(false), m.loadVisibleTaskFocusCmd(false), focusDismissNotifyCmd())
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
			return m, tea.Batch(loadPomoCmd(m.repo, m.pomoViewDate, false), loadTodayPomoCmd(m.repo, false), m.loadVisibleCalendarFocusCmd(false), m.loadVisibleTaskFocusCmd(false), focusDismissNotifyCmd())
		case "switch":
			if msg.taskTitle != "" {
				m.toast = fmt.Sprintf("switched to %q · timer running", msg.taskTitle)
			} else {
				m.toast = "task switched · timer running"
			}
			return m, tea.Batch(loadPomoCmd(m.repo, m.pomoViewDate, false), loadTodayPomoCmd(m.repo, false), m.loadVisibleCalendarFocusCmd(false), m.loadVisibleTaskFocusCmd(false))
		}
		m.toast = "focus " + msg.action
		return m, tea.Batch(loadPomoCmd(m.repo, m.pomoViewDate, false), m.loadVisibleCalendarFocusCmd(false), m.loadVisibleTaskFocusCmd(false))

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
		if msg.err != nil && msg.tasks == nil {
			m.mode = modeNormal
			if !m.noteCache(msg.cache, msg.err) {
				m.errMsg = msg.err.Error()
			}
			return m, nil
		}
		m.noteCache(msg.cache, msg.err)
		m.focusPickerTasks = msg.tasks
		m.focusPickerProjectNames = msg.names
		m.focusPickerFilter = ""
		m.focusPickerCursor = m.focusPickerInitialCursor(msg.tasks)
		return m, nil

	case tickMsg:
		m.pomoNowTick = time.Now()
		if m.view == viewPomodoro && m.pomoFollowNow {
			m.centerPomoTimelineOnNow()
		}
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
		m.loading = len(m.calTasks) == 0
		load = loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday())
	case viewPomodoro:
		m.loading = m.focusStats == nil
		m.pomoCursor = 0
		m.pomoGridCursor = 0
		m.pomoLegendCursor = 0
		m.pomoScrollToNow = true
		m.pomoFollowNow = true
		if m.pomoViewDate.IsZero() {
			m.pomoViewDate = dateOnly(time.Now())
		}
		load = tea.Batch(
			loadPomoCmd(m.repo, m.pomoViewDate, false),
			loadPomoHistoryCmd(m.repo, false),
		)
	case viewHabits:
		m.loading = len(m.habits) == 0
		load = loadHabitsCmd(m.repo, false)
	default:
		m.loading = false
		load = tea.Batch(
			loadTodayPomoCmd(m.repo, false),
			loadPomoHistoryCmd(m.repo, false),
		)
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
		return m, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, true, m.uiSettings.weekStartsMonday())
	case viewPomodoro:
		return m, tea.Batch(
			loadPomoCmd(m.repo, m.pomoViewDate, true),
			loadPomoHistoryCmd(m.repo, true),
		)
	case viewHabits:
		return m, loadHabitsCmd(m.repo, true)
	default:
		return m, tea.Batch(
			loadTreeCmd(m.repo, true),
			loadTasksCmd(m.repo, m.projectID, m.projectName, true),
			loadTodayPomoCmd(m.repo, true),
			loadPomoHistoryCmd(m.repo, true),
		)
	}
}

func (m model) checkinFor(seriesID string, day time.Time) (taskcheckin.Record, bool) {
	date := dateKey(day)
	for _, record := range m.taskCheckins {
		recordSeries := record.SeriesID
		if recordSeries == "" {
			recordSeries = record.TaskID
		}
		if recordSeries == seriesID && record.Date == date {
			return record, true
		}
	}
	return taskcheckin.Record{}, false
}

func (m model) beginTaskCheckin(
	task ticktick.Task,
	day time.Time,
	currentlyDone bool,
	native bool,
) (model, tea.Cmd) {
	day = dateOnly(day)
	if day.After(dateOnly(time.Now())) {
		m.errMsg = "cannot check in a future date"
		return m, nil
	}
	if task.ID == "" {
		m.errMsg = "select a task to check in"
		return m, nil
	}
	if task.ProjectID == "" {
		task.ProjectID = m.projectID
	}
	m.errMsg = ""
	m.pendingCheckin = taskCheckinKey(task.SeriesID(), day)
	if currentlyDone {
		m.toast = "undoing check-in…"
	} else {
		m.toast = "checking in…"
	}
	return m, toggleTaskCheckinCmd(
		m.client, m.checkinStore, task, day, currentlyDone, native,
	)
}

func (m model) selectedCalEntry() (calEntry, bool) {
	idx := m.calIdx()
	switch m.calMode {
	case calModeDay:
		rows, _ := m.calDayRows(idx)
		if len(rows) == 0 {
			return calEntry{}, false
		}
		row := rows[normalizeCalDayGridCursor(rows, m.calGridCursor)]
		if calDayRowSelectable(row) {
			return row.entry, true
		}
	case calModeWeek:
		rows := m.calWeekRows(idx)
		if len(rows) == 0 {
			return calEntry{}, false
		}
		row := rows[normalizeCalWeekGridCursor(rows, m.calGridCursor)]
		if calWeekRowSelectable(row) {
			return row.entry, true
		}
	}
	return calEntry{}, false
}

func (m model) updatePermanentDeleteConfirmation(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		tasks := append([]ticktick.Task(nil), m.pendingPermanentDelete...)
		m.pendingPermanentDelete = nil
		m.mode = modeNormal
		m.toast = "archiving before permanent deletion…"
		return m, permanentlyDeleteTasksCmd(m.client, m.archiveStore, tasks, m.projectID)
	case "n", "N", "esc":
		m.pendingPermanentDelete = nil
		m.mode = modeNormal
		m.toast = "permanent deletion cancelled"
		return m, nil
	}
	return m, nil
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
	case "x":
		if m.paneFocus == paneTasks {
			task, ok := m.selectedTask()
			if !ok {
				m.errMsg = "select a task to check in"
				return m, nil
			}
			if task.Trashed() {
				m.errMsg = "task is in trash"
				return m, nil
			}
			record, checked := m.checkinFor(task.SeriesID(), time.Now())
			if task.Repeating() && checked && !task.Done() && record.Native {
				m.toast = "already checked in today · undo from Calendar day view"
				return m, nil
			}
			if task.Done() && !task.Repeating() {
				m.errMsg = "task is already completed in TickTick"
				return m, nil
			}
			return m.beginTaskCheckin(task, time.Now(), checked || task.Done(), task.Repeating())
		}
		if m.paneFocus != paneLists {
			return m, nil
		}
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
	case "backspace":
		if m.paneFocus == paneTasks {
			toDelete := m.tasksToDelete()
			if len(toDelete) == 0 {
				return m, nil
			}
			switch m.effectiveTaskScope() {
			case TaskScopeArchive:
				m.errMsg = "archive records are durable recovery snapshots"
				return m, nil
			case TaskScopeTrash:
				m.pendingPermanentDelete = toDelete
				m.mode = modeConfirmDelete
				m.toast = fmt.Sprintf("permanently delete %d task(s)?", len(toDelete))
				return m, nil
			default:
				return m, trashTasksCmd(m.client, toDelete, m.projectID)
			}
		}
		if m.paneFocus != paneLists {
			return m, nil
		}
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
	case "/":
		m.mode = modeFilter
		m.filterInput.Focus()
		return m, textinput.Blink
	case "c":
		if m.paneFocus == paneTasks {
			m = m.setTaskScope(m.effectiveTaskScope().Next())
		}
		return m, nil
	case "C":
		if m.paneFocus == paneTasks {
			m = m.setTaskScope(m.effectiveTaskScope().Prev())
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
		if m.effectiveTaskScope() == TaskScopeArchive {
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			record, ok := m.archiveRecordForTaskID(task.ID)
			if !ok {
				m.errMsg = "archive record unavailable"
				return m, nil
			}
			m.toast = "recreating archived task…"
			return m, recreateArchivedTaskCmd(m.client, m.archiveStore, record)
		}
		if m.effectiveTaskScope() == TaskScopeTrash {
			toRestore := m.tasksToDelete()
			if len(toRestore) == 0 {
				return m, nil
			}
			return m, restoreTasksCmd(m.client, toRestore, m.projectID)
		}
		if toReopen := m.tasksToReopen(); len(toReopen) > 0 {
			for _, task := range toReopen {
				if task.Repeating() {
					m.errMsg = "use x to check in recurring tasks"
					return m, nil
				}
			}
			return m, reopenTasksCmd(m.client, toReopen, m.projectID)
		}
		toComplete := m.tasksToComplete()
		if len(toComplete) == 0 {
			return m, nil
		}
		for _, task := range toComplete {
			if task.Repeating() {
				m.errMsg = "use x to check in recurring tasks"
				return m, nil
			}
		}
		return m, completeTasksCmd(m.client, toComplete, m.projectID)
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
	case "o":
		if m.calMode != calModeDay {
			return m, nil
		}
		m.uiSettings.CalendarDayShowOverdue = !m.uiSettings.CalendarDayShowOverdue
		m.calGridCursor = 0
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else if m.uiSettings.CalendarDayShowOverdue {
			m.toast = "overdue tasks shown"
		} else {
			m.toast = "overdue tasks hidden"
		}
		return m, nil
	case "x":
		entry, ok := m.selectedCalEntry()
		if !ok {
			m.errMsg = "select a Day or Week task to check in"
			return m, nil
		}
		return m.beginTaskCheckin(entry.Task, entry.Date, entry.Done(), entry.NativeCheckin())
	case "d":
		m.calMode = calModeDay
		m.calDate = dateOnly(time.Now())
		m.calTaskCursor = 0
		m.calGridCursor = 0
		m.calDayCenterNow = true
		m.calWeekViewport = 0
		return m, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday())
	case "w":
		m.calMode = calModeWeek
		m.calTaskCursor = 0
		m.calGridCursor = 0
		m.calDayCenterNow = false
		m.calWeekViewport = 0
		return m, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday())
	case "m":
		m.calMode = calModeMonth
		m.calTaskCursor = 0
		m.calGridCursor = 0
		m.calDayCenterNow = false
		m.calWeekViewport = 0
		return m, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday())
	case "y":
		m.calMode = calModeYear
		m.calTaskCursor = 0
		m.calGridCursor = 0
		m.calDayCenterNow = false
		m.calWeekViewport = 0
		return m, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday())
	case "t":
		m.calDate = dateOnly(time.Now())
		m.calTaskCursor = 0
		m.calGridCursor = 0
		if m.calMode == calModeDay {
			m.calDayCenterNow = true
		} else if m.calMode == calModeWeek {
			m.calWeekViewport = -1
		}
		return m, loadCalendarViewCmd(m.repo, m.calDate, m.calMode, false, m.uiSettings.weekStartsMonday())
	case "z", "Z":
		if m.calMode != calModeWeek {
			return m, nil
		}
		density := m.uiSettings.calendarWeekDensity().Toggle()
		m.uiSettings.CalendarWeekDensity = density
		m.calWeekViewport = 0
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else {
			m.toast = "week timeline " + density.Label()
		}
		return m, nil
	case "pgdown", "ctrl+d":
		if m.calMode == calModeWeek {
			m.calWeekViewport = max(m.calWeekViewport, 0) + max(m.layout().innerLines/2, 5)
		}
		return m, nil
	case "pgup", "ctrl+u":
		if m.calMode == calModeWeek {
			m.calWeekViewport = max(max(m.calWeekViewport, 0)-max(m.layout().innerLines/2, 5), 0)
		}
		return m, nil
	case "enter":
		out := m.calDrillDown()
		return out, loadCalendarViewCmd(out.repo, out.calDate, out.calMode, false, out.uiSettings.weekStartsMonday())
	case "[", "left":
		out := m.calNavPrev()
		return out, loadCalendarViewCmd(out.repo, out.calDate, out.calMode, false, out.uiSettings.weekStartsMonday())
	case "]", "right":
		out := m.calNavNext()
		return out, loadCalendarViewCmd(out.repo, out.calDate, out.calMode, false, out.uiSettings.weekStartsMonday())
	case "h":
		out := m.calMoveHoriz(-1)
		return out, loadCalendarViewCmd(out.repo, out.calDate, out.calMode, false, out.uiSettings.weekStartsMonday())
	case "l":
		out := m.calMoveHoriz(1)
		return out, loadCalendarViewCmd(out.repo, out.calDate, out.calMode, false, out.uiSettings.weekStartsMonday())
	case "j", "down":
		out := m.calMoveVert(1)
		out.calDayCenterNow = false
		if out.calMode == calModeWeek {
			out.calWeekViewport = -2
		}
		return out, nil
	case "k", "up":
		out := m.calMoveVert(-1)
		out.calDayCenterNow = false
		if out.calMode == calModeWeek {
			out.calWeekViewport = -2
		}
		return out, nil
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
			m.followPomoViewport(grid)
		}
		return m, nil
	case "k", "up":
		if len(grid) > 0 {
			m.pomoFollowNow = false
			m.pomoGridCursor = nextPomoGridCursor(grid, m.pomoGridCursor, -1)
			m.syncPomoCursorFromGrid(grid)
			m.followPomoViewport(grid)
		}
		return m, nil
	case "J":
		if len(grid) > 0 {
			m.pomoFollowNow = false
			m.pomoGridCursor = nextSelectablePomoGridRow(grid, m.pomoGridCursor, 1)
			m.syncPomoCursorFromGrid(grid)
			m.followPomoViewport(grid)
		}
		return m, nil
	case "K":
		if len(grid) > 0 {
			m.pomoFollowNow = false
			m.pomoGridCursor = nextSelectablePomoGridRow(grid, m.pomoGridCursor, -1)
			m.syncPomoCursorFromGrid(grid)
			m.followPomoViewport(grid)
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
	case "+", "=":
		m.uiSettings.PomoDailyGoal = min(m.uiSettings.pomoDailyGoal()+1, 999)
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else {
			m.toast = fmt.Sprintf("daily pomodoro goal · %d", m.uiSettings.PomoDailyGoal)
		}
		return m, nil
	case "-":
		m.uiSettings.PomoDailyGoal = max(m.uiSettings.pomoDailyGoal()-1, 1)
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else {
			m.toast = fmt.Sprintf("daily pomodoro goal · %d", m.uiSettings.PomoDailyGoal)
		}
		return m, nil
	case "v", "V":
		design := m.pomoFocusDesign().Next()
		m.uiSettings.PomoFocusDesign = design
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else {
			m.toast = pomoFocusDesignToast(design)
		}
		return m, nil
	case "z", "Z":
		anchorHour := m.pomoVisibleAnchorHour(grid)
		d := m.pomoTimelineDensity().Toggle()
		m.uiSettings.PomoTimelineDensity = d
		if err := saveUISettings(m.uiSettings); err != nil {
			m.errMsg = "settings: " + err.Error()
		} else {
			m.toast = "timeline " + d.Label()
		}
		if m.pomoFollowNow {
			m.centerPomoTimelineOnNow()
		} else {
			newGrid := m.pomoDayGrid()
			m.pomoViewport = gridRowForHour(newGrid, anchorHour)
			m.pomoGridCursor = clamp(m.pomoGridCursor, 0, max(len(newGrid)-1, 0))
			m.followPomoViewport(newGrid)
		}
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
	grid := m.pomoDayGrid()
	if m.pomoGridCursor < 0 || m.pomoGridCursor >= len(grid) {
		return ticktick.FocusRecord{}, false
	}
	row := grid[m.pomoGridCursor]
	if row.kind != "pomo" || row.recIdx < 0 || row.recIdx >= len(recs) {
		return ticktick.FocusRecord{}, false
	}
	return recs[row.recIdx], true
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
	return m, loadFocusPickerCmd(m.repo)
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
	return m, loadFocusPickerCmd(m.repo)
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
	if m.effectiveTaskScope() == TaskScopeArchive {
		return m.archiveTaskRows()
	}
	return buildVisibleTaskRowsForScope(
		m.tasks,
		m.taskSortMode,
		m.effectiveTaskScope(),
		m.filterInput.Value(),
	)
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
	return loadTasksCmd(m.repo, row.node.ID, row.node.Name, false)
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
