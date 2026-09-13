package ticktick

import "testing"

func TestFindTaskInRawList(t *testing.T) {
	tasks := []map[string]any{
		{"id": "a", "title": "one"},
		{"id": "b", "title": "two", "status": float64(2)},
	}
	if task, ok := findTaskInRawList(tasks, "b"); !ok || task["title"] != "two" {
		t.Fatalf("got %v ok=%v", task, ok)
	}
	if _, ok := findTaskInRawList(tasks, "missing"); ok {
		t.Fatal("expected miss")
	}
}
