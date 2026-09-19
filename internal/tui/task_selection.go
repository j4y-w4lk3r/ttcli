package tui

import (
	"fmt"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func (m model) isTaskMarked(id string) bool {
	if m.taskMarked == nil {
		return false
	}
	_, ok := m.taskMarked[id]
	return ok
}

func (m model) markedTaskCount() int {
	return len(m.taskMarked)
}

func (m model) toggleTaskMark(id string) model {
	if m.taskMarked == nil {
		m.taskMarked = make(map[string]struct{})
	}
	if _, ok := m.taskMarked[id]; ok {
		delete(m.taskMarked, id)
	} else {
		m.taskMarked[id] = struct{}{}
	}
	return m
}

func (m model) toggleTaskMarkAndAdvance() model {
	tasks := m.visibleTasks()
	if len(tasks) == 0 || m.taskCursor < 0 || m.taskCursor >= len(tasks) {
		return m
	}
	m = m.toggleTaskMark(tasks[m.taskCursor].ID)
	if m.taskCursor < len(tasks)-1 {
		m.taskCursor++
	}
	return m
}

func (m model) clearTaskMarks() model {
	m.taskMarked = nil
	return m
}

func (m model) markAllVisibleTasks() model {
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return m
	}
	if m.taskMarked == nil {
		m.taskMarked = make(map[string]struct{}, len(tasks))
	}
	for _, t := range tasks {
		m.taskMarked[t.ID] = struct{}{}
	}
	return m
}

func (m model) markAllVisibleDoneTasks() model {
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return m
	}
	if m.taskMarked == nil {
		m.taskMarked = make(map[string]struct{})
	}
	for _, t := range tasks {
		if t.Done() {
			m.taskMarked[t.ID] = struct{}{}
		}
	}
	return m
}

func (m model) markedTasksFromList() []ticktick.Task {
	if m.markedTaskCount() == 0 {
		return nil
	}
	var marked []ticktick.Task
	for _, t := range m.tasks {
		if !m.isTaskMarked(t.ID) {
			continue
		}
		marked = append(marked, t)
	}
	return marked
}

func (m model) tasksToMove() []ticktick.Task {
	if marked := m.markedTasksFromList(); len(marked) > 0 {
		return marked
	}
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return nil
	}
	if m.taskCursor >= 0 && m.taskCursor < len(tasks) {
		return []ticktick.Task{tasks[m.taskCursor]}
	}
	return nil
}

func (m model) tasksToComplete() []ticktick.Task {
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return nil
	}
	var marked []ticktick.Task
	for _, t := range tasks {
		if m.isTaskMarked(t.ID) && !t.Done() && !t.Trashed() {
			marked = append(marked, t)
		}
	}
	if len(marked) > 0 {
		return marked
	}
	if m.taskCursor >= 0 && m.taskCursor < len(tasks) {
		t := tasks[m.taskCursor]
		if !t.Done() && !t.Trashed() {
			return []ticktick.Task{t}
		}
	}
	return nil
}

func (m model) tasksToReopen() []ticktick.Task {
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return nil
	}
	var marked []ticktick.Task
	for _, t := range tasks {
		if m.isTaskMarked(t.ID) && t.Done() && !t.Trashed() {
			marked = append(marked, t)
		}
	}
	if len(marked) > 0 {
		return marked
	}
	if m.taskCursor >= 0 && m.taskCursor < len(tasks) {
		t := tasks[m.taskCursor]
		if t.Done() && !t.Trashed() {
			return []ticktick.Task{t}
		}
	}
	return nil
}

func (m model) tasksToDelete() []ticktick.Task {
	if marked := m.markedTasksFromList(); len(marked) > 0 {
		return marked
	}
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return nil
	}
	if m.taskCursor >= 0 && m.taskCursor < len(tasks) {
		return []ticktick.Task{tasks[m.taskCursor]}
	}
	return nil
}

func (m model) markedTasksHint() string {
	n := m.markedTaskCount()
	if n == 0 {
		return ""
	}
	if n == 1 {
		return "1 marked · d done · m move · Backspace delete · u clear"
	}
	return fmt.Sprintf("%d marked · d done · m move · Backspace delete · u clear", n)
}
