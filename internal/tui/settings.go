package tui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/j4y-w4lk3r/ttcli/internal/planning"
)

type TaskScope string

const (
	TaskScopeOpen    TaskScope = "open"
	TaskScopeDone    TaskScope = "done"
	TaskScopeTrash   TaskScope = "trash"
	TaskScopeAll     TaskScope = "all"
	TaskScopeArchive TaskScope = "archive"
)

func normalizeTaskScope(scope TaskScope) TaskScope {
	switch scope {
	case TaskScopeOpen, TaskScopeDone, TaskScopeTrash, TaskScopeAll, TaskScopeArchive:
		return scope
	default:
		return TaskScopeOpen
	}
}

func (s TaskScope) Label() string {
	switch normalizeTaskScope(s) {
	case TaskScopeDone:
		return "Done"
	case TaskScopeTrash:
		return "Trash"
	case TaskScopeAll:
		return "All"
	case TaskScopeArchive:
		return "Archive"
	default:
		return "Open"
	}
}

func (s TaskScope) Next() TaskScope {
	switch normalizeTaskScope(s) {
	case TaskScopeOpen:
		return TaskScopeDone
	case TaskScopeDone:
		return TaskScopeTrash
	case TaskScopeTrash:
		return TaskScopeAll
	case TaskScopeAll:
		return TaskScopeArchive
	default:
		return TaskScopeOpen
	}
}

func (s TaskScope) Prev() TaskScope {
	switch normalizeTaskScope(s) {
	case TaskScopeOpen:
		return TaskScopeArchive
	case TaskScopeDone:
		return TaskScopeOpen
	case TaskScopeTrash:
		return TaskScopeDone
	case TaskScopeAll:
		return TaskScopeTrash
	default:
		return TaskScopeAll
	}
}

// TaskSortMode controls how tasks are ordered within a list.
type TaskSortMode string

const (
	TaskSortCustom   TaskSortMode = "custom"
	TaskSortDue      TaskSortMode = "due"
	TaskSortPriority TaskSortMode = "priority"
	TaskSortTitle    TaskSortMode = "title"
)

func (m TaskSortMode) Label() string {
	switch m {
	case TaskSortDue:
		return "due date"
	case TaskSortPriority:
		return "priority"
	case TaskSortTitle:
		return "title"
	default:
		return "custom"
	}
}

func (m TaskSortMode) Next() TaskSortMode {
	switch m {
	case TaskSortCustom:
		return TaskSortDue
	case TaskSortDue:
		return TaskSortPriority
	case TaskSortPriority:
		return TaskSortTitle
	default:
		return TaskSortCustom
	}
}

func defaultTaskSortForProject(projectID, inboxID string) TaskSortMode {
	if inboxID != "" && projectID == inboxID {
		return TaskSortDue
	}
	return TaskSortCustom
}

type PomoTimelineDensity string

const (
	PomoDensityStretch PomoTimelineDensity = "stretch"
	PomoDensityCompact PomoTimelineDensity = "compact"
)

func (d PomoTimelineDensity) Label() string {
	if d == PomoDensityCompact {
		return "compact"
	}
	return "stretch"
}

func (d PomoTimelineDensity) Toggle() PomoTimelineDensity {
	if d == PomoDensityCompact {
		return PomoDensityStretch
	}
	return PomoDensityCompact
}

type PomoFocusDesign string

const (
	PomoFocusArc  PomoFocusDesign = "arc"
	PomoFocusBar  PomoFocusDesign = "bar"
	PomoFocusCard PomoFocusDesign = "card"
)

func (d PomoFocusDesign) Label() string {
	switch d {
	case PomoFocusBar:
		return "focus bar"
	case PomoFocusCard:
		return "status card"
	default:
		return "segmented arc"
	}
}

func (d PomoFocusDesign) Next() PomoFocusDesign {
	switch d {
	case PomoFocusArc:
		return PomoFocusBar
	case PomoFocusBar:
		return PomoFocusCard
	default:
		return PomoFocusArc
	}
}

func normalizePomoFocusDesign(design PomoFocusDesign) PomoFocusDesign {
	switch design {
	case PomoFocusArc, PomoFocusBar, PomoFocusCard:
		return design
	default:
		return PomoFocusArc
	}
}

type TaskDetailLayout string

const (
	TaskDetailBottom TaskDetailLayout = "bottom"
	TaskDetailSide   TaskDetailLayout = "side"
)

func (l TaskDetailLayout) Label() string {
	if l == TaskDetailSide {
		return "side"
	}
	return "bottom"
}

func (l TaskDetailLayout) Toggle() TaskDetailLayout {
	if l == TaskDetailSide {
		return TaskDetailBottom
	}
	return TaskDetailSide
}

func parseTaskDetailLayout(s string) TaskDetailLayout {
	if s == string(TaskDetailSide) {
		return TaskDetailSide
	}
	return TaskDetailBottom
}

func appViewName(v appView) string {
	switch v {
	case viewCalendar:
		return "calendar"
	case viewPomodoro:
		return "pomo"
	case viewHabits:
		return "habits"
	default:
		return "tasks"
	}
}

func parseAppView(s string) appView {
	switch s {
	case "calendar":
		return viewCalendar
	case "pomo":
		return viewPomodoro
	case "habits":
		return viewHabits
	default:
		return viewTasks
	}
}

