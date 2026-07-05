package secrets

import (
	"fmt"
	"strings"
	"sync"
)

// Memory is an in-memory Backend for tests.
type Memory struct {
	mu    sync.Mutex
	items map[string]loginItem // key: vault/item title
}

type loginItem struct {
	Vault    string
	Title    string
	Username string
	Password string
	URL      string
}

func NewMemory() *Memory {
	return &Memory{items: map[string]loginItem{}}
}

var _ Backend = (*Memory)(nil)

func (m *Memory) Available() error     { return nil }
func (m *Memory) CheckSignedIn() error { return nil }

// SetLogin stores credentials under vault+title. Test helper.
func (m *Memory) SetLogin(vault, title, username, password string) {
	m.SetLoginURL(vault, title, username, password, "")
}

// SetLoginURL stores credentials with an optional website URL for discovery tests.
func (m *Memory) SetLoginURL(vault, title, username, password, website string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vault == "" {
		vault = "Private"
	}
	m.items[memKey(vault, title)] = loginItem{
		Vault: vault, Title: title, Username: username, Password: password, URL: website,
	}
}

func (m *Memory) GetLogin(vault, itemRef string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if itemRef == "" {
		itemRef = "TickTick"
	}

	if vault != "" {
		if it, ok := m.items[memKey(vault, itemRef)]; ok {
			return it.Username, it.Password, nil
		}
		for _, it := range m.items {
			if !vaultMatches(vault, it.Vault) {
				continue
			}
			if strings.EqualFold(it.Title, itemRef) {
				return it.Username, it.Password, nil
			}
		}
		return "", "", fmt.Errorf("memory: login item %q not found in vault %q", itemRef, vault)
	}

	loc, err := m.findLoginLocked("", itemRef)
	if err != nil {
		return "", "", err
	}
	it := m.items[memKey(loc.Vault, loc.Title)]
	return it.Username, it.Password, nil
}

func (m *Memory) findLoginLocked(vaultFilter, title string) (loginLocation, error) {
	want := strings.ToLower(strings.TrimSpace(title))
	var matches []loginLocation

	for key, it := range m.items {
		if vaultFilter != "" && !vaultMatches(vaultFilter, it.Vault) {
			continue
		}
		score := scoreLoginItem(loginListItem{
			ID:    key,
			Title: it.Title,
			Vault: struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}{Name: it.Vault},
			URLs: urlsFromHref(it.URL),
		}, want)
		if score < 0 {
			continue
		}
		matches = append(matches, loginLocation{
			ID: key, Title: it.Title, Vault: it.Vault, Score: score,
		})
	}

	if len(matches) == 0 {
		return loginLocation{}, fmt.Errorf("memory: no login item matching %q", title)
	}
	best := matches[0]
	for _, c := range matches[1:] {
		if c.Score > best.Score {
			best = c
		}
	}
	return best, nil
}

func vaultMatches(filter, vault string) bool {
	return strings.EqualFold(filter, vault)
}

func urlsFromHref(href string) []struct {
	Href    string `json:"href"`
	Primary bool   `json:"primary"`
} {
	if href == "" {
		return nil
	}
	return []struct {
		Href    string `json:"href"`
		Primary bool   `json:"primary"`
	}{{Href: href, Primary: true}}
}

func memKey(vault, title string) string { return vault + "/" + title }
