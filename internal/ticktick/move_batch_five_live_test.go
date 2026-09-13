//go:build live

package ticktick

import "testing"

func TestMoveFiveCompletedTasksLive(t *testing.T) {
	c, err := New("", CredentialOptions{})
	if err != nil {
		t.Skip(err)
	}
	const list0 = "699a273ed216d127a22a4b39"
	inboxID, _ := c.InboxID()

	var ids []string
	for i := 0; i < 5; i++ {
		id, err := c.AddTask("zz-batch-move-probe", list0, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := c.CompleteTask(list0, id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_ = c.DeleteTask(list0, id)
			_ = c.DeleteTask(inboxID, id)
		}
	})

	n, err := c.MoveTasks(ids, list0, inboxID)
	if err != nil {
		t.Fatalf("MoveTasks moved=%d err=%v", n, err)
	}
	if n != len(ids) {
		t.Fatalf("moved %d want %d", n, len(ids))
	}
}
