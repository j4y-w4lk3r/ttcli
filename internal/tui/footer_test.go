package tui

import (
	"strings"
	"testing"
	"time"
)

func TestFooterErrorClearsAfterTenSeconds(t *testing.T) {
	m := fixtureModel(80, 24)
	m.errMsg = "task abcdef0123456789abcdef01 not found in project 0123456789abcdef01234567"
	start := time.Date(2026, 10, 3, 11, 24, 0, 0, time.UTC)

	next, _ := m.Update(tickMsg(start))
	m = next.(model)
	if m.errMsg == "" || !strings.Contains(stripANSI(m.renderFooter()), "not found in project") {
		t.Fatalf("error should stay visible, footer=%q", stripANSI(m.renderFooter()))
	}

	next, _ = m.Update(tickMsg(start.Add(9 * time.Second)))
	m = next.(model)
	if m.errMsg == "" {
		t.Fatal("cleared before 10 seconds")
	}

	next, _ = m.Update(tickMsg(start.Add(errorVisibleFor)))
	m = next.(model)
	if m.errMsg != "" || strings.Contains(stripANSI(m.renderFooter()), "not found in project") {
		t.Fatalf("error still showing: %q footer=%q", m.errMsg, stripANSI(m.renderFooter()))
	}
}

func TestFooterErrorTimerRestartsForANewMessage(t *testing.T) {
	m := fixtureModel(80, 24)
	m.errMsg = "first"
	start := time.Date(2026, 10, 3, 11, 24, 0, 0, time.UTC)
	next, _ := m.Update(tickMsg(start))
	m = next.(model)

	m.errMsg = "second"
	next, _ = m.Update(tickMsg(start.Add(9 * time.Second)))
	m = next.(model)
	if m.errMsg != "second" {
		t.Fatalf("new error was cleared early: %q", m.errMsg)
	}
	next, _ = m.Update(tickMsg(start.Add(9*time.Second + errorVisibleFor)))
	m = next.(model)
	if m.errMsg != "" {
		t.Fatalf("new error stuck: %q", m.errMsg)
	}
}
