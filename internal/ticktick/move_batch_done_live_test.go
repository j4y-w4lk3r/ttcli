//go:build live

package ticktick

import "testing"

func TestMoveMultipleCompletedTasksLive(t *testing.T) {
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
	var ids []string
	for _, task := range tasks {
		if task.Done() && task.Status.Int() == 2 {
			ids = append(ids, task.ID)
		}
		if len(ids) == 3 {
			break
		}
	}
	if len(ids) < 2 {
		t.Skip("need at least 2 completed tasks")
	}
	t.Logf("moving %d completed tasks: %v", len(ids), ids)

	n, err := c.MoveTasks(ids, list0, inboxID)
	if err != nil {
		t.Fatalf("MoveTasks: moved=%d err=%v", n, err)
	}
	if n != len(ids) {
		t.Fatalf("moved %d want %d", n, len(ids))
	}

	after, _ := c.ProjectTasks(list0)
	for _, task := range after {
		for _, id := range ids {
			if task.ID == id {
				t.Fatalf("task %s still in List0", id)
			}
		}
	}
}
