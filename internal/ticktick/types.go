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

// Task is a TickTick task. The private API uses integer enums:
//
//	priority: 0 none, 1 low, 3 medium, 5 high
//	status:   0 todo, 2 completed (-1 won't-do on some clients)
//	deleted:  0 live, 1/2 trashed
type Task struct {
	ID         string   `json:"id"`
	ProjectID  string   `json:"projectId"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Priority   int      `json:"priority"`
	Status     int      `json:"status"`
	Deleted    int      `json:"deleted"`
	DueDate    string   `json:"dueDate"`
	StartDate  string   `json:"startDate"`
	Tags       []string `json:"tags"`
	SortOrder  int64    `json:"sortOrder"`
	IsAllDay   bool     `json:"isAllDay"`
	TimeZone   string   `json:"timeZone"`
	CompletedT string   `json:"completedTime"`
}

// PriorityLabel renders the integer priority as a short word.
func (t Task) PriorityLabel() string {
	switch {
	case t.Priority >= 5:
		return "high"
	case t.Priority >= 3:
		return "med"
	case t.Priority >= 1:
		return "low"
	default:
		return "-"
	}
}

// Done reports whether the task is completed.
func (t Task) Done() bool { return t.Status != 0 }
