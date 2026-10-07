package listarchive

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLatestPendingIsTheNewestUnrestoredList(t *testing.T) {
	store := NewStoreAt(filepath.Join(t.TempDir(), "list-archive.json"))
	older := Record{Kind: "list", Name: "older", Project: json.RawMessage(`{"id":"p1","name":"older"}`)}
	if _, err := store.Put(older); err != nil {
		t.Fatal(err)
	}
	newer := Record{Kind: "list", Name: "newer", Project: json.RawMessage(`{"id":"p2","name":"newer"}`)}
	if _, err := store.Put(newer); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.LatestPending()
	if err != nil || !ok || got.Name != "newer" {
		t.Fatalf("latest=%+v ok=%v err=%v", got, ok, err)
	}
	if err := store.MarkRestored(got.ArchiveID, "new-id"); err != nil {
		t.Fatal(err)
	}
	got, ok, err = store.LatestPending()
	if err != nil || !ok || got.Name != "older" {
		t.Fatalf("after restore latest=%+v ok=%v err=%v", got, ok, err)
	}
}
