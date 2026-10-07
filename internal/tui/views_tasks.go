package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func (m model) renderTasksView(l layout) string {
	focusLeft := m.paneFocus == paneLists && (m.mode == modeNormal || m.mode == modeListSearch)
	focusRight := m.paneFocus == paneTasks && (m.mode == modeNormal || m.mode == modeFilter)

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
	if m.mode == modeFilter {
		if h := m.keyHint("↑↓ select · enter close · esc clear"); h != "" {
			hint = truncateInner(h, contentW)
		}
	} else if h := m.keyHint("z layout · j/k task"); h != "" {
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
	if !isSmartList(m.projectID) && !m.showsTaskListName() {
		title += "  " + hintStyle.Render("["+m.effectiveTaskScope().Label()+"]")
	}
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
	if m.mode == modeFilter {
		hint = truncateRenderedWidth(m.filterInput.View(), contentW)
	} else if m.taskDetailUsesSide(l) {
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
	focusColW := m.visibleFocusColW
	var listName func(string) string
	if m.showsTaskListName() {
		listName = m.listName
	}
	rowLayout := computeTaskRowLayoutNamed(tasks, contentW, focusColW, listName)
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
	if m.mode == modeListSearch {
		return m.renderListSearch(contentW, l.innerLines)
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

	projectEnd := len(m.listRows)
	for i, row := range m.listRows {
		if row.node.Kind == "smart" || row.node.Kind == "rule" {
			projectEnd = i
			break
		}
	}
	smartCount := len(m.listRows) - projectEnd
	maxRows := paneScrollRows(l.innerLines, 0)
	projectBudget := maxRows - smartCount
	if projectBudget < 1 {
		projectBudget = 1
	}
	scrollCursor := m.listCursor
	if scrollCursor >= projectEnd {
		scrollCursor = max(projectEnd-1, 0)
	}
	win := computeScrollWindow(scrollCursor, projectEnd, projectBudget)
	hint := ""
	if m.mode == modeReorderList {
		hint = truncateInner(hintStyle.Render("j/k · enter · esc"), contentW)
	} else if projectEnd > projectBudget {
		hint = truncateInner(m.scrollHint(win), contentW)
	}

	var items []string
	for i := win.Start; i < win.End && len(items) < projectBudget; i++ {
		items = append(items, m.renderListRow(m.listRows[i], i, contentW))
	}
	for i := projectEnd; i < len(m.listRows) && len(items) < maxRows; i++ {
		items = append(items, m.renderListRow(m.listRows[i], i, contentW))
	}
	return fillInner(title, hint, items, l.innerLines)
}

func (m model) renderListSearch(contentW, innerLines int) string {
	title := truncateInner(headerStyle.Render(iconList+" Find list"), contentW)
	matches := m.listSearchMatches()
	hint := ""
	if h := m.keyHint("enter open · esc cancel · ↑/↓ match"); h != "" {
		hint = truncateInner(h, contentW)
	}
	maxRows := paneScrollRows(innerLines, 0)
	if maxRows > 2 {
		maxRows -= 2
	}
	if m.listSearchCursor >= len(matches) {
		m.listSearchCursor = 0
	}
	win := computeScrollWindow(m.listSearchCursor, len(matches), maxRows)
	var items []string
	items = append(items, truncateRenderedWidth(m.listSearchInput.View(), contentW))
	if len(matches) == 0 {
		items = append(items, truncateInner(hintStyle.Render("(no matching list)"), contentW))
	}
	for i := win.Start; i < win.End && len(items) < maxRows+1; i++ {
		row := m.listRows[matches[i]]
		items = append(items, m.renderListRowSelected(row, contentW, i == m.listSearchCursor))
	}
	return fillInner(title, hint, items, innerLines)
}

func (m model) renderListRow(r listRow, index, contentW int) string {
	return m.renderListRowSelected(r, contentW, index == m.listCursor && m.mode != modeListSearch)
}

func (m model) renderListRowSelected(r listRow, contentW int, selected bool) string {
	indent := strings.Repeat("  ", r.node.Depth)
	switch r.node.Kind {
	case "folder":
		folderIcon := iconFolder
		if selected {
			folderIcon = iconFolderOpen
		}
		prefix := indent + folderIcon + " "
		name := ansi.Truncate(r.node.Name, max(1, contentW-ansi.StringWidth(prefix)), "…")
		style := folderStyle
		if selected {
			style = listSelStyle
		}
		return truncateRenderedWidth(style.Render(prefix+name), contentW)
	case "empty":
		return truncateInner(hintStyle.Render(indent+"(empty)"), contentW)
	case "rule":
		return truncateInner(sectionRuleStyle.Render(strings.Repeat("─", max(contentW, 1))), contentW)
	default:
		icon := iconTaskOpen
		if r.node.Kind == "all" {
			icon = iconFilter
		}
		if r.node.Kind == "smart" {
			icon = smartListIcon(r.node.ID)
		}
		if selected && icon == iconTaskOpen {
			icon = iconListSelected
		}
		prefix := indent + icon + " "
		name := ansi.Truncate(r.node.Name, max(1, contentW-ansi.StringWidth(prefix)), "…")
		style := listIdleStyle
		if selected {
			style = listSelStyle
		}
		return truncateRenderedWidth(style.Render(prefix+name), contentW)
	}
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
	if !isSmartList(m.projectID) && !m.showsTaskListName() {
		title += "  " + hintStyle.Render("["+m.effectiveTaskScope().Label()+"]")
	}
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
	if m.mode == modeAskAI || m.mode == modeAskAIPreview {
		return m.renderAskAI(contentW, l.innerLines)
	}

	if m.loading {
		return fillInner(title, truncateInner(hintStyle.Render("loading…"), contentW), nil, l.innerLines)
	}

	tasks := m.visibleTaskRows()
	if len(tasks) == 0 {
		total, matching, _ := m.taskScopeStats()
		label := strings.ToLower(m.taskListScope().Label())
		if m.projectID != allTasksID && m.showsTaskListName() {
			label = "open"
		}
		msg := fmt.Sprintf("(no %s tasks)", label)
		if total > 0 && matching == 0 && strings.TrimSpace(m.filterInput.Value()) != "" {
			msg = fmt.Sprintf("(%d %s tasks · no search matches)", total, label)
		}
		hint := truncateInner(hintStyle.Render(msg), contentW)
		var items []string
		if m.mode == modeFilter {
			hint = truncateRenderedWidth(m.filterInput.View(), contentW)
			items = []string{truncateInner(hintStyle.Render(msg), contentW)}
		}
		return fillInner(title, hint, items, l.innerLines)
	}

	detailBudget := 0
	if _, ok := m.selectedTask(); m.showTaskDetail() && ok && !m.taskDetailUsesSide(l) {
		detailBudget = m.taskDetailLineBudget(l.innerLines)
	}
	return m.renderTasksList(l, detailBudget)
}

func (m model) selectedTask() (ticktick.Task, bool) {
	rows := m.visibleTaskRows()
	if len(rows) == 0 || m.taskCursor < 0 || m.taskCursor >= len(rows) {
		return ticktick.Task{}, false
	}
	return rows[m.taskCursor].Task, true
}

func (m model) openDoneHint() string {
	total, matching, shown := m.taskScopeStats()
	scopeLabel := m.taskListScope().Label()
	if m.projectID != allTasksID && m.showsTaskListName() {
		scopeLabel = "open"
	}
	parts := []string{fmt.Sprintf("%s · %d total", scopeLabel, total)}
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

func closedTaskStyle(t ticktick.Task) lipgloss.Style {
	switch {
	case t.Trashed():
		return taskTrashedStyle
	case t.WontDo():
		return taskWontStyle
	default:
		return taskCompletedStyle
	}
}

func (m model) taskRowListName(t ticktick.Task) string {
	if t.ProjectID == "" {
		return ""
	}
	if m.showsTaskListName() || (t.ProjectID != m.projectID && realListID(m.projectID)) {
		return m.listName(t.ProjectID)
	}
	return ""
}

func (m model) taskHasChildren(t ticktick.Task) bool {
	if t.ID == "" {
		return false
	}
	byID := make(map[string]ticktick.Task, len(m.tasks))
	for _, other := range m.tasks {
		if other.ID != "" {
			byID[other.ID] = other
		}
	}
	for _, id := range t.ChildIDs {
		child, ok := byID[id]
		if !ok || child.ParentID == "" || child.ParentID == t.ID {
			return true
		}
	}
	for _, other := range m.tasks {
		if other.ParentID == t.ID {
			return true
		}
	}
	return false
}

func (m model) formatTaskLine(t ticktick.Task, depth int, selected, marked bool, contentW int, layout taskRowLayout) string {
	marker := iconTaskOpen
	style := taskIdleStyle
	hasChildren := m.taskHasChildren(t)

	switch {
	case t.Trashed():
		marker = iconTrash
		style = taskTrashedStyle
	case t.WontDo():
		marker = iconWont
		style = taskWontStyle
	case t.Done():
		marker = iconCheck
		style = taskCompletedStyle
	case hasChildren:
		style = taskWontStyle
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
	closed := t.Trashed() || t.WontDo() || t.Done()
	if closed && marked {
		style = style.Foreground(colorTeal).Bold(true)
	}
	if hasChildren && marked && !closed {
		style = style.Foreground(colorTeal).Bold(true)
	}
	if selected {
		style = style.Bold(true).Background(colorOverlay)
	}
	prefix := tree + style.Render(marker) + " "

	title := style.Render(displayText(t.Title))
	if name := m.taskRowListName(t); name != "" {
		namePart := hintStyle.Render(" · " + name)
		budget := layout.TitleColW - lipgloss.Width(namePart)
		if budget < 4 {
			budget = 4
		}
		if lipgloss.Width(title) > budget {
			title = truncateRenderedWidth(title, budget)
		}
		title += namePart
	}

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
	line := formatTaskRow(prefix, title, focusPart, due, layout, contentW)
	if !selected {
		return line
	}
	plain := stripANSI(line)
	if gap := contentW - lipgloss.Width(plain); gap > 0 {
		plain += strings.Repeat(" ", gap)
	}
	return truncateRenderedWidth(style.Render(plain), contentW)
}
