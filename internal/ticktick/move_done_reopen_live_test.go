//go:build live

package ticktick

import "testing"

func TestMoveCompletedTaskLive(t *testing.T) {
	c, err := New("", CredentialOptions{})
	if err != nil {
		t.Skip(err)
	}
	const list0 = "699a273ed216d127a22a4b39"
	inboxID, err := c.InboxID()
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := c.ProjectTasks(list0)
	if err != nil {
		t.Fatal(err)
	}
	var target *Task
	for i := range tasks {
		if tasks[i].Done() && tasks[i].Status.Int() == 2 {
			target = &tasks[i]
			break
		}
	}
	if target == nil {
		t.Skip("no completed task in List0")
	}
	origID := target.ID
	t.Logf("moving completed %q", target.Title)

	res, err := c.MoveTask(origID, list0, inboxID)
	if err != nil {
		t.Fatal(err)
	}

	afterList0, _ := c.ProjectTasks(list0)
	for _, task := range afterList0 {
		if task.ID == origID {
			t.Fatal("task still in source list")
		}
	}

	task, err := c.findTaskRawByID(res.NewTaskID, inboxID)
	if err != nil {
		t.Fatalf("not in inbox: %v", err)
	}
	if taskMapStatus(task) != 2 {
		t.Fatalf("expected completed in inbox, status=%v", task["status"])
	}
}

func TestTaskMapStatus(t *testing.T) {
	if taskMapStatus(map[string]any{"status": float64(2)}) != 2 {
		t.Fatal("float64")
	}
	if taskMapStatus(map[string]any{"status": 0}) != 0 {
		t.Fatal("int")
	}
}
