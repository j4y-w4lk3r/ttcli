package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Config for the local webhook HTTP server.
type Config struct {
	Addr       string // e.g. 127.0.0.1:8787
	Secret     string // optional Bearer token
	PushScript string // tmux push hook
}

type pomoBody struct {
	Count *int `json:"count"`
}

// Serve starts the webhook HTTP server until error.
func Serve(cfg Config) error {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8787"
	}
	if cfg.PushScript == "" {
		cfg.PushScript = defaultPushScript()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/hooks/pomo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.Secret != "" && !authOK(r, cfg.Secret) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var count *int
		if r.Body != nil {
			defer r.Body.Close()
			b, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
			if len(strings.TrimSpace(string(b))) > 0 {
				var body pomoBody
				if err := json.Unmarshal(b, &body); err != nil {
					http.Error(w, "bad json", http.StatusBadRequest)
					return
				}
				count = body.Count
			}
		}

		line, err := pushPomo(cfg.PushScript, count)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"pomo": line})
	})

	log.Printf("ttcli webhook listening on http://%s", cfg.Addr)
	log.Printf("  POST /hooks/pomo  (optional JSON: {\"count\": 5})")
	log.Printf("  GET  /health")
	return http.ListenAndServe(cfg.Addr, mux)
}

func authOK(r *http.Request, secret string) bool {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") && strings.TrimPrefix(h, "Bearer ") == secret {
		return true
	}
	return r.Header.Get("X-TTCLI-Secret") == secret
}

func defaultPushScript() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h + "/.config/zsh/ttcli-pomo-push.sh"
	}
	return ""
}

func pushPomo(script string, count *int) (string, error) {
	if script == "" {
		return "", fmt.Errorf("push script not configured")
	}
	args := []string{script}
	if count != nil {
		args = append(args, strconv.Itoa(*count))
	} else {
		args = append(args, "--fetch")
	}
	out, err := exec.Command("/bin/bash", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w (%s)", script, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
