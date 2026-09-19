package tui

import (
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/taskarchive"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const archiveTaskIDPrefix = "archive:"

func (m model) effectiveTaskScope() TaskScope {
	scope := normalizeTaskScope(m.taskScope)
	// Preserve compatibility with older in-memory callers while persisted UI
	// state uses the explicit scope.
	if scope == TaskScopeOpen {
		if m.showDeleted || m.showCompleted {
			return TaskScopeAll
		}
	}
	return scope
}

func (m model) setTaskScope(scope TaskScope) model {
	scope = normalizeTaskScope(scope)
	m.taskScope = scope
	m.showCompleted = false
	m.showDeleted = false
	m.taskCursor = 0
	m.taskMarked = nil
	m.uiSettings.TaskScope = scope
	if m.client == nil {
		m.toast = "task scope · " + scope.Label()
	} else if err := saveUISettings(m.uiSettings); err != nil {
		m.errMsg = "settings: " + err.Error()
	} else {
		m.toast = "task scope · " + scope.Label()
	}
	m.applyFilter()
	return m
}

func archiveTaskID(record taskarchive.Record) string {
	return archiveTaskIDPrefix + record.ArchiveID
}

func (m model) archiveTaskRows() []taskListRow {
	filter := m.filterInput.Value()
	rows := make([]taskListRow, 0, len(m.archiveRecords))
	for _, record := range m.archiveRecords {
		if m.projectID != "" && record.Task.ProjectID != m.projectID {
			continue
		}
		task := record.Task
		if !taskMatchesFilter(task, filter) {
			continue
		}
		task.ID = archiveTaskID(record)
		task.Deleted = 2
		if record.NewTaskID != "" {
			task.Title = strings.TrimSpace(task.Title) + " · recreated"
		}
		rows = append(rows, taskListRow{Task: task})
	}
	return rows
}

func (m model) archiveRecordForTaskID(taskID string) (taskarchive.Record, bool) {
	if !strings.HasPrefix(taskID, archiveTaskIDPrefix) {
		return taskarchive.Record{}, false
	}
	archiveID := strings.TrimPrefix(taskID, archiveTaskIDPrefix)
	for _, record := range m.archiveRecords {
		if record.ArchiveID == archiveID {
			return record, true
		}
	}
	return taskarchive.Record{}, false
}

func (m model) taskScopeStats() (total, matching, shown int) {
	scope := m.effectiveTaskScope()
	if scope == TaskScopeArchive {
		for _, record := range m.archiveRecords {
			if m.projectID != "" && record.Task.ProjectID != m.projectID {
				continue
			}
			total++
			if taskMatchesFilter(record.Task, m.filterInput.Value()) {
				matching++
			}
		}
		return total, matching, len(m.archiveTaskRows())
	}
	for _, task := range m.tasks {
		if !taskMatchesScope(task, scope) {
			continue
		}
		total++
		if taskMatchesFilter(task, m.filterInput.Value()) {
			matching++
		}
	}
	return total, matching, len(buildVisibleTaskRowsForScope(
		m.tasks, m.taskSortMode, scope, m.filterInput.Value(),
	))
}

func groupTasksByProject(tasks []ticktick.Task, fallbackProjectID string) map[string][]string {
	grouped := make(map[string][]string)
	for _, task := range tasks {
		projectID := task.ProjectID
		if projectID == "" {
			projectID = fallbackProjectID
		}
		grouped[projectID] = append(grouped[projectID], task.ID)
	}
	return grouped
}
