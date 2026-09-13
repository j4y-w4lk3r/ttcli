package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
	TaskSortDefault     TaskSortMode            `json:"taskSortDefault,omitempty"`
	TaskSortByProject   map[string]TaskSortMode `json:"taskSortByProject,omitempty"`
	PomoTimelineDensity PomoTimelineDensity     `json:"pomoTimelineDensity,omitempty"`
	TaskDetailLayout    TaskDetailLayout        `json:"taskDetailLayout,omitempty"`
	LastView            string                  `json:"lastView,omitempty"`
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttcli", "tui.json"), nil
}

func loadUISettings() uiSettings {
	path, err := settingsPath()
	if err != nil {
		return uiSettings{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return uiSettings{}
	}
	var s uiSettings
	if json.Unmarshal(b, &s) != nil {
		return uiSettings{}
	}
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
