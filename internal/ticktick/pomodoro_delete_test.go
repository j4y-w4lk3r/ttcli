package ticktick

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeletePomodoroUsesDeleteEndpoint(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, http: srv.Client()}
	c.headers = map[string]string{"Content-Type": "application/json"}

	if err := c.DeletePomodoro("abc123"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete {
		t.Fatalf("method=%q want DELETE", method)
	}
	if path != "/api/v2/pomodoro/abc123" {
		t.Fatalf("path=%q", path)
	}
}
