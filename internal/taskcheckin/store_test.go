package taskcheckin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePutToggleRemoveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "task-checkins.json")
	store := NewStoreAt(path)
	record := Record{
		TaskID: "task-1", SeriesID: "task-1", ProjectID: "home",
		Title: "Groceries", Date: "2026-09-17",
		CompletedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local),
	}
	if err := store.Put(record); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.On("task-1", "2026-09-17")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Title != "Groceries" {
		t.Fatalf("record=%+v ok=%v", got, ok)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode=%o want 600", mode)
	}

	checked, err := store.Toggle(record)
	if err != nil {
		t.Fatal(err)
	}
	if checked {
		t.Fatal("toggle should remove existing check-in")
	}
	if _, ok, err := store.On("task-1", "2026-09-17"); err != nil || ok {
		t.Fatalf("record still present ok=%v err=%v", ok, err)
	}

	checked, err = store.Toggle(record)
	if err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("toggle should insert missing check-in")
	}
	if err := store.Remove("task-1", "2026-09-17"); err != nil {
		t.Fatal(err)
	}
	records, err := store.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("records=%+v", records)
	}
}

func TestStoreRejectsInvalidDate(t *testing.T) {
	store := NewStoreAt(filepath.Join(t.TempDir(), "task-checkins.json"))
	err := store.Put(Record{TaskID: "task-1", Date: "tomorrow"})
	if err == nil {
		t.Fatal("expected invalid date error")
	}
}
