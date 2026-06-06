// ttcli — a terminal client for TickTick (tasks, lists, pomodoros).
//
// Authenticated via a captured browser session (see internal/ticktick).
// This is a personal automation tool built on TickTick's private web API;
// it is not affiliated with TickTick and may break when their app changes.
package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("ttcli %s\n", version)
		return
	case "help", "-h", "--help":
		usage()
		return
	case "ls", "lists", "projects":
		err = cmdLists(args)
	case "tasks":
		err = cmdTasks(args)
	case "raw":
		err = cmdRaw(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
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
  ttcli ls                 list projects/lists
  ttcli tasks <project>    list live tasks in a project (by id or name)
  ttcli raw <api-path>     GET an arbitrary /api/... path (debug)
  ttcli version            print version

auth:
  session is read from $TICKTICK_AUTH_FILE, else ~/.ticktick_auth.json,
  else ./ticktick_auth.json  (JSON: {"cookies":{...},"headers":{...}})
`)
}

func client() (*ticktick.Client, error) {
	return ticktick.New("")
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
	q := args[0]
	c, err := client()
	if err != nil {
		return err
	}
	// Resolve a name to an id if needed.
	projectID := q
	ps, err := c.ListProjects()
	if err != nil {
		return err
	}
	if !looksLikeID(q) {
		var matched string
		for _, p := range ps {
			if strings.EqualFold(p.Name, q) {
				matched = p.ID
				break
			}
		}
		if matched == "" {
			return fmt.Errorf("no project named %q (try `ttcli ls`)", q)
		}
		projectID = matched
	}
	tasks, err := c.ProjectTasks(projectID)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tPRIO\tDUE\tTITLE")
	for _, t := range tasks {
		status := "[ ]"
		if t.Done() {
			status = "[x]"
		}
		due := t.DueDate
		if len(due) >= 10 {
			due = due[:10]
		}
		if due == "" {
			due = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", status, t.PriorityLabel(), due, t.Title)
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

func looksLikeID(s string) bool {
	if len(s) < 16 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
