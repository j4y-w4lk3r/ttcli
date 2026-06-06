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
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	args := os.Args[2:]

	var err error
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("ttcli %s\n", version)
		return
	case "help", "-h", "--help":
		usage()
		return
	case "login":
		err = cmdLogin(args)
	case "ls", "lists", "projects":
		err = cmdLists(args)
	case "tasks":
		err = cmdTasks(args)
	case "add":
		err = cmdAdd(args)
	case "done":
		err = cmdDone(args)
	case "rm", "delete":
		err = cmdRm(args)
	case "focus":
		err = cmdFocus(args)
	case "raw":
		err = cmdRaw(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ttcli: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ttcli — TickTick from the terminal

usage:
  ttcli login                       mint/refresh a session (TICKTICK_EMAIL/PASSWORD)
  ttcli ls                          list projects/lists
  ttcli tasks <project>             live tasks in a project (id or name)
  ttcli add <title> [-p PROJECT] [-P none|low|medium|high] [-n NOTE]
  ttcli done <project> <task-id>    mark a task complete
  ttcli rm <project> <task-id>      delete a task
  ttcli focus [YYYY-MM-DD]          pomodoro/focus summary for a day (default today)
  ttcli raw <api-path>              GET an arbitrary /api/... path (debug)
  ttcli version

auth:
  session is read from $TICKTICK_AUTH_FILE, else ~/.ticktick_auth.json,
  else ./ticktick_auth.json. With TICKTICK_EMAIL/PASSWORD set, ttcli mints
  and auto-refreshes the session itself (on 401).
`)
}

func client() (*ticktick.Client, error) { return ticktick.New("") }

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	email := fs.String("email", os.Getenv("TICKTICK_EMAIL"), "TickTick email (or $TICKTICK_EMAIL)")
	pass := fs.String("password", os.Getenv("TICKTICK_PASSWORD"), "TickTick password (or $TICKTICK_PASSWORD)")
	out := fs.String("out", "", "path to write the session file (default ~/.ticktick_auth.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := ticktick.Login(ticktick.Credentials{Email: *email, Password: *pass}, *out)
	if err != nil {
		return err
	}
	fmt.Printf("✓ logged in — session saved to %s\n", c.AuthPath)
	return nil
}

func cmdLists(_ []string) error {
	c, err := client()
	if err != nil {
		return err
	}
	ps, err := c.ListProjects()
	if err != nil {
		return err
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

func cmdTasks(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: ttcli tasks <project-id|project-name>")
	}
	c, err := client()
	if err != nil {
		return err
	}
	pid, err := c.ResolveProject(args[0])
	if err != nil {
		return err
	}
	tasks, err := c.ProjectTasks(pid)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tPRIO\tDUE\tID\tTITLE")
	for _, t := range tasks {
		status := "open"
		if t.Done() {
			status = "done"
		}
		due := t.DueDate
		if len(due) >= 10 {
			due = due[:10]
		}
		if due == "" {
			due = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", status, t.PriorityLabel(), due, t.ID, t.Title)
	}
	return w.Flush()
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
	var day time.Time
	if len(args) >= 1 {
		d, err := time.Parse("2006-01-02", args[0])
		if err != nil {
			return fmt.Errorf("bad date %q (want YYYY-MM-DD): %w", args[0], err)
		}
		day = d
	}
	c, err := client()
	if err != nil {
		return err
	}
	s, err := c.FocusForDay(day)
	if err != nil {
		return err
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
