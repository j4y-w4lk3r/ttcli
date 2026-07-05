package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type helpItem struct {
	cmd commandHelp
}

func (i helpItem) Title() string       { return i.cmd.Name }
func (i helpItem) Description() string { return i.cmd.Summary }
func (i helpItem) FilterValue() string {
	return i.cmd.Name + " " + i.cmd.Summary + " " + i.cmd.Group
}

type helpBrowseModel struct {
	list    list.Model
	detail  string
	showing bool
	width   int
	height  int
}

func (m helpBrowseModel) Init() tea.Cmd { return nil }

func (m helpBrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 6)
		return m, nil
	case tea.KeyMsg:
		if m.showing {
			switch msg.String() {
			case "esc", "q", "enter", "backspace":
				m.showing = false
				m.detail = ""
				return m, nil
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter", " ":
			if it, ok := m.list.SelectedItem().(helpItem); ok {
				m.detail = formatCommandDetail(it.cmd)
				m.showing = true
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m helpBrowseModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render("ttcli help")
	hint := lipgloss.NewStyle().Faint(true).Render("↑↓ navigate · enter detail · / filter · q quit")
	if m.showing {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Width(min(m.width-4, 72)).
			Render(m.detail)
		return title + "\n" + hint + "\n\n" + box + "\n\n" + lipgloss.NewStyle().Faint(true).Render("esc back")
	}
	return title + "\n" + hint + "\n\n" + m.list.View()
}

func cmdHelpBrowse() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("interactive help needs a terminal (TTY)\n  run: ttcli help gum  or  ttcli help tasks")
	}
	items := make([]list.Item, 0, len(allCommands()))
	for _, c := range allCommands() {
		items = append(items, helpItem{cmd: c})
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	l := list.New(items, delegate, 40, 14)
	l.Title = "Commands"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := helpBrowseModel{list: l}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
