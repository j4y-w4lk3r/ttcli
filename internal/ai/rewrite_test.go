package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRewriteParsesTitleAndNotes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer test-key") {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "shorten") {
			http.Error(w, "missing instruction", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"title\":\"Oat milk\",\"notes\":\"Oatly\\n1L\"}"}}]}`))
	}))
	defer server.Close()

	title, notes, err := Rewrite(context.Background(), Config{
		BaseURL: server.URL, Model: "test", APIKey: "test-key",
	}, "buy oat milk please", "", "shorten the title and add notes")
	if err != nil {
		t.Fatal(err)
	}
	if title != "Oat milk" || notes != "Oatly\n1L" {
		t.Fatalf("title=%q notes=%q", title, notes)
	}
}

func TestLoadRequiresKeyAndModel(t *testing.T) {
	t.Setenv("TTCLI_AI_API_KEY", "")
	t.Setenv("TTCLI_AI_MODEL", "")
	t.Setenv("TTCLI_AI_OP_ITEM", "")
	if _, err := Load(nil); err == nil {
		t.Fatal("expected missing config error")
	}
}
