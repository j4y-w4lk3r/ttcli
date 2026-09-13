package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFocusDoneIconPathMaterializes(t *testing.T) {
	path, err := FocusDoneIconPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, focusDoneIconName) {
		t.Fatalf("path=%q", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != len(focusDoneIconPNG) {
		t.Fatalf("icon size=%d want %d", len(b), len(focusDoneIconPNG))
	}
	if filepath.Base(path) != focusDoneIconName {
		t.Fatalf("basename=%q", filepath.Base(path))
	}
}

func TestBuildNotifySendArgsUsesBundledIcon(t *testing.T) {
	args := BuildNotifySendArgs(VariantOrange, "Write docs")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, focusDoneIconName) {
		t.Fatalf("expected bundled icon path in %v", args)
	}
}
