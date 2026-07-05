package main

import (
	"fmt"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func cmdTree(args []string) error {
	return cmdLists(append([]string{"--tree"}, args...))
}

func cmdFolder(args []string) error {
	if len(args) == 0 {
		args = []string{"ls"}
	}
	sub := args[0]
	rest := args[1:]
	c, err := client()
	if err != nil {
		return err
	}
	switch sub {
	case "ls", "list":
		gs, err := c.ListProjectGroups()
		if err != nil {
			return err
		}
		ps, err := c.ListProjects()
		if err != nil {
			return err
		}
		fmt.Print(ticktick.FormatProjectTree(ticktick.ProjectTree(gs, ps)))
		return nil
	case "add", "mkdir", "create":
		if len(rest) < 1 {
			return fmt.Errorf("usage: ttcli folder add <name>")
		}
		name := strings.Join(rest, " ")
		id, err := c.CreateProjectGroup(name)
		if err != nil {
			return err
		}
		fmt.Printf("✓ created folder %q (%s)\n", name, id)
		return nil
	case "rename", "mv":
		if len(rest) < 2 {
			return fmt.Errorf("usage: ttcli folder rename <folder> <new-name>")
		}
		newName := rest[len(rest)-1]
		oldRef := strings.Join(rest[:len(rest)-1], " ")
		if err := c.RenameProjectGroup(oldRef, newName); err != nil {
			return err
		}
		fmt.Printf("✓ renamed folder %q → %q\n", oldRef, newName)
		return nil
	case "rm", "delete":
		if len(rest) < 1 {
			return fmt.Errorf("usage: ttcli folder rm <folder>")
		}
		ref := strings.Join(rest, " ")
		if err := c.DeleteProjectGroup(ref); err != nil {
			return err
		}
		fmt.Printf("✓ deleted folder %q\n", ref)
		return nil
	default:
		return fmt.Errorf("usage: ttcli folder [ls|add|rename|rm]")
	}
}

func cmdProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage:
  ttcli project add <name> [--folder FOLDER] [--color #hex]
  ttcli project rename <list> <new-name>
  ttcli project move <list> --folder FOLDER|none
  ttcli project rm <list>`)
	}
	sub := args[0]
	rest := args[1:]
	c, err := client()
	if err != nil {
		return err
	}
	switch sub {
	case "add", "create":
		name, folder, color := "", "", ""
		var nameParts []string
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			next := func() string {
				if i+1 < len(rest) {
					i++
					return rest[i]
				}
				return ""
			}
			switch a {
			case "--folder", "-f":
				folder = next()
			case "--color":
				color = next()
			default:
				nameParts = append(nameParts, a)
			}
		}
		name = strings.TrimSpace(strings.Join(nameParts, " "))
		if name == "" {
			return fmt.Errorf("usage: ttcli project add <name> [--folder FOLDER]")
		}
		id, err := c.CreateProject(name, folder, color, "TASK")
		if err != nil {
			return err
		}
		fmt.Printf("✓ created list %q (%s)\n", name, id)
		return nil
	case "rename":
		if len(rest) < 2 {
			return fmt.Errorf("usage: ttcli project rename <list> <new-name>")
		}
		newName := rest[len(rest)-1]
		ref := strings.Join(rest[:len(rest)-1], " ")
		if err := c.RenameProject(ref, newName); err != nil {
			return err
		}
		fmt.Printf("✓ renamed list %q → %q\n", ref, newName)
		return nil
	case "move", "mv":
		ref, folder := "", ""
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			switch a {
			case "--folder", "-f":
				if i+1 < len(rest) {
					i++
					folder = rest[i]
				}
			default:
				if ref == "" {
					ref = a
				}
			}
		}
		if ref == "" || folder == "" {
			return fmt.Errorf("usage: ttcli project move <list> --folder FOLDER|none")
		}
		if err := c.MoveProject(ref, folder); err != nil {
			return err
		}
		fmt.Printf("✓ moved list %q → folder %q\n", ref, folder)
		return nil
	case "rm", "delete":
		if len(rest) < 1 {
			return fmt.Errorf("usage: ttcli project rm <list>")
		}
		ref := strings.Join(rest, " ")
		if err := c.DeleteProject(ref); err != nil {
			return err
		}
		fmt.Printf("✓ deleted list %q\n", ref)
		return nil
	default:
		return fmt.Errorf("unknown project subcommand %q", sub)
	}
}

func cmdEdit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: ttcli edit <task> [--title T] [-n NOTE] [-P none|low|medium|high] [-p LIST]")
	}
	query := args[0]
	var title, note, project, prio string
	setTitle := false
	for i := 1; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch a {
		case "--title", "-t":
			title = next()
			setTitle = true
		case "-n", "--note":
			note = next()
		case "-P", "--priority":
			prio = next()
		case "-p", "--project", "--list":
			project = next()
		default:
			return fmt.Errorf("unknown flag %q", a)
		}
	}
	edit := ticktick.TaskEdit{Project: project}
	if setTitle {
		edit.Title = &title
	}
	if note != "" {
		edit.Content = &note
	}
	if prio != "" {
		p := priorityValue(prio)
		edit.Priority = &p
	}
	if edit.Title == nil && edit.Content == nil && edit.Priority == nil && edit.Project == "" {
		return fmt.Errorf("nothing to edit — pass --title, -n, -P, or -p")
	}
	c, err := client()
	if err != nil {
		return err
	}
	if err := c.EditTask(query, edit); err != nil {
		return err
	}
	fmt.Printf("✓ updated task %q\n", query)
	return nil
}
