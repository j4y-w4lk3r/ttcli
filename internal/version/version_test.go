package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestString_releaseMetadata(t *testing.T) {
	Version = "0.1.0"
	Commit = "abc1234"
	Date = "2026-06-06T12:00:00Z"
	t.Cleanup(func() {
		Version = "dev"
		Commit = "none"
		Date = "unknown"
	})

	got := String()
	want := "ttcli 0.1.0 (commit abc1234, built 2026-06-06T12:00:00Z)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestString_devUsesBuildInfo(t *testing.T) {
	Version = "dev"
	Commit = "none"
	Date = "unknown"

	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info (go test without module context)")
	}

	got := String()
	if strings.Contains(got, "commit none") && vcsSetting(info, "vcs.revision") != "" {
		t.Fatalf("expected VCS commit fallback, got %q", got)
	}
	if !strings.HasPrefix(got, "ttcli ") {
		t.Fatalf("unexpected format: %q", got)
	}
}

func TestModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}
	if got := moduleVersion(info); got != "0.1.0" {
		t.Fatalf("got %q want 0.1.0", got)
	}
}

func TestShortRev(t *testing.T) {
	if got := shortRev("d15b7ece463c95043f0cf433beeec962b6d780b8"); got != "d15b7ece463c" {
		t.Fatalf("got %q", got)
	}
}
