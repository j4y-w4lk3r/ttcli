package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func (m model) renderTasksView(l layout) string {
	focusLeft := m.paneFocus == paneLists && m.mode == modeNormal
	focusRight := m.paneFocus == paneTasks && m.mode == modeNormal

	leftPane := renderPane(m.renderLists(l), l, l.leftBoxW, focusLeft)
	rightPane := m.renderTasksRightPane(l, focusRight)

	if l.narrow {
		if m.paneFocus == paneLists {
			return fitPaneLines(leftPane, l.bodyLines, l.termW)
		}
		return fitPaneLines(rightPane, l.bodyLines, l.termW)
	}
	return joinHorizPanes(leftPane, rightPane, l.bodyLines, l.leftBoxW, l.rightBoxW, l.termW)
}

func (m model) renderTasksRightPane(l layout, focused bool) string {
	if m.mode == modeConfirmDelete {
		return renderPane(m.renderTasks(l), l, l.rightBoxW, focused)
	}
	listBoxW, detailBoxW := taskDetailSplitWidths(l.rightBoxW)
	if m.taskDetailUsesSide(l) && listBoxW > 0 {
		t, ok := m.selectedTask()
		if ok {
			listPane := renderPane(m.renderTasksList(l.withBoxW(listBoxW), 0), l, listBoxW, focused)
			detailPane := renderPane(m.renderTasksDetailPane(t, l.withBoxW(detailBoxW)), l, detailBoxW, false)
			return joinHorizPanes(listPane, detailPane, l.bodyLines, listBoxW, detailBoxW, l.rightBoxW)
		}
	}
	return renderPane(m.renderTasks(l), l, l.rightBoxW, focused)
}

func (m model) renderTasksDetailPane(t ticktick.Task, l layout) string {
	contentW := l.rightW
	if contentW < 1 {
		contentW = 1
	}
	title := truncateInner(headerStyle.Render(iconEdit+" "+displayText(t.Title)), contentW)
	budget := paneScrollRows(l.innerLines, 0)
	detail := taskDetailPanelSide(t, m.taskFocusSummary, contentW, budget)
	hint := ""
	if h := m.keyHint("z layout · j/k task"); h != "" {
		hint = truncateInner(h, contentW)
	}
	return fillInner(title, hint, detail, l.innerLines)
}

func (m model) renderTasksList(l layout, detailBudget int) string {
	contentW := l.rightW
	if contentW < 1 {
		contentW = 1
	}
	title := headerStyle.Render(iconTasks + " Tasks")
	if m.projectName != "" {
		title = headerStyle.Render(iconTasks + " " + m.projectName)
	}
	title += "  " + hintStyle.Render("["+m.effectiveTaskScope().Label()+"]")
	title = truncateInner(title, contentW)

	tasks := m.visibleTaskRows()
	var detail []string
	if detailBudget > 0 {
		if t, ok := m.selectedTask(); ok {
			detail = padFooterLines(taskDetailPanel(t, m.taskFocusSummary, contentW, detailBudget), detailBudget)
		}
	}

	maxRows := paneScrollRows(l.innerLines, detailBudget)
	win := computeScrollWindow(m.taskCursor, len(tasks), maxRows)
	hint := m.tasksPaneHint(m.openDoneHint(), contentW)
	if m.taskDetailUsesSide(l) {
		if h := m.keyHint("z layout · j/k task"); h != "" {
			hint = truncateInner(h, contentW)
		}
	}
	if markHint := m.markedTasksHint(); markHint != "" {
		if hint != "" {
			hint = truncateInner(hint+" · "+markHint, contentW)
		} else {
			hint = truncateInner(hintStyle.Render(markHint), contentW)
		}
	}
	if len(tasks) > maxRows {
		scroll := truncateInner(m.scrollHint(win), contentW)
		if hint != "" && scroll != "" {
			hint = truncateInner(hint+" · "+scroll, contentW)
		} else if scroll != "" {
			hint = scroll
		}
	}

	var items []string
	focusColW := maxTaskFocusInlineW(m, tasks)
	rowLayout := computeTaskRowLayout(tasks, contentW, focusColW)
	for i := win.Start; i < win.End && len(items) < maxRows; i++ {
		row := tasks[i]
		line := m.formatTaskLine(row.Task, row.Depth, i == m.taskCursor, m.isTaskMarked(row.Task.ID), contentW, rowLayout)
		items = append(items, truncateRenderedWidth(line, contentW))
	}
	if detailBudget > 0 {
		return fillInnerWithFooter(title, hint, items, detail, l.innerLines, detailBudget)
	}
	return fillInner(title, hint, items, l.innerLines)
}

