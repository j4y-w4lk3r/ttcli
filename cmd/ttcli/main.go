// ttcli — a terminal client for TickTick (tasks, lists, pomodoros).
//
// Authenticated via a captured browser session (see internal/ticktick).
// This is a personal automation tool built on TickTick's private web API;
// it is not affiliated with TickTick and may break when their app changes.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/secrets"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
	"github.com/j4y-w4lk3r/ttcli/internal/version"
	"github.com/j4y-w4lk3r/ttcli/internal/webhook"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ttcli <command>  (try: ttcli help  or  ttcli shell)")
		os.Exit(2)
	}
	if err := runCommand(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ttcli: %v\n", err)
		os.Exit(1)
	}
}

func client() (*ticktick.Client, error) {
	return ticktick.New("", ticktick.CredentialOptions{})
}

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	vault := fs.String("vault", "", "1Password vault (default Private, or $TTCLI_OP_VAULT)")
	item := fs.String("item", "", "1Password Login item title or id (default TickTick, or $TTCLI_OP_ITEM)")
	email := fs.String("email", "", "TickTick email (skip 1Password; requires --password)")
	pass := fs.String("password", "", "TickTick password (skip 1Password; requires --email)")
	out := fs.String("out", "", "path to write the session file (default ~/.ticktick_auth.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := ticktick.CredentialOptions{
		Vault: *vault, Item: *item, Email: *email, Password: *pass,
	}
	c, err := ticktick.Login(opts, *out)
	if err != nil {
		return err
	}
	fmt.Printf("✓ logged in — session saved to %s\n", c.AuthPath)
	return nil
}

func cmdLists(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	namesOnly := fs.Bool("names", false, "print list names only (one per line)")
	tree := fs.Bool("tree", false, "print folder tree (TickTick project groups)")
	all := fs.Bool("all", false, "include closed and NOTE lists")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := client()
	if err != nil {
		return err
	}
	ps, err := c.ListProjects()
	if err != nil {
		return err
	}
	if !*all {
		ps = ticktick.OpenProjects(ps, false)
	}
	if *tree {
		gs, err := c.ListProjectGroups()
		if err != nil {
			return err
		}
		fmt.Print(ticktick.FormatProjectTree(ticktick.ProjectTree(gs, ps)))
		return nil
	}
	if *namesOnly {
		for _, p := range ps {
			fmt.Println(p.Name)
		}
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tKIND")
	for _, p := range ps {
		kind := p.Kind
		if kind == "" {
			kind = "TASK"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.ID, p.Name, kind)
	}
	return w.Flush()
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	ping := fs.Bool("ping", false, "also verify API connectivity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := client()
	if err != nil {
		return err
	}
	st := c.Status()
	fmt.Println(version.String())
	fmt.Printf("session  %s", st.AuthPath)
	if st.SessionSaved != "" {
		fmt.Printf("  (saved %s)", st.SessionSaved)
	}
	fmt.Println()

	switch {
	case *ping:
		if err := c.Ping(); err != nil {
			fmt.Printf("api      ✗ %v\n", err)
		} else {
			fmt.Println("api      ✓ connected")
		}
	default:
		fmt.Println("api      (run with --ping to verify)")
	}

	if b := secrets.Default(); b.Available() == nil {
		switch b.CheckSignedIn() {
		case nil:
			fmt.Println("1pass    ✓ signed in (auto-refresh ok)")
		default:
			fmt.Println("1pass    · available (run `op signin` for auto-refresh)")
		}
	} else {
		fmt.Println("1pass    · not installed")
	}
	return nil
}

func cmdTasks(args []string) error {
	fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
	completed := fs.Bool("completed", false, "show completed tasks (account-wide)")
	done := fs.Bool("done", false, "alias for --completed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if *completed || *done {
		c, err := client()
		if err != nil {
			return err
		}
		tasks, err := c.CompletedTasks()
		if err != nil {
			return err
		}
		printTasks(tasks)
		return nil
	}
	if len(rest) < 1 {
		return fmt.Errorf("usage: ttcli tasks <project-id|project-name|all> [--completed]")
	}
	c, err := client()
	if err != nil {
		return err
	}
	if strings.EqualFold(rest[0], "all") {
		tasks, err := c.AllOpenTasks()
		if err != nil {
			return err
		}
		printTasks(tasks)
		return nil
	}
	pid, err := c.ResolveProject(rest[0])
	if err != nil {
		return err
	}
	tasks, err := c.ProjectTasks(pid)
	if err != nil {
		return err
	}
	printTasks(tasks)
	return nil
}

func printTasks(tasks []ticktick.Task) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tPRIO\tDUE\tID\tTITLE")
	for _, t := range tasks {
		status := "open"
		if t.Done() {
			status = "done"
		}
		due := t.DueDate
		if len(due) >= 16 {
			due = due[11:16] + " " + due[:10]
		} else if len(due) >= 10 {
			due = due[:10]
		}
		if due == "" {
			due = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", status, t.PriorityLabel(), due, t.ID, t.Title)
	}
	w.Flush()
}

func cmdDue(args []string) error {
	date, at, project, query := "", "10:00", "", ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch a {
		case "-d", "--date":
			date = next()
		case "-t", "--time":
			at = next()
		case "-p", "--project":
			project = next()
		default:
			if query == "" {
				query = a
			} else {
				query += " " + a
			}
		}
	}
	if date == "" || query == "" {
		return fmt.Errorf("usage: ttcli due <task-id|title> -d YYYY-MM-DD [-t HH:MM] [-p PROJECT]")
	}
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", date, err)
	}
	parts := strings.Split(at, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid time %q (use HH:MM)", at)
	}
	hh, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return fmt.Errorf("invalid time %q (use HH:MM)", at)
	}
	d = d.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute)

	c, err := client()
	if err != nil {
		return err
	}
	var task map[string]any
	if looksLikeTaskID(query) {
		task, err = c.FindTaskByID(query)
	} else {
		task, err = c.FindTask(query)
	}
	if err != nil {
		return err
	}
	if project != "" {
		pid, err := c.ResolveProject(project)
		if err != nil {
			return err
		}
		if got, _ := task["projectId"].(string); got != pid {
			return fmt.Errorf("task %q is not in project %q", query, project)
		}
	}
	if err := c.RescheduleTask(task, d); err != nil {
		return err
	}
	title, _ := task["title"].(string)
	fmt.Printf("✓ rescheduled %q → %s %s\n", title, d.Format("2006-01-02"), d.Format("15:04"))
	return nil
}

