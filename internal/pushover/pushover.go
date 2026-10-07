// Package pushover sends optional alerts through the Pushover API.
package pushover

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/secrets"
)

// Endpoint is the Pushover message API. Tests replace it.
var Endpoint = "https://api.pushover.net/1/messages.json"

// Config is a Pushover user key and application token.
type Config struct {
	User  string
	Token string
}

// pushoverKey is the 30-character user key or application token.
var pushoverKey = regexp.MustCompile(`^[A-Za-z0-9]{30}$`)

// Load reads credentials from the environment. When those are empty and
// TTCLI_PUSHOVER_OP_ITEM is set, getItems reads that 1Password item.
// A software-license "license key" is the user key. A field named
// "API token" is the application token. A login whose username and password
// are both 30-character keys works too. A website email login is ignored.
// Unconfigured returns ok=false and a nil error.
func Load(getItems func(vault, item string) ([]secrets.Item, error)) (Config, bool, error) {
	cfg := Config{
		User:  strings.TrimSpace(os.Getenv("TTCLI_PUSHOVER_USER")),
		Token: strings.TrimSpace(os.Getenv("TTCLI_PUSHOVER_TOKEN")),
	}
	if cfg.User != "" || cfg.Token != "" {
		if cfg.User == "" || cfg.Token == "" {
			return Config{}, false, fmt.Errorf("set both TTCLI_PUSHOVER_USER and TTCLI_PUSHOVER_TOKEN")
		}
		return cfg, true, nil
	}
	item := strings.TrimSpace(os.Getenv("TTCLI_PUSHOVER_OP_ITEM"))
	if item == "" {
		return Config{}, false, nil
	}
	if getItems == nil {
		return Config{}, false, fmt.Errorf("1Password is unavailable for item %q", item)
	}
	items, err := getItems(os.Getenv("TTCLI_OP_VAULT"), item)
	if err != nil {
		return Config{}, false, err
	}
	cfg, err = configFromItems(items)
	if err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

func configFromItems(items []secrets.Item) (Config, error) {
	var userOnlyTitle string
	sawWebsite := false
	for _, it := range items {
		user, token, kind := credsFromItem(it)
		switch kind {
		case credComplete:
			return Config{User: user, Token: token}, nil
		case credUserOnly:
			if userOnlyTitle == "" {
				userOnlyTitle = it.Title
				if it.Vault != "" {
					userOnlyTitle = it.Title + " (" + it.Vault + ")"
				}
			}
		case credWebsite:
			sawWebsite = true
		}
	}
	if userOnlyTitle != "" {
		return Config{}, fmt.Errorf("1Password item %q has the Pushover user key and no application token; add a field named \"API token\"", userOnlyTitle)
	}
	if sawWebsite {
		return Config{}, fmt.Errorf("1Password Pushover item is the website login; save the user key and an application token instead")
	}
	return Config{}, fmt.Errorf("1Password item has no Pushover user key")
}

const (
	credNone = iota
	credComplete
	credUserOnly
	credWebsite
)

func credsFromItem(it secrets.Item) (user, token string, kind int) {
	var license, labeledToken, username, password string
	for _, f := range it.Fields {
		label := strings.ToLower(strings.TrimSpace(f.Label))
		value := strings.TrimSpace(f.Value)
		switch label {
		case "license key", "user key":
			if pushoverKey.MatchString(value) {
				license = value
			}
		case "api token", "application token", "app token", "token":
			if pushoverKey.MatchString(value) {
				labeledToken = value
			}
		}
		switch f.Purpose {
		case "USERNAME":
			username = value
		case "PASSWORD":
			password = value
		}
	}
	if license != "" && labeledToken != "" {
		return license, labeledToken, credComplete
	}
	if pushoverKey.MatchString(username) && pushoverKey.MatchString(password) {
		return username, password, credComplete
	}
	if license != "" {
		return license, "", credUserOnly
	}
	if strings.Contains(username, "@") {
		return "", "", credWebsite
	}
	return "", "", credNone
}

// Send delivers one message. Pushover priority stays at the normal level.
func Send(ctx context.Context, cfg Config, title, message string) error {
	if cfg.User == "" || cfg.Token == "" {
		return fmt.Errorf("pushover credentials missing")
	}
	if strings.TrimSpace(title) == "" {
		title = "ttcli"
	}
	form := url.Values{}
	form.Set("token", cfg.Token)
	form.Set("user", cfg.User)
	form.Set("title", title)
	form.Set("message", message)
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("pushover HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