func (m model) renderLists(l layout) string {
	contentW := l.leftW
	if contentW < 1 {
		contentW = 1
	}
	title := truncateInner(headerStyle.Render(iconList+" Lists"), contentW)

	if m.mode == modeRenameList {
		var b strings.Builder
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(m.renameInput.View())
		b.WriteString("\n")
		if h := m.keyHint("enter save · esc cancel"); h != "" {
			b.WriteString(h)
		}
		return fitLines(b.String(), l.innerLines)
	}
	if m.mode == modeAddList {
		var b strings.Builder
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(truncateInner(hintStyle.Render("folder: "+folderDisplayLabel(m.addListFolder)), contentW))
		b.WriteString("\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
		if h := m.keyHint("ctrl+f change folder · enter create · esc cancel"); h != "" {
			b.WriteString(h)
		}
		return fitLines(b.String(), l.innerLines)
	}
	if m.mode == modeAddFolder {
		var b strings.Builder
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
		if h := m.keyHint("enter create · esc cancel"); h != "" {
			b.WriteString(h)
		}
		return fitLines(b.String(), l.innerLines)
	}

	if len(m.listRows) == 0 {
		return fillInner(title, truncateInner(hintStyle.Render("(no lists)"), contentW), nil, l.innerLines)
	}

	maxRows := paneScrollRows(l.innerLines, 0)
	win := computeScrollWindow(m.listCursor, len(m.listRows), maxRows)
	hint := ""
	if len(m.listRows) > maxRows {
		hint = truncateInner(m.scrollHint(win), contentW)
	}

	var items []string
	for i := win.Start; i < win.End && len(items) < maxRows; i++ {
		r := m.listRows[i]
		indent := strings.Repeat("  ", r.node.Depth)
		switch r.node.Kind {
		case "folder":
			prefix := indent + iconFolder + " "
			name := ansi.Truncate(r.node.Name, max(1, contentW-ansi.StringWidth(prefix)), "…")
			items = append(items, truncateInner(folderStyle.Render(prefix+name), contentW))
		case "empty":
			items = append(items, truncateInner(hintStyle.Render(indent+"(empty)"), contentW))
		default:
			prefix := indent + iconTaskOpen + " "
			name := ansi.Truncate(r.node.Name, max(1, contentW-ansi.StringWidth(prefix)), "…")
			style := listIdleStyle
			if i == m.listCursor {
				style = listSelStyle
			}
			items = append(items, truncateRenderedWidth(style.Render(prefix+name), contentW))
		}
	}
	return fillInner(title, hint, items, l.innerLines)
}

func (m model) renderTasks(l layout) string {
	contentW := l.rightW
	if contentW < 1 {
		contentW = 1
	}
	title := headerStyle.Render(iconTasks + " Tasks")
	if m.projectName != "" {
		title = headerStyle.Render(iconTasks + " " + m.projectName)
	}
	title += "  " + hintStyle.Render("["+m.effectiveTaskScope().Label()+"]")
	title = truncateInner(title, contentW)

	if m.mode == modeConfirmDelete {
		count := len(m.pendingPermanentDelete)
		items := []string{
			errStyle.Render(fmt.Sprintf("Permanently delete %d task(s)?", count)),
			"Full snapshots will be saved to the local archive first.",
			"This cannot be undone in TickTick.",
		}
		return fillInner(title, truncateInner(hintStyle.Render("y/Enter confirm · n/Esc cancel"), contentW), items, l.innerLines)
	}

	if m.mode == modeAddTask || m.mode == modeEditTask {
		return m.renderAddTaskForm(contentW, l.innerLines)
	}
	if m.mode == modeFilter {
		var b strings.Builder
		b.WriteString(title)
		b.WriteString("\n\n")
		b.WriteString(m.filterInput.View())
		b.WriteString("\n")
		if h := m.keyHint("enter apply · esc clear"); h != "" {
			b.WriteString(h)
		}
		return fitLines(b.String(), l.innerLines)
	}

	if m.loading {
		return fillInner(title, truncateInner(hintStyle.Render("loading…"), contentW), nil, l.innerLines)
	}

	tasks := m.visibleTaskRows()
	if len(tasks) == 0 {
		total, matching, _ := m.taskScopeStats()
		msg := fmt.Sprintf("(no %s tasks)", strings.ToLower(m.effectiveTaskScope().Label()))
		if total > 0 && matching == 0 && strings.TrimSpace(m.filterInput.Value()) != "" {
			msg = fmt.Sprintf("(%d %s tasks · no search matches)", total, strings.ToLower(m.effectiveTaskScope().Label()))
		}
		return fillInner(title, truncateInner(hintStyle.Render(msg), contentW), nil, l.innerLines)
	}

	detailBudget := 0
	if _, ok := m.selectedTask(); m.showTaskDetail() && ok && !m.taskDetailUsesSide(l) {
		detailBudget = m.taskDetailLineBudget(l.innerLines)
	}
	return m.renderTasksList(l, detailBudget)
}

func (m model) selectedTask() (ticktick.Task, bool) {
	tasks := m.visibleTasks()
	if len(tasks) == 0 || m.taskCursor < 0 || m.taskCursor >= len(tasks) {
		return ticktick.Task{}, false
	}
	return tasks[m.taskCursor], true
}

func (m model) openDoneHint() string {
	total, matching, shown := m.taskScopeStats()
	parts := []string{fmt.Sprintf("%s · %d total", m.effectiveTaskScope().Label(), total)}
	if strings.TrimSpace(m.filterInput.Value()) != "" {
		parts = append(parts, fmt.Sprintf("%d matching", matching))
	}
	parts = append(parts, fmt.Sprintf("%d shown", shown))
	if m.effectiveTaskScope() != TaskScopeArchive {
		if planned, remaining, inferred := m.listFocusEstimate(); planned > 0 {
			prefix := ""
			if inferred > 0 {
				prefix = "~"
			}
			parts = append(parts, fmt.Sprintf(
				"list %s%s planned · %s left",
				prefix, planning.FormatMinutes(planned), planning.FormatMinutes(remaining),
			))
		}
		parts = append(parts, "sort "+m.taskSortMode.Label())
	}
	return strings.Join(parts, " · ")
}

func (m model) listFocusEstimate() (planned, remaining, inferred int) {
	config := m.uiSettings.planningConfig()
	for _, task := range m.tasks {
		if task.Trashed() || task.Done() {
			continue
		}
		estimate := planning.EstimateTask(task, config)
		planned += estimate.Minutes
		if estimate.Source == planning.EstimateDefault {
			inferred++
		}
		invested := 0
		if summary, ok := m.taskFocusSummary(task); ok {
			invested = int((summary.TotalSeconds + 30) / 60)
		}
		remaining += max(estimate.Minutes-invested, 0)
	}
	return planned, remaining, inferred
}

func (m model) tasksPaneHint(extra string, contentW int) string {
	if extra == "" {
		return ""
	}
	return truncateInner(hintStyle.Render(extra), contentW)
}

func (m model) formatTaskLine(t ticktick.Task, depth int, selected, marked bool, contentW int, layout taskRowLayout) string {
	marker := iconTaskOpen
	style := taskIdleStyle

	switch {
	case t.Trashed():
		marker = iconTrash
		style = taskTrashedStyle
		if selected {
			style = taskTrashedStyle.Copy().Bold(true).Foreground(colorRed)
		}
	case t.Done():
		if marked {
			marker = iconTaskSel
		} else {
			marker = iconCheck
		}
		style = taskDoneStyle
		if selected {
			style = taskDoneStyle.Copy().Bold(true).Foreground(colorGreen)
		}
	case selected:
		if !marked {
			marker = iconTaskSel
		}
		style = taskSelStyle
	case marked:
		marker = iconCheck
		style = taskSelStyle.Copy().Foreground(colorTeal)
	}

	tree := taskTreePrefix(depth)
	if depth > 0 {
		tree = hintStyle.Render(tree)
	}
	prefix := tree + style.Render(marker) + " "

	var focusPart string
	if layout.FocusColW > 0 {
		if s, ok := m.taskFocusDisplay(t); ok {
			focusPart = padToWidth(
				renderTaskFocusProgressInlineWithConfig(t, s, m.uiSettings.planningConfig()),
				layout.FocusColW,
			) + strings.Repeat(" ", taskSuffixGap)
		} else {
			focusPart = strings.Repeat(" ", layout.FocusColW) + strings.Repeat(" ", taskSuffixGap)
		}
	} else if s, ok := m.taskFocusDisplay(t); ok {
		focusPart = renderTaskFocusProgressInlineWithConfig(
			t, s, m.uiSettings.planningConfig(),
		) + strings.Repeat(" ", taskSuffixGap)
	}
	due := padDueWidth(dueInlineTask(t))
	return formatTaskRow(prefix, style.Render(displayText(t.Title)), focusPart, due, layout, contentW)
}
