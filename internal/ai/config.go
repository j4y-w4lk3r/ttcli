package ai

import (
	"fmt"
	"os"
	"strings"
)

// Config is an OpenAI-compatible chat endpoint.
type Config struct {
	BaseURL string
	Model   string
	APIKey  string
}

// Load reads the model endpoint from the environment, then from 1Password
// when TTCLI_AI_OP_ITEM is set. getLogin may be nil.
func Load(getLogin func(vault, item string) (username, password string, err error)) (Config, error) {
	cfg := Config{
		BaseURL: strings.TrimRight(os.Getenv("TTCLI_AI_BASE_URL"), "/"),
		Model:   strings.TrimSpace(os.Getenv("TTCLI_AI_MODEL")),
		APIKey:  strings.TrimSpace(os.Getenv("TTCLI_AI_API_KEY")),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	item := strings.TrimSpace(os.Getenv("TTCLI_AI_OP_ITEM"))
	if cfg.APIKey == "" && item != "" {
		if getLogin == nil {
			return Config{}, fmt.Errorf("1Password is unavailable for item %q", item)
		}
		user, secret, err := getLogin(os.Getenv("TTCLI_OP_VAULT"), item)
		if err != nil {
			return Config{}, err
		}
		cfg.APIKey = strings.TrimSpace(secret)
		if cfg.Model == "" && !strings.Contains(user, "@") {
			cfg.Model = strings.TrimSpace(user)
		}
	}
	if cfg.APIKey == "" || cfg.Model == "" {
		return Config{}, fmt.Errorf("set TTCLI_AI_API_KEY and TTCLI_AI_MODEL (or TTCLI_AI_OP_ITEM)")
	}
	return cfg, nil
}
