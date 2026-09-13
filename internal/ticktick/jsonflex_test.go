package ticktick

import (
	"encoding/json"
	"testing"
)

func TestFlexIntUnmarshalFloat(t *testing.T) {
	var h Habit
	raw := `[{"id":"h1","name":"Read","goal":1.0,"totalCheckIns":42.0,"status":0}]`
	if err := json.Unmarshal([]byte(raw), &[]Habit{h}); err != nil {
		// unmarshal into slice properly
	}
	var habits []Habit
	if err := json.Unmarshal([]byte(raw), &habits); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if habits[0].Goal.Int() != 1 {
		t.Fatalf("goal=%d want 1", habits[0].Goal.Int())
	}
	if habits[0].TotalCheckIns.Int() != 42 {
		t.Fatalf("totalCheckIns=%d want 42", habits[0].TotalCheckIns.Int())
	}
}

func TestFlexIntUnmarshalInt(t *testing.T) {
	var h Habit
	if err := json.Unmarshal([]byte(`{"goal":3,"totalCheckIns":10,"status":1}`), &h); err != nil {
		t.Fatal(err)
	}
	if h.Goal.Int() != 3 || h.TotalCheckIns.Int() != 10 {
		t.Fatalf("got goal=%d total=%d", h.Goal.Int(), h.TotalCheckIns.Int())
	}
}
