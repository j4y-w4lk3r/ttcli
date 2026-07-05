package main

import "strings"

// commandHelp documents one ttcli subcommand for help, completions, and TUIs.
type commandHelp struct {
	Name     string
	Summary  string
	Usage    string
	Examples []string
	Group    string
	Aliases  []string
}

var commandGroups = []struct {
	Name string
}{
	{Name: "getting started"},
	{Name: "lists & folders"},
	{Name: "tasks"},
	{Name: "focus & misc"},
}

func allCommands() []commandHelp {
	return []commandHelp{
		{Name: "login", Group: "getting started", Summary: "Mint session via 1Password", Usage: "ttcli login [--vault V] [--item I]", Examples: []string{"ttcli login", "ttcli login --vault Employee --item TickTick"}},
		{Name: "status", Group: "getting started", Summary: "Version + session health", Usage: "ttcli status [--ping]", Examples: []string{"ttcli status", "ttcli status --ping"}},
		{Name: "tree", Group: "lists & folders", Summary: "Folder → list tree", Usage: "ttcli tree", Aliases: []string{"folders"}, Examples: []string{"ttcli tree", "ttcli ls --tree"}},
		{Name: "ls", Group: "lists & folders", Summary: "List projects (flat)", Usage: "ttcli ls [--tree] [--names] [--all]", Aliases: []string{"lists", "projects"}, Examples: []string{"ttcli ls", "ttcli ls --names"}},
		{Name: "folder", Group: "lists & folders", Summary: "Manage folders", Usage: "ttcli folder [ls|add|rename|rm] ...", Examples: []string{"ttcli folder add personal", "ttcli folder rename x work"}},
		{Name: "project", Group: "lists & folders", Summary: "Manage lists", Usage: "ttcli project [add|rename|move|rm] ...", Aliases: []string{"list"}, Examples: []string{"ttcli project add Ideas --folder personal", "ttcli project move PXC --folder x"}},
		{Name: "tasks", Group: "tasks", Summary: "List tasks in a project", Usage: "ttcli tasks <list|all> [--completed]", Examples: []string{"ttcli tasks PXC", "ttcli tasks all", "ttcli tasks --completed"}},
		{Name: "add", Group: "tasks", Summary: "Create a task", Usage: "ttcli add <title> [-p LIST] [-P prio] [-n NOTE]", Examples: []string{`ttcli add "buy milk" -p Home -P high`}},
		{Name: "edit", Group: "tasks", Summary: "Edit task fields", Usage: "ttcli edit <task> [--title T] [-n NOTE] [-P prio] [-p LIST]", Examples: []string{`ttcli edit "🔟 - CLI" --title "CLI tools"`}},
		{Name: "due", Group: "tasks", Summary: "Reschedule a task", Usage: "ttcli due <task> -d YYYY-MM-DD [-t HH:MM]", Aliases: []string{"schedule"}, Examples: []string{`ttcli due "🔟 - CLI" -d 2026-07-03 -t 10:00`}},
		{Name: "done", Group: "tasks", Summary: "Complete a task", Usage: "ttcli done <list> <task-id>", Examples: []string{"ttcli done PXC 69f52ef1413e916fa01f1be8"}},
		{Name: "rm", Group: "tasks", Summary: "Delete a task", Usage: "ttcli rm <list> <task-id>", Aliases: []string{"delete"}, Examples: []string{"ttcli rm PXC 69f52ef1413e916fa01f1be8"}},
		{Name: "focus", Group: "focus & misc", Summary: "Pomodoro summary", Usage: "ttcli focus [YYYY-MM-DD] [--short]", Examples: []string{"ttcli focus", "ttcli focus --short"}},
		{Name: "pomo", Group: "focus & misc", Summary: "Short pomodoro line (tmux)", Usage: "ttcli pomo", Examples: []string{"ttcli pomo"}},
		{Name: "shell", Group: "focus & misc", Summary: "Interactive REPL", Usage: "ttcli shell", Aliases: []string{"repl"}, Examples: []string{"ttcli shell"}},
		{Name: "help", Group: "focus & misc", Summary: "Help ([cmd] | gum | bt)", Usage: "ttcli help [command|gum|bt]", Examples: []string{"ttcli help tasks", "ttcli help gum", "ttcli help bt"}},
		{Name: "raw", Group: "focus & misc", Summary: "Debug API GET", Usage: "ttcli raw <api-path>", Examples: []string{"ttcli raw /api/v2/projects"}},
		{Name: "version", Group: "focus & misc", Summary: "Print version", Usage: "ttcli version", Aliases: []string{"-v", "--version"}},
	}
}

func lookupCommand(name string) (commandHelp, bool) {
	for _, c := range allCommands() {
		if stringsEqual(name, c.Name) {
			return c, true
		}
		for _, a := range c.Aliases {
			if stringsEqual(name, a) {
				return c, true
			}
		}
	}
	return commandHelp{}, false
}

func stringsEqual(a, b string) bool {
	return strings.EqualFold(a, b)
}
