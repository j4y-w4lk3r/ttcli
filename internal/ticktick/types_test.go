package ticktick

import (
	"encoding/json"
	"testing"
)

const purchaseTaskJSON = `{
  "id": "699a2798d1e55127a22a4da9",
  "title": "Purchase",
  "content": "this is purhcase note...",
  "childIds": ["6a4391f6ba2f512c5006fdd4","6a106db3ceb9d13363674e00","6a106dc5eefe513363674ea2"],
  "items": [],
  "status": 0,
  "priority": 0,
  "deleted": 0
}`

func TestTaskUnmarshalChildIDsAndContent(t *testing.T) {
	var tsk Task
	if err := json.Unmarshal([]byte(purchaseTaskJSON), &tsk); err != nil {
		t.Fatal(err)
	}
	if tsk.Content != "this is purhcase note..." {
		t.Fatalf("content=%q", tsk.Content)
	}
	if len(tsk.ChildIDs) != 3 {
		t.Fatalf("childIds=%v", tsk.ChildIDs)
	}
}

func TestIsSubtask(t *testing.T) {
	child := Task{ParentID: "abc"}
	if !child.IsSubtask() {
		t.Fatal("expected subtask")
	}
	top := Task{}
	if top.IsSubtask() {
		t.Fatal("expected top-level")
	}
}
