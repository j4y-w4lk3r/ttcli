package pushover

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/secrets"
)

func TestSendPostsUserTokenAndMessage(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = string(body)
		_, _ = w.Write([]byte(`{"status":1}`))
	}))
	defer server.Close()
	previous := Endpoint
	Endpoint = server.URL
	t.Cleanup(func() { Endpoint = previous })

	err := Send(context.Background(), Config{User: "user-key", Token: "app-token"}, "Done", "Done · wake up")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"user=user-key", "token=app-token", "title=Done", "message=Done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("body %q missing %q", got, want)
		}
	}
}

func TestLoadSkipsWhenUnset(t *testing.T) {
	t.Setenv("TTCLI_PUSHOVER_USER", "")
	t.Setenv("TTCLI_PUSHOVER_TOKEN", "")
	t.Setenv("TTCLI_PUSHOVER_OP_ITEM", "")
	_, ok, err := Load(nil)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestConfigFromItemsReadsLicenseKeyAndAPIToken(t *testing.T) {
	user := strings.Repeat("a", 30)
	token := strings.Repeat("b", 30)
	cfg, err := configFromItems([]secrets.Item{
		{
			Title: "Pushover", Vault: "Employee", Category: "LOGIN",
			Fields: []secrets.Field{
				{Label: "user[email]", Purpose: "USERNAME", Value: "me@example.com"},
				{Label: "user[password_confirmation]", Purpose: "PASSWORD", Value: "website-password"},
			},
		},
		{
			Title: "Pushover", Vault: "Software License", Category: "SOFTWARE_LICENSE",
			Fields: []secrets.Field{
				{Label: "license key", Value: user},
				{Label: "API token", Value: token},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.User != user || cfg.Token != token {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestConfigFromItemsReportsMissingToken(t *testing.T) {
	_, err := configFromItems([]secrets.Item{{
		Title: "Pushover", Vault: "Software License",
		Fields: []secrets.Field{{Label: "license key", Value: strings.Repeat("a", 30)}},
	}})
	if err == nil || !strings.Contains(err.Error(), "API token") {
		t.Fatal(err)
	}
}

func TestConfigFromItemsIgnoresWebsiteLogin(t *testing.T) {
	_, err := configFromItems([]secrets.Item{{
		Title: "Pushover", Vault: "Employee", Category: "LOGIN",
		Fields: []secrets.Field{
			{Purpose: "USERNAME", Value: "me@example.com"},
			{Purpose: "PASSWORD", Value: "website-password"},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "website login") {
		t.Fatal(err)
	}
}
