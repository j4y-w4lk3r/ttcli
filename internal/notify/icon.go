package notify

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed assets/focus-done-128.png
var focusDoneIconPNG []byte

const focusDoneIconName = "focus-done-128.png"

// FocusDoneIconPath returns a 128px completion icon (green check, peach ring).
// Materialized under ~/.cache/ttcli/ so notify-send can pass an absolute path.
func FocusDoneIconPath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "ttcli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, focusDoneIconName)
	if b, err := os.ReadFile(path); err == nil && len(b) == len(focusDoneIconPNG) {
		return path, nil
	}
	if err := os.WriteFile(path, focusDoneIconPNG, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// focusDoneIconArg resolves the notify-send -i value (bundled PNG or theme fallback).
func focusDoneIconArg() string {
	if path, err := FocusDoneIconPath(); err == nil {
		return path
	}
	return "task-due"
}
