package taskarchive

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestStorePreservesFullRawSnapshotAndRecreationState(t *testing.T) {
	store := NewStoreAt(filepath.Join(t.TempDir(), "state", "archive.json"))
	record := Record{
		Task: ticktick.Task{
			ID: "task-1", ProjectID: "project-1", Title: "Recover me",
			Content: "notes", Tags: []string{"home"},
		},
		RawTask: json.RawMessage(`{"id":"task-1","projectId":"project-1","serverOnly":"keep-me"}`),
	}
	if err := store.Put(record); err != nil {
		t.Fatal(err)
	}
	records, err := store.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ArchiveID != "project-1:task-1" {
		t.Fatalf("records=%+v", records)
	}
	var raw map[string]any
	if err := json.Unmarshal(records[0].RawTask, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["serverOnly"] != "keep-me" {
		t.Fatalf("full snapshot lost unknown field: %+v", raw)
	}
	if err := store.MarkRecreated(records[0].ArchiveID, "task-2"); err != nil {
		t.Fatal(err)
	}
	records, err = store.Records()
	if err != nil {
		t.Fatal(err)
	}
	if records[0].NewTaskID != "task-2" || records[0].RecreatedAt.IsZero() {
		t.Fatalf("recreation state=%+v", records[0])
	}
}

func TestStoreRejectsInvalidRawSnapshot(t *testing.T) {
	store := NewStoreAt(filepath.Join(t.TempDir(), "archive.json"))
	err := store.Put(Record{
		Task:    ticktick.Task{ID: "task-1", ProjectID: "project-1"},
		RawTask: json.RawMessage(`{broken`),
	})
	if err == nil {
		t.Fatal("expected invalid snapshot to be rejected")
	}
}
