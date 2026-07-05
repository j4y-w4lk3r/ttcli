package secrets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// OnePassword shells out to the 1Password CLI (`op`).
type OnePassword struct{}

var _ Backend = OnePassword{}

func (OnePassword) Available() error {
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("1Password CLI (op) not found on PATH: %w\n  install: brew install --cask 1password-cli", err)
	}
	return nil
}

func (OnePassword) CheckSignedIn() error {
	if _, err := runOp("whoami"); err != nil {
		return fmt.Errorf("1Password not signed in (run `op signin` or enable Touch ID): %w", err)
	}
	return nil
}

func (OnePassword) GetLogin(vault, itemRef string) (string, string, error) {
	if itemRef == "" {
		itemRef = "TickTick"
	}

	resolvedVault, itemID, err := resolveLoginLocation(vault, itemRef)
	if err != nil {
		return "", "", err
	}

	args := []string{"item", "get", itemID, "--format", "json"}
	if resolvedVault != "" {
		args = append(args, "--vault", resolvedVault)
	}
	out, err := runOp(args...)
	if err != nil {
		return "", "", fmt.Errorf("read 1Password item %q: %w", itemRef, err)
	}
	return parseLoginFields(out)
}

func resolveLoginLocation(vault, itemRef string) (resolvedVault, itemID string, err error) {
	if vault != "" {
		// Caller named a vault — op resolves title or id within it.
		return vault, itemRef, nil
	}
	loc, err := findLoginItem("", itemRef)
	if err != nil {
		return "", "", err
	}
	return loc.Vault, loc.ID, nil
}

type loginListItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Vault struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"vault"`
	URLs []struct {
		Href    string `json:"href"`
		Primary bool   `json:"primary"`
	} `json:"urls"`
}

type loginLocation struct {
	ID    string
	Title string
	Vault string // vault name (op accepts name or id)
	Score int
}

// findLoginItem locates a Login item by title across vaults (or within one
// vault when vaultFilter is set). Prefers exact title matches and items whose
// URL is ticktick.com.
func findLoginItem(vaultFilter, title string) (loginLocation, error) {
	args := []string{"item", "list", "--categories", "Login", "--format", "json"}
	if vaultFilter != "" {
		args = append(args, "--vault", vaultFilter)
	}
	out, err := runOp(args...)
	if err != nil {
		return loginLocation{}, fmt.Errorf("op item list: %w", err)
	}

	var items []loginListItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return loginLocation{}, fmt.Errorf("op item list: parse JSON: %w", err)
	}

	want := strings.ToLower(strings.TrimSpace(title))
	var matches []loginLocation

	for _, it := range items {
		score := scoreLoginItem(it, want)
		if score < 0 {
			continue
		}
		vault := it.Vault.Name
		if vault == "" {
			vault = it.Vault.ID
		}
		matches = append(matches, loginLocation{
			ID: it.ID, Title: it.Title, Vault: vault, Score: score,
		})
	}

	if len(matches) == 0 {
		if vaultFilter != "" {
			return loginLocation{}, fmt.Errorf("no Login item matching %q in vault %q", title, vaultFilter)
		}
		return loginLocation{}, fmt.Errorf("no Login item matching %q in any vault (create one or pass --vault/--item)", title)
	}

	best := matches[0]
	for _, m := range matches[1:] {
		if m.Score > best.Score {
			best = m
		}
	}
	return best, nil
}

// scoreLoginItem returns -1 if the item is not a candidate, higher = better.
func scoreLoginItem(it loginListItem, wantTitle string) int {
	title := strings.ToLower(strings.TrimSpace(it.Title))
	score := -1

	if title == wantTitle {
		score = 100
	} else if strings.Contains(title, wantTitle) || strings.Contains(wantTitle, title) {
		score = 50
	}

	urlScore := ticktickURLScore(it.URLs)
	if urlScore > 0 && score < 0 {
		// No title match but URL points at TickTick — accept as fallback.
		score = 10 + urlScore
	} else if urlScore > 0 {
		score += urlScore
	}

	return score
}

func ticktickURLScore(urls []struct {
	Href    string `json:"href"`
	Primary bool   `json:"primary"`
}) int {
	score := 0
	for _, u := range urls {
		href := strings.ToLower(u.Href)
		if !strings.Contains(href, "ticktick.com") {
			continue
		}
		score += 20
		if u.Primary {
			score += 10
		}
	}
	return score
}

func runOp(args ...string) (string, error) {
	cmd := exec.Command("op", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

// parseLoginFields extracts username + password from `op item get --format json`.
func parseLoginFields(out string) (username, password string, err error) {
	var item struct {
		Fields []struct {
			Purpose string `json:"purpose"`
			Label   string `json:"label"`
			ID      string `json:"id"`
			Value   string `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(out), &item); err != nil {
		return "", "", fmt.Errorf("parse op item JSON: %w", err)
	}
	for _, f := range item.Fields {
		switch f.Purpose {
		case "USERNAME":
			username = f.Value
		case "PASSWORD":
			password = f.Value
		}
	}
	if username == "" || password == "" {
		for _, f := range item.Fields {
			switch strings.ToLower(f.Label) {
			case "username", "email":
				if username == "" {
					username = f.Value
				}
			case "password":
				if password == "" {
					password = f.Value
				}
			}
		}
	}
	if username == "" || password == "" {
		return "", "", fmt.Errorf("1Password item is missing username or password fields")
	}
	return username, password, nil
}