type uiSettings struct {
	TaskSortDefault        TaskSortMode            `json:"taskSortDefault,omitempty"`
	TaskSortByProject      map[string]TaskSortMode `json:"taskSortByProject,omitempty"`
	TaskScope              TaskScope               `json:"taskScope,omitempty"`
	PomoTimelineDensity    PomoTimelineDensity     `json:"pomoTimelineDensity,omitempty"`
	CalendarWeekDensity    PomoTimelineDensity     `json:"calendarWeekDensity,omitempty"`
	PomoFocusDesign        PomoFocusDesign         `json:"pomoFocusDesign,omitempty"`
	PomoDailyGoal          int                     `json:"pomoDailyGoal,omitempty"`
	TaskDetailLayout       TaskDetailLayout        `json:"taskDetailLayout,omitempty"`
	LastView               string                  `json:"lastView,omitempty"`
	WorkStart              string                  `json:"workStart,omitempty"`
	WorkEnd                string                  `json:"workEnd,omitempty"`
	PlanningBufferMinutes  int                     `json:"planningBufferMinutes,omitempty"`
	DefaultTaskMinutes     int                     `json:"defaultTaskMinutes,omitempty"`
	WeekStartsOn           string                  `json:"weekStartsOn,omitempty"`
	CalendarDayShowOverdue bool                    `json:"calendarDayShowOverdue,omitempty"`
}

func (s *uiSettings) applyPlanningDefaults() {
	s.TaskScope = normalizeTaskScope(s.TaskScope)
	if _, err := planning.ParseClockMinutes(s.WorkStart); err != nil {
		s.WorkStart = "09:00"
	}
	if _, err := planning.ParseClockMinutes(s.WorkEnd); err != nil {
		s.WorkEnd = "18:00"
	}
	start, _ := planning.ParseClockMinutes(s.WorkStart)
	end, _ := planning.ParseClockMinutes(s.WorkEnd)
	if end <= start {
		s.WorkStart = "09:00"
		s.WorkEnd = "18:00"
	}
	if s.PlanningBufferMinutes < 0 || s.PlanningBufferMinutes > 12*60 {
		s.PlanningBufferMinutes = planning.DefaultBufferMinutes
	}
	if s.DefaultTaskMinutes < 1 || s.DefaultTaskMinutes > 24*60 {
		s.DefaultTaskMinutes = planning.DefaultTaskMinutes
	}
	s.WeekStartsOn = "monday"
	if s.CalendarWeekDensity != PomoDensityCompact {
		s.CalendarWeekDensity = PomoDensityStretch
	}
}

func (s *uiSettings) applyPomoDefaults() {
	s.PomoFocusDesign = normalizePomoFocusDesign(s.PomoFocusDesign)
	if s.PomoDailyGoal < 1 || s.PomoDailyGoal > 999 {
		s.PomoDailyGoal = dailyPomoGoal
	}
}

func (s uiSettings) pomoDailyGoal() int {
	if s.PomoDailyGoal < 1 {
		return dailyPomoGoal
	}
	return s.PomoDailyGoal
}

func (s uiSettings) planningConfig() planning.Config {
	start, err := planning.ParseClockMinutes(s.WorkStart)
	if err != nil {
		start = planning.DefaultWorkStartMinutes
	}
	end, err := planning.ParseClockMinutes(s.WorkEnd)
	if err != nil {
		end = planning.DefaultWorkEndMinutes
	}
	return planning.Config{
		WorkStartMinutes: start,
		WorkEndMinutes:   end,
		BufferMinutes:    s.PlanningBufferMinutes,
		DefaultMinutes:   s.DefaultTaskMinutes,
	}.Normalized()
}

func (s uiSettings) weekStartsMonday() bool {
	return true
}

func (s uiSettings) calendarWeekDensity() PomoTimelineDensity {
	if s.CalendarWeekDensity == PomoDensityCompact {
		return PomoDensityCompact
	}
	return PomoDensityStretch
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttcli", "tui.json"), nil
}

func defaultUISettings() uiSettings {
	return uiSettings{
		WorkStart:             "09:00",
		WorkEnd:               "18:00",
		PlanningBufferMinutes: planning.DefaultBufferMinutes,
		DefaultTaskMinutes:    planning.DefaultTaskMinutes,
		WeekStartsOn:          "monday",
		CalendarWeekDensity:   PomoDensityStretch,
		PomoFocusDesign:       PomoFocusArc,
		PomoDailyGoal:         dailyPomoGoal,
		TaskScope:             TaskScopeOpen,
	}
}

func loadUISettings() uiSettings {
	s := defaultUISettings()
	path, err := settingsPath()
	if err != nil {
		return s
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if json.Unmarshal(b, &s) != nil {
		return defaultUISettings()
	}
	s.applyPlanningDefaults()
	s.applyPomoDefaults()
	return s
}

func saveUISettings(s uiSettings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func (s *uiSettings) sortForProject(projectID, inboxID string) TaskSortMode {
	if s.TaskSortByProject != nil {
		if mode, ok := s.TaskSortByProject[projectID]; ok && mode != "" {
			return mode
		}
	}
	if s.TaskSortDefault != "" {
		return s.TaskSortDefault
	}
	return defaultTaskSortForProject(projectID, inboxID)
}

func (s *uiSettings) setSortForProject(projectID string, mode TaskSortMode) {
	if s.TaskSortByProject == nil {
		s.TaskSortByProject = map[string]TaskSortMode{}
	}
	s.TaskSortByProject[projectID] = mode
}
