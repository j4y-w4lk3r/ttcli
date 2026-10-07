package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMoveListStepUsesTheGapBetweenNeighbors(t *testing.T) {
	projects := []Project{
		{ID: "a", GroupID: "x", SortOrder: 10},
		{ID: "b", GroupID: "x", SortOrder: 30},
		{ID: "c", GroupID: "x", SortOrder: 50},
		{ID: "z", GroupID: "NONE", SortOrder: 1},
	}
	place := MoveListStep(projects, "c", -1)
	if !place.Active() || len(place.Orders) != 1 {
		t.Fatalf("place=%+v", place)
	}
	if place.Orders["c"] <= 10 || place.Orders["c"] >= 30 {
		t.Fatalf("order=%d", place.Orders["c"])
	}
	if MoveListStep(projects, "a", -1).Active() {
		t.Fatal("first list moved up")
	}
	if MoveListStep(projects, "c", 1).Active() {
		t.Fatal("last list moved down")
	}
}

func TestMoveListStepRenumbersWhenOrdersTie(t *testing.T) {
	projects := []Project{
		{ID: "a", GroupID: "x", SortOrder: 0},
		{ID: "b", GroupID: "x", SortOrder: 0},
		{ID: "c", GroupID: "x", SortOrder: 0},
	}
	place := MoveListStep(projects, "c", -1)
	if !place.Active() {
		t.Fatal("inactive")
	}
	if !(place.Orders["a"] < place.Orders["c"] && place.Orders["c"] < place.Orders["b"]) {
		t.Fatalf("orders=%v", place.Orders)
	}
}

func TestMoveListToSlotPicksAPlaceInsideTheFolder(t *testing.T) {
	projects := []Project{
		{ID: "zero", GroupID: "NONE", SortOrder: 5},
		{ID: "ideas", GroupID: "x", SortOrder: 10},
		{ID: "tech", GroupID: "x", SortOrder: 30},
	}
	before := MoveListToSlot(projects, "zero", "x", "tech", false)
	if !before.SetGroup || before.GroupID != "x" {
		t.Fatalf("group=%+v", before)
	}
	if before.Orders["zero"] <= 10 || before.Orders["zero"] >= 30 {
		t.Fatalf("before tech=%d", before.Orders["zero"])
	}
	top := MoveListToSlot(projects, "zero", "x", "", false)
	if top.Orders["zero"] >= 10 {
		t.Fatalf("top=%d", top.Orders["zero"])
	}
	end := MoveListToSlot(projects, "zero", "x", "", true)
	if end.Orders["zero"] <= 30 {
		t.Fatalf("end=%d", end.Orders["zero"])
	}
}

func TestMoveListVisualEntersTheFolderAbove(t *testing.T) {
	groups := []ProjectGroup{{ID: "x", Name: "x", SortOrder: 1}}
	projects := []Project{
		{ID: "ideas", GroupID: "x", SortOrder: 10},
		{ID: "tech", GroupID: "x", SortOrder: 30},
		{ID: "zero", GroupID: "NONE", SortOrder: 5},
	}
	place := MoveListVisual(projects, groups, "zero", -1)
	if !place.SetGroup || place.GroupID != "x" || place.Orders["zero"] <= 30 {
		t.Fatalf("place=%+v", place)
	}
	projects[2].GroupID = place.GroupID
	projects[2].SortOrder = place.Orders["zero"]
	again := MoveListVisual(projects, groups, "zero", -1)
	if again.Orders["zero"] <= 10 || again.Orders["zero"] >= 30 {
		t.Fatalf("again=%+v", again)
	}
}

func TestPlacementFromProjectsKeepsTheFinalSpot(t *testing.T) {
	before := []Project{{ID: "zero", GroupID: "NONE", SortOrder: 5}}
	after := []Project{{ID: "zero", GroupID: "x", SortOrder: 40}}
	place := PlacementFromProjects(before, after, "zero")
	if place.GroupID != "x" || place.Orders["zero"] != 40 {
		t.Fatalf("place=%+v", place)
	}
	if PlacementFromProjects(before, before, "zero").Active() {
		t.Fatal("unchanged placement was active")
	}
}

func TestMoveFolderStep(t *testing.T) {
	groups := []ProjectGroup{
		{ID: "x", Name: "x", SortOrder: 10},
		{ID: "y", Name: "y", SortOrder: 30},
	}
	place := MoveFolderStep(groups, "y", -1)
	if !place.Active() || place.Orders["y"] >= 10 {
		t.Fatalf("place=%+v", place)
	}
	if MoveFolderStep(groups, "x", -1).Active() {
		t.Fatal("first folder moved up")
	}
}

func TestApplyProjectOrdersSetsFolderAndSort(t *testing.T) {
	const (
		list   = "aaaaaaaaaaaaaaaaaaaaaaaa"
		ideas  = "bbbbbbbbbbbbbbbbbbbbbbbb"
		folder = "cccccccccccccccccccccccc"
	)
	var body batchProjectPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": list, "name": "0", "groupId": "NONE", "sortOrder": 5},
				{"id": ideas, "name": "ideas", "groupId": folder, "sortOrder": 10},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/project":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.ApplyProjectOrders(ProjectPlacement{
		MovedID:  list,
		GroupID:  folder,
		SetGroup: true,
		Orders:   map[string]int64{list: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Update) != 1 {
		t.Fatalf("update=%v", body.Update)
	}
	item, _ := body.Update[0].(map[string]any)
	if item["id"] != list || item["groupId"] != folder || item["sortOrder"] != float64(20) {
		t.Fatalf("item=%v", item)
	}
}
