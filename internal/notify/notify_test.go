package notify

import (
	"strings"
	"testing"
	"time"
)

func TestBuildContentNoDismissHint(t *testing.T) {
	c := BuildContent(VariantStandard, "Cancel Proton Pass manager")
	if strings.Contains(c.Body, "Dismiss") {
		t.Fatalf("body should not mention dismiss: %q", c.Body)
	}
	if strings.Contains(c.Body, "ttcli") {
		t.Fatalf("body should not mention ttcli: %q", c.Body)
	}
	if c.Body != "Done · Cancel Proton Pass manager" {
		t.Fatalf("body=%q", c.Body)
	}
	if !strings.Contains(c.Title, "✓") {
		t.Fatalf("title should use check icon: %q", c.Title)
	}
}

func TestBuildNotifySendArgsThemedHints(t *testing.T) {
	args := BuildNotifySendArgs(VariantThemed, "Task A")
	joined := strings.Join(args, " ")
	for _, want := range []string{"frcolor:#fab387", "bgcolor:#1e1e2e", "fgcolor:#cdd6f4", "Task complete"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, args)
		}
	}
	if strings.Contains(joined, "Dismiss") {
		t.Fatal("should not include dismiss text")
	}
}

func TestEscalatedUrgencyNormal(t *testing.T) {
	c := BuildContent(VariantEscalated, "X")
	if c.Urgency != "normal" {
		t.Fatalf("urgency=%q", c.Urgency)
	}
	if !strings.Contains(c.Body, "Still waiting") {
		t.Fatalf("body=%q", c.Body)
	}
}

func TestParseVariant(t *testing.T) {
	v, ok := ParseVariant("orange")
	if !ok || v != VariantOrange {
		t.Fatalf("parse orange: ok=%v v=%q", ok, v)
	}
	_, ok = ParseVariant("nope")
	if ok {
		t.Fatal("expected unknown variant")
	}
}

func TestBuildNTEscalateArgsSkipInitial(t *testing.T) {
	args := buildNTEscalateArgs("✓ Done", "body", 20*time.Second, true)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--skip-initial") {
		t.Fatalf("expected --skip-initial in %v", args)
	}
	if strings.Contains(joined, "critical") {
		t.Fatal("should not embed critical urgency")
	}
}

func TestBuildNotifySendArgsSoftIcon(t *testing.T) {
	args := BuildNotifySendArgs(VariantOrange, "Task")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-u normal") {
		t.Fatalf("expected normal urgency in %v", args)
	}
	if !strings.Contains(joined, "focus-done-128.png") && !strings.Contains(joined, "task-due") {
		t.Fatalf("expected bundled or fallback icon in %v", args)
	}
	if strings.Contains(joined, "emblem-ok") {
		t.Fatal("should use bundled completion icon, not emblem-ok")
	}
}

func TestMakoConfigSnippet(t *testing.T) {
	s := MakoConfigSnippet()
	if !strings.Contains(s, "app-name=ttcli") || !strings.Contains(s, "#fab387") {
		t.Fatal("snippet missing ttcli theme")
	}
}
