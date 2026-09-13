package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/notify"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func cmdFocus(args []string) error {
	if len(args) == 0 {
		return cmdFocusSummary([]string{})
	}
	switch args[0] {
	case "start":
		return cmdFocusStart(args[1:])
	case "repeat":
		return cmdFocusRepeat(args[1:])
	case "stop", "finish":
		return cmdFocusStop(args[1:])
	case "pause":
		return cmdFocusPause(args[1:])
	case "resume":
		return cmdFocusResume(args[1:])
	case "dismiss":
		return cmdFocusDismiss(args[1:])
	case "delete-range", "prune":
		return cmdFocusDeleteRange(args[1:])
	case "notify-test", "notify-setup", "alert-preview":
		return cmdFocusNotify(args)
	case "status":
		return cmdFocusStatus(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return cmdFocusSummary(args)
		}
		return cmdFocusSummary(args)
	}
}

func cmdFocusStart(args []string) error {
	fs := flag.NewFlagSet("focus start", flag.ExitOnError)
	duration := fs.Int("duration", 25, "focus length in minutes")
	taskRef := fs.String("task", "", "task id or title substring")
	projectRef := fs.String("project", "", "project id or name (required with --task title)")
	title := fs.String("title", "", "create a new task with this title and focus on it")
	noSync := fs.Bool("no-sync", false, "do not log to TickTick on stop")
	if err := fs.Parse(args); err != nil {
		return err
	}

	c, err := client()
	if err != nil {
		return err
	}

	var taskID, taskTitle, projectID, projectName string
	switch {
	case *title != "":
		pid := *projectRef
		if pid == "" {
			pid = "Inbox"
		}
		p, err := c.ResolveProject(pid)
		if err != nil {
			return err
		}
		projectID = p
		ps, _ := c.ListProjects()
		for _, pr := range ps {
			if pr.ID == p {
				projectName = pr.Name
				break
			}
		}
		id, err := c.AddTask(*title, p, 0, "")
		if err != nil {
			return err
		}
		taskID, taskTitle = id, *title
	case *taskRef != "":
		if ticktick.LooksLikeID(*taskRef) {
			raw, err := c.FindTaskByID(*taskRef)
			if err != nil {
				return err
			}
			taskID = fmt.Sprint(raw["id"])
			taskTitle = fmt.Sprint(raw["title"])
			projectID = fmt.Sprint(raw["projectId"])
		} else {
			if *projectRef == "" {
				return fmt.Errorf("with a task title, pass --project")
			}
			pid, err := c.ResolveProject(*projectRef)
			if err != nil {
				return err
			}
			projectID = pid
			tasks, err := c.ProjectTasks(pid)
			if err != nil {
				return err
			}
			found := false
			q := strings.ToLower(*taskRef)
			for _, t := range tasks {
				if strings.Contains(strings.ToLower(t.Title), q) {
					taskID, taskTitle = t.ID, t.Title
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no task matching %q in project", *taskRef)
			}
		}
		if projectName == "" {
			ps, _ := c.ListProjects()
			for _, pr := range ps {
				if pr.ID == projectID {
					projectName = pr.Name
					break
				}
			}
		}
	}

	d := time.Duration(*duration) * time.Minute
	s, err := focus.Start(d, taskID, taskTitle, projectID, projectName)
	if err != nil {
		return err
	}
	_ = os.Setenv("TTCLI_FOCUS_NO_SYNC", boolEnv(*noSync))
	fmt.Printf("focus started — %d min", *duration)
	if taskTitle != "" {
		fmt.Printf(" on %q", taskTitle)
	}
	fmt.Printf(" (ends ~%s)\n", time.Now().Add(d).Format("15:04"))
	fmt.Println("  ttcli focus status · pause · stop")
	_ = s
	return nil
}

func cmdFocusStop(args []string) error {
	fs := flag.NewFlagSet("focus stop", flag.ExitOnError)
	noSync := fs.Bool("no-sync", false, "do not log to TickTick")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if os.Getenv("TTCLI_FOCUS_NO_SYNC") == "1" {
		*noSync = true
	}

	s, err := focus.Load()
	if err != nil {
		return err
	}
	if s.State == focus.StateAwaitingDismiss {
		if *noSync {
			return s.FinishAfterDismiss()
		}
		c, err := client()
		if err != nil {
			return err
		}
		if err := focus.FinalizeDismiss(c); err != nil {
			return err
		}
		fmt.Println("focus dismissed — planned time logged, unclaimed time recorded if any")
		return nil
	}

	segStart := s.SegmentLogStart()
	segElapsed := s.CurrentSegmentElapsed()
	taskID := s.TaskID
	taskTitle := s.TaskTitle
	projectName := s.ProjectName
	elapsed, err := s.Finish()
	if err != nil {
		return err
	}

	fmt.Printf("focus stopped — %s elapsed\n", formatDuration(elapsed))
	if *noSync || segElapsed < focus.MinLogDuration || taskID == "" {
		return nil
	}

	c, err := client()
	if err != nil {
		return fmt.Errorf("logged locally only (TickTick sync failed: %w)", err)
	}
	id, err := c.LogPomodoro(ticktick.LogPomodoroInput{
		TaskID:      taskID,
		TaskTitle:   taskTitle,
		ProjectName: projectName,
		StartedAt:   segStart,
		Elapsed:     segElapsed,
	})
	if err != nil {
		return fmt.Errorf("session ended locally but TickTick sync failed: %w", err)
	}
	fmt.Printf("✓ logged pomodoro %s to TickTick\n", id)
	return nil
}

func cmdFocusRepeat(_ []string) error {
	c, err := client()
	if err != nil {
		return err
	}
	s, err := focus.Repeat(c)
	if err != nil {
		return err
	}
	fmt.Printf("focus repeated — %d min", int(s.Duration.Minutes()))
	if s.TaskTitle != "" {
		fmt.Printf(" on %q", s.TaskTitle)
	}
	fmt.Println()
	return nil
}

func cmdFocusPause(_ []string) error {
	s, err := focus.Load()
	if err != nil {
		return err
	}
	if err := s.Pause(); err != nil {
		return err
	}
	fmt.Println("focus paused")
	return nil
}

func cmdFocusResume(_ []string) error {
	s, err := focus.Load()
	if err != nil {
		return err
	}
	if err := s.Resume(); err != nil {
		return err
	}
	fmt.Println("focus resumed")
	return nil
}

func cmdFocusStatus(args []string) error {
	fs := flag.NewFlagSet("focus status", flag.ExitOnError)
	short := fs.Bool("short", false, "one-line output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	s, err := focus.Load()
	if err != nil {
		return err
	}
	if !s.Active() {
		if *short {
			fmt.Println("idle")
			return nil
		}
		fmt.Println("no active focus session")
		return nil
	}

	remaining := s.Remaining()
	elapsed := s.Elapsed()
	parts := []string{s.State, formatDuration(remaining) + " left", formatDuration(elapsed) + " elapsed"}
	if p := s.PauseTotal(); p > 0 {
		parts = append(parts, formatDuration(p)+" paused")
	}
	line := strings.Join(parts, " · ")
	if s.TaskTitle != "" {
		line += " · " + s.TaskTitle
	}
	if *short {
		fmt.Println(line)
		return nil
	}
	fmt.Println(line)
	if s.Finished() {
		fmt.Println("timer complete — run: ttcli focus stop")
	}
	return nil
}

func cmdFocusSummary(args []string) error {
	fs := flag.NewFlagSet("focus", flag.ExitOnError)
	short := fs.Bool("short", false, "print N/GOAL only (for scripts/tmux)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	day, err := parseFocusDay(fs.Args())
	if err != nil {
		return err
	}
	c, err := client()
	if err != nil {
		return err
	}
	s, err := c.FocusForDay(day)
	if err != nil {
		return err
	}
	writePomoCache(s.FullPomoCount)
	runPomoPush(s.FullPomoCount)
	if *short {
		fmt.Println(formatPomoStatus(s.FullPomoCount))
		return nil
	}
	mins := s.TotalSeconds / 60
	fmt.Printf("%s — %d full pomodoro(s), %d logged, %dh%02dm focused\n", s.Date, s.FullPomoCount, s.PomoCount, mins/60, mins%60)
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, r := range s.Records {
		title := "(untitled)"
		if len(r.Tasks) > 0 && r.Tasks[0].Title != "" {
			title = r.Tasks[0].Title
		}
		start := r.StartTime
		if len(start) >= 16 {
			start = start[11:16]
		}
		fmt.Fprintf(w, "  %s\t%s\n", start, title)
	}
	return w.Flush()
}

func cmdFocusDismiss(args []string) error {
	fs := flag.NewFlagSet("focus dismiss", flag.ExitOnError)
	noSync := fs.Bool("no-sync", false, "do not log unclaimed time to TickTick")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := notify.Dismiss(); err != nil && !errors.Is(err, notify.ErrNoActive) {
		return err
	}
	s, err := focus.Load()
	if err != nil {
		return err
	}
	if s.State == focus.StateAwaitingDismiss {
		if *noSync {
			return s.FinishAfterDismiss()
		}
		c, err := client()
		if err != nil {
			return err
		}
		if err := focus.FinalizeDismiss(c); err != nil {
			return err
		}
		fmt.Println("focus dismissed — unclaimed time recorded if any")
		return nil
	}
	fmt.Println("focus notification dismissed")
	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m >= 60 {
		return fmt.Sprintf("%dh%02dm", m/60, m%60)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}

func boolEnv(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
