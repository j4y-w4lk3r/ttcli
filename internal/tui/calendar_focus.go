package tui

import (
	"strings"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type calLoggedSpan struct {
	Entry    calEntry
	Start    time.Time
	End      time.Time
	Estimate planning.TaskEstimate
}

func calendarLoggedSpans(idx calIndex, stats *ticktick.FocusStats, day time.Time) []calLoggedSpan {
	if stats == nil {
		return nil
	}
	day = dateOnly(day)
	var out []calLoggedSpan
	for _, record := range stats.Records {
		spans := loggedSpansFromRecord(idx, record, day)
		out = append(out, spans...)
	}
	return out
}

func loggedSpansFromRecord(idx calIndex, record ticktick.FocusRecord, day time.Time) []calLoggedSpan {
	recStart, recStartOK := focusTimeLocal(record.StartTime)
	recEnd, recEndOK := focusTimeLocal(record.EndTime)
	if recStartOK && (!recEndOK || !recEnd.After(recStart)) {
		recEnd = recStart.Add(ticktick.RecordDuration(record))
		recEndOK = true
	}
	type piece struct {
		taskID, title string
		start, end    time.Time
	}
	var pieces []piece
	if len(record.Tasks) == 0 {
		if !recStartOK {
			return nil
		}
		pieces = append(pieces, piece{
			title: record.TaskTitle(),
			start: recStart,
			end:   recEnd,
		})
	} else {
		for _, task := range record.Tasks {
			start, ok := focusTimeLocal(task.StartTime)
			if !ok {
				start, ok = recStart, recStartOK
			}
			if !ok {
				continue
			}
			end, endOK := focusTimeLocal(task.EndTime)
			if !endOK {
				end, endOK = recEnd, recEndOK
			}
			if !endOK || !end.After(start) {
				end = start.Add(ticktick.RecordDuration(record))
			}
			title := strings.TrimSpace(task.Title)
			if title == "" {
				title = record.TaskTitle()
			}
			pieces = append(pieces, piece{
				taskID: task.TaskID,
				title:  title,
				start:  start,
				end:    end,
			})
		}
	}
	var out []calLoggedSpan
	for _, piece := range pieces {
		start, end := clipIntervalToDay(piece.start, piece.end, day)
		if start.IsZero() || !end.After(start) {
			continue
		}
		title := strings.TrimSpace(piece.title)
		if title == "" {
			title = "Focus"
		}
		minutes := int(end.Sub(start).Minutes() + 0.5)
		if minutes < 1 {
			minutes = 1
		}
		entry := idx.entryForFocus(piece.taskID, title, day)
		entry.Task.IsAllDay = false
		entry.Task.StartDate = start.Format("2006-01-02T15:04:05.000-0700")
		entry.Task.DueDate = end.Format("2006-01-02T15:04:05.000-0700")
		out = append(out, calLoggedSpan{
			Entry: entry,
			Start: start,
			End:   end,
			Estimate: planning.TaskEstimate{
				Minutes:  minutes,
				Pomos:    (minutes + ticktick.StandardPomoMinutes - 1) / ticktick.StandardPomoMinutes,
				Explicit: true,
				Source:   planning.EstimateLogged,
			},
		})
	}
	return out
}

func clipIntervalToDay(start, end, day time.Time) (time.Time, time.Time) {
	day = dateOnly(day)
	next := day.AddDate(0, 0, 1)
	if start.IsZero() {
		return time.Time{}, time.Time{}
	}
	if end.IsZero() || !end.After(start) {
		end = start.Add(ticktick.StandardPomoMinutes * time.Minute)
	}
	if !end.After(day) || !start.Before(next) {
		return time.Time{}, time.Time{}
	}
	if start.Before(day) {
		start = day
	}
	if end.After(next) {
		end = next
	}
	return start, end
}

func (idx calIndex) entryForFocus(taskID, title string, day time.Time) calEntry {
	day = dateOnly(day)
	if taskID != "" {
		for _, entry := range idx.on(day) {
			if entryMatchesFocusID(entry, taskID) {
				return entry
			}
		}
		for _, entries := range idx.byDate {
			for _, entry := range entries {
				if entryMatchesFocusID(entry, taskID) {
					copyEntry := entry
					copyEntry.Date = day
					return copyEntry
				}
			}
		}
	}
	key := ticktick.NormalizeFocusTaskTitle(title)
	if key != "" {
		for _, entry := range idx.on(day) {
			if ticktick.NormalizeFocusTaskTitle(entry.Task.Title) == key {
				return entry
			}
		}
	}
	return calEntry{
		Task: ticktick.Task{ID: taskID, Title: title},
		Date: day,
	}
}

func entryMatchesFocusID(entry calEntry, taskID string) bool {
	if taskID == "" {
		return false
	}
	if entry.Task.ID == taskID {
		return true
	}
	series := entry.Task.SeriesID()
	return series != "" && series == taskID
}

func loggedSpanOverlapsScheduled(span calLoggedSpan, scheduled []calEntry, config planning.Config) bool {
	for _, entry := range scheduled {
		if !sameFocusTask(span.Entry, entry) {
			continue
		}
		_, start, end := calTaskSchedule(entry, config)
		if start.IsZero() {
			continue
		}
		if end.IsZero() || !end.After(start) {
			end = start.Add(time.Duration(max(span.Estimate.Minutes, 1)) * time.Minute)
			if minutes := planning.EstimateTask(entry.Task, config).Minutes; minutes > 0 {
				end = start.Add(time.Duration(minutes) * time.Minute)
			}
		}
		if span.Start.Before(end) && span.End.After(start) {
			return true
		}
	}
	return false
}

func sameFocusTask(a, b calEntry) bool {
	if a.Task.ID != "" {
		if a.Task.ID == b.Task.ID || entryMatchesFocusID(b, a.Task.ID) {
			return true
		}
	}
	if b.Task.ID != "" && entryMatchesFocusID(a, b.Task.ID) {
		return true
	}
	ta := ticktick.NormalizeFocusTaskTitle(a.Task.Title)
	tb := ticktick.NormalizeFocusTaskTitle(b.Task.Title)
	return ta != "" && ta == tb
}

func visibleLoggedSpans(idx calIndex, stats *ticktick.FocusStats, scheduled []calEntry, day time.Time, config planning.Config) []calLoggedSpan {
	var out []calLoggedSpan
	for _, span := range calendarLoggedSpans(idx, stats, day) {
		if loggedSpanOverlapsScheduled(span, scheduled, config) {
			continue
		}
		out = append(out, span)
	}
	return out
}
