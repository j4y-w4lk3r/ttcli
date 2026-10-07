package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/notify"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const reminderScanEvery = 15 * time.Second

type reminderScanMsg struct {
	tasks []ticktick.Task
	at    time.Time
	err   error
}

func (m model) armReminderScan(now time.Time) (model, tea.Cmd) {
	if m.repo == nil {
		return m, nil
	}
	if !m.reminderCheckedAt.IsZero() && now.Sub(m.reminderCheckedAt) < reminderScanEvery {
		return m, nil
	}
	m.reminderCheckedAt = now
	repo := m.repo
	return m, func() tea.Msg {
		tasks, _, err := repo.OpenTasks(false)
		return reminderScanMsg{tasks: tasks, at: time.Now(), err: err}
	}
}

func (m model) deliverReminders(msg reminderScanMsg) (model, tea.Cmd) {
	if msg.err != nil || len(msg.tasks) == 0 {
		return m, nil
	}
	if m.reminderSent == nil {
		m.reminderSent = loadReminderSent()
	}
	hits := ticktick.DueReminders(msg.tasks, msg.at, m.reminderSent, ticktick.ReminderGrace)
	if len(hits) == 0 {
		return m, nil
	}
	for _, hit := range hits {
		m.reminderSent[hit.Key] = struct{}{}
	}
	saveReminderSent(m.reminderSent)
	if len(hits) == 1 {
		m.toast = "reminder · " + hits[0].Task.Title
	} else {
		m.toast = fmt.Sprintf("reminders · %d tasks", len(hits))
	}
	return m, sendRemindersCmd(hits, msg.at)
}

func sendRemindersCmd(hits []ticktick.ReminderHit, now time.Time) tea.Cmd {
	return func() tea.Msg {
		for _, hit := range hits {
			notify.TaskDue(hit.Task.Title, reminderWhen(hit.Due, now))
		}
		return nil
	}
}

func reminderWhen(due, now time.Time) string {
	if due.IsZero() {
		return ""
	}
	due = due.In(time.Local)
	now = now.In(time.Local)
	if due.Year() == now.Year() && due.YearDay() == now.YearDay() {
		return due.Format("15:04")
	}
	return due.Format("02/01 15:04")
}

type reminderSentFile struct {
	Keys []string `json:"keys"`
}

func reminderSentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "reminder-sent.json"), nil
}

func loadReminderSent() map[string]struct{} {
	sent := map[string]struct{}{}
	path, err := reminderSentPath()
	if err != nil {
		return sent
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return sent
	}
	var file reminderSentFile
	if json.Unmarshal(b, &file) != nil {
		return sent
	}
	cutoff := time.Now().Add(-48 * time.Hour)
	for _, key := range file.Keys {
		at, ok := reminderKeyTime(key)
		if ok && at.Before(cutoff) {
			continue
		}
		sent[key] = struct{}{}
	}
	return sent
}

func saveReminderSent(sent map[string]struct{}) {
	path, err := reminderSentPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	cutoff := time.Now().Add(-48 * time.Hour)
	file := reminderSentFile{}
	for key := range sent {
		at, ok := reminderKeyTime(key)
		if ok && at.Before(cutoff) {
			continue
		}
		file.Keys = append(file.Keys, key)
	}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o600)
}

func reminderKeyTime(key string) (time.Time, bool) {
	_, rest, ok := strings.Cut(key, "@")
	if !ok {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, rest)
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}
