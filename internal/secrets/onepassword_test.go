package secrets

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseLoginFields(t *testing.T) {
	const sample = `{
  "title": "TickTick",
  "category": "LOGIN",
  "fields": [
    {"purpose": "USERNAME", "label": "username", "value": "you@example.com"},
    {"purpose": "PASSWORD", "label": "password", "value": "s3cret"}
  ]
}`
	user, pass, err := parseLoginFields(sample)
	if err != nil {
		t.Fatal(err)
	}
	if user != "you@example.com" || pass != "s3cret" {
		t.Fatalf("got %q / %q", user, pass)
	}
}

func TestParseLoginFields_labelFallback(t *testing.T) {
	const sample = `{
  "fields": [
    {"label": "email", "value": "a@b.c"},
    {"label": "password", "value": "pw"}
  ]
}`
	user, pass, err := parseLoginFields(sample)
	if err != nil {
		t.Fatal(err)
	}
	if user != "a@b.c" || pass != "pw" {
		t.Fatalf("got %q / %q", user, pass)
	}
}

func TestScoreLoginItem(t *testing.T) {
	it := loginListItem{
		Title: "TickTick",
		URLs:  urlsFromHref("https://ticktick.com"),
	}
	if scoreLoginItem(it, "ticktick") != 130 {
		t.Fatalf("expected high score for exact title + URL, got %d", scoreLoginItem(it, "ticktick"))
	}
	if scoreLoginItem(loginListItem{Title: "Other"}, "ticktick") >= 0 {
		t.Fatal("unrelated item should not match")
	}
}

func TestPickBestLoginMatch(t *testing.T) {
	const listOut = `[
  {"id":"a","title":"Wi-Fi","vault":{"id":"v1","name":"Private"}},
  {"id":"b","title":"TickTick","vault":{"id":"v2","name":"Employee"},"urls":[{"href":"https://ticktick.com","primary":true}]},
  {"id":"c","title":"TickTick","vault":{"id":"v3","name":"Archive"},"urls":[{"href":"https://example.com"}]}
]`
	loc, err := pickBestLoginMatch(listOut, "", "TickTick")
	if err != nil {
		t.Fatal(err)
	}
	if loc.ID != "b" || loc.Vault != "Employee" {
		t.Fatalf("got %+v, want Employee/TickTick id=b", loc)
	}
}

func TestPickBestLoginMatch_vaultFilter(t *testing.T) {
	const listOut = `[
  {"id":"b","title":"TickTick","vault":{"id":"v2","name":"Employee"},"urls":[{"href":"https://ticktick.com"}]}
]`
	loc, err := pickBestLoginMatch(listOut, "Employee", "TickTick")
	if err != nil {
		t.Fatal(err)
	}
	if loc.Vault != "Employee" {
		t.Fatalf("got %+v", loc)
	}
}

func TestMemoryGetLogin_discover(t *testing.T) {
	m := NewMemory()
	m.SetLoginURL("Employee", "TickTick", "u@x.y", "pw", "https://ticktick.com")
	m.SetLogin("Private", "Other", "no@pe.com", "nope")

	user, pass, err := m.GetLogin("", "TickTick")
	if err != nil || user != "u@x.y" || pass != "pw" {
		t.Fatalf("got %q %q err=%v", user, pass, err)
	}
}

// pickBestLoginMatch parses `op item list --format json` output without shelling out.
func pickBestLoginMatch(listOut, vaultFilter, title string) (loginLocation, error) {
	var items []loginListItem
	if err := json.Unmarshal([]byte(listOut), &items); err != nil {
		return loginLocation{}, err
	}
	want := strings.ToLower(strings.TrimSpace(title))
	var matches []loginLocation
	for _, it := range items {
		if vaultFilter != "" && !vaultMatches(vaultFilter, it.Vault.Name) && !vaultMatches(vaultFilter, it.Vault.ID) {
			continue
		}
		score := scoreLoginItem(it, want)
		if score < 0 {
			continue
		}
		vault := it.Vault.Name
		if vault == "" {
			vault = it.Vault.ID
		}
		matches = append(matches, loginLocation{ID: it.ID, Title: it.Title, Vault: vault, Score: score})
	}
	if len(matches) == 0 {
		return loginLocation{}, fmt.Errorf("no match")
	}
	best := matches[0]
	for _, m := range matches[1:] {
		if m.Score > best.Score {
			best = m
		}
	}
	return best, nil
}
