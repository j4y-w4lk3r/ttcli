package ticktick

// Project is a TickTick list/project as returned by /api/v2/projects.
// Only the fields we render or act on are mapped; the API returns more.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Kind      string `json:"kind"` // "TASK" or "NOTE"
	Closed    *bool  `json:"closed"`
	GroupID   string `json:"groupId"`
	SortOrder int64  `json:"sortOrder"`
}

// ChecklistItem is a subtask/checklist entry on a task.
type ChecklistItem struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Status flexInt `json:"status"` // 0 open, 1 done
}

// Done reports whether the checklist item is completed.
func (i ChecklistItem) Done() bool { return i.Status.Int() == 1 }

// Task is a TickTick task. The private API uses integer enums:
//
//	priority: 0 none, 1 low, 3 medium, 5 high
//	status:   0 todo, 2 completed (-1 won't-do on some clients)
//	deleted:  0 live, 1/2 trashed
type Task struct {
	ID         string   `json:"id"`
	ProjectID  string   `json:"projectId"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	Desc       string          `json:"desc"`
	Priority   flexInt         `json:"priority"`
	Status     flexInt         `json:"status"`
	Deleted    flexInt         `json:"deleted"`
	DueDate    string   `json:"dueDate"`
	StartDate  string   `json:"startDate"`
	Reminder   string   `json:"reminder"`
	Tags       []string `json:"tags"`
	SortOrder  int64    `json:"sortOrder"`
	IsAllDay   bool     `json:"isAllDay"`
	TimeZone   string   `json:"timeZone"`
	CompletedT string          `json:"completedTime"`
	ParentID   string          `json:"parentId"`
	ChildIDs   []string        `json:"childIds"`
	Items      []ChecklistItem `json:"items"`
}

// IsSubtask reports whether this task is a child of another task.
func (t Task) IsSubtask() bool { return t.ParentID != "" }

// PriorityLabel renders the integer priority as a short word.
func (t Task) PriorityLabel() string {
	switch {
	case t.Priority.Int() >= 5:
		return "high"
	case t.Priority.Int() >= 3:
		return "med"
	case t.Priority.Int() >= 1:
		return "low"
	default:
		return "-"
	}
}

// Trashed reports whether the task is in the trash (soft-deleted in TickTick).
func (t Task) Trashed() bool { return t.Deleted.Int() != 0 }

// Done reports whether the task is completed.
func (t Task) Done() bool { return t.Status.Int() != 0 }