func looksLikeTaskID(s string) bool {
	return len(s) >= 20 && !strings.Contains(s, " ")
}

func cmdAdd(args []string) error {
	// Hand-rolled parse so flags may appear before OR after the title
	// (Go's flag pkg stops at the first positional arg, which would
	// otherwise swallow trailing flags into the title).
	project, prio, note := "", "none", ""
	var titleParts []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch a {
		case "-p", "--project":
			project = next()
		case "-P", "--priority":
			prio = next()
		case "-n", "--note":
			note = next()
		default:
			titleParts = append(titleParts, a)
		}
	}
	title := strings.TrimSpace(strings.Join(titleParts, " "))
	if title == "" {
		return fmt.Errorf("usage: ttcli add <title> [-p PROJECT] [-P PRIORITY] [-n NOTE]")
	}
	c, err := client()
	if err != nil {
		return err
	}
	id, err := c.AddTask(title, project, priorityValue(prio), note)
	if err != nil {
		return err
	}
	fmt.Printf("✓ added task %s\n", id)
	return nil
}

func cmdDone(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: ttcli done <project-id|name> <task-id>")
	}
	c, err := client()
	if err != nil {
		return err
	}
	if err := c.CompleteTask(args[0], args[1]); err != nil {
		return err
	}
	fmt.Printf("✓ completed %s\n", args[1])
	return nil
}

func cmdRm(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: ttcli rm <project-id|name> <task-id>")
	}
	c, err := client()
	if err != nil {
		return err
	}
	if err := c.DeleteTask(args[0], args[1]); err != nil {
		return err
	}
	fmt.Printf("✓ deleted %s\n", args[1])
	return nil
}

func cmdFocus(args []string) error {
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
	writePomoCache(s.PomoCount)
	runPomoPush(s.PomoCount)
	if *short {
		fmt.Println(formatPomoStatus(s.PomoCount))
		return nil
	}
	mins := s.TotalSeconds / 60
	fmt.Printf("%s — %d pomodoro(s), %dh%02dm focused\n", s.Date, s.PomoCount, mins/60, mins%60)
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

func cmdPomo(args []string) error {
	all := append([]string{"--short"}, args...)
	return cmdFocus(all)
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", envOr("TTCLI_WEBHOOK_ADDR", "127.0.0.1:8787"), "listen address")
	secret := fs.String("secret", os.Getenv("TTCLI_WEBHOOK_SECRET"), "optional bearer token")
	push := fs.String("push-script", envOr("TTCLI_POMO_PUSH_SCRIPT", ""), "script to update tmux (default ~/.config/zsh/ttcli-pomo-push.sh)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg := webhook.Config{Addr: *addr, Secret: *secret, PushScript: *push}
	return webhook.Serve(cfg)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseFocusDay(args []string) (time.Time, error) {
	if len(args) >= 1 {
		d, err := time.Parse("2006-01-02", args[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("bad date %q (want YYYY-MM-DD): %w", args[0], err)
		}
		return d, nil
	}
	return time.Time{}, nil
}

func pomoGoal() int {
	if g := os.Getenv("TTCLI_POMO_GOAL"); g != "" {
		if n, err := strconv.Atoi(g); err == nil && n > 0 {
			return n
		}
	}
	return 30
}

func formatPomoStatus(count int) string {
	return fmt.Sprintf("%d/%d", count, pomoGoal())
}

func writePomoCache(count int) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "ttcli")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "pomo-status")
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%d %d\n", count, time.Now().Unix())
}

func runPomoPush(count int) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	script := os.Getenv("TTCLI_POMO_PUSH_SCRIPT")
	if script == "" {
		script = filepath.Join(home, ".config", "zsh", "ttcli-pomo-push.sh")
	}
	if _, err := os.Stat(script); err != nil {
		return
	}
	cmd := exec.Command("/bin/bash", script, strconv.Itoa(count))
	_ = cmd.Run()
}

func cmdRaw(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: ttcli raw <api-path>  (e.g. /api/v2/projects)")
	}
	c, err := client()
	if err != nil {
		return err
	}
	b, err := c.GetRaw(args[0])
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func priorityValue(s string) int {
	switch strings.ToLower(s) {
	case "high":
		return 5
	case "medium", "med":
		return 3
	case "low":
		return 1
	default:
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
		return 0
	}
}
