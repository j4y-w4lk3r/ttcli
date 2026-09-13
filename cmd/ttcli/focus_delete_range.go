package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func cmdFocusDeleteRange(args []string) error {
	fs := flag.NewFlagSet("focus delete-range", flag.ExitOnError)
	task := fs.String("task", "", "task title substring (required)")
	from := fs.String("from", "", "local start time HH:MM, inclusive (required)")
	to := fs.String("to", "", "local start time HH:MM, inclusive (required)")
	date := fs.String("date", "", "calendar day YYYY-MM-DD (default today)")
	keep := fs.Int("keep", 0, "keep N earliest matches; delete the rest (0 = delete all matches)")
	includeUnclaimed := fs.Bool("include-unclaimed", false, "also match Unclaimed · … slices")
	dryRun := fs.Bool("dry-run", false, "list matches without deleting")
	yes := fs.Bool("yes", false, "delete without confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*task) == "" {
		return fmt.Errorf("usage: ttcli focus delete-range --task TITLE --from HH:MM --to HH:MM [--date YYYY-MM-DD] [--keep N] [--dry-run] [--yes]")
	}
	if strings.TrimSpace(*from) == "" || strings.TrimSpace(*to) == "" {
		return fmt.Errorf("--from and --to are required (HH:MM, local time)")
	}

	day := time.Now()
	if strings.TrimSpace(*date) != "" {
		d, err := time.Parse("2006-01-02", *date)
		if err != nil {
			return fmt.Errorf("bad --date %q: %w", *date, err)
		}
		day = d
	}

	c, err := client()
	if err != nil {
		return err
	}
	stats, err := c.FocusForDay(day)
	if err != nil {
		return err
	}
	matched, err := ticktick.FilterFocusRecords(stats.Records, ticktick.FocusRecordFilter{
		Day:              day,
		TaskSubstring:    *task,
		FromClock:        *from,
		ToClock:          *to,
		IncludeUnclaimed: *includeUnclaimed,
	})
	if err != nil {
		return err
	}
	if len(matched) == 0 {
		fmt.Println("no matching pomodoros")
		return nil
	}

	sort.Slice(matched, func(i, j int) bool {
		ti, _ := ticktick.ParseAPITime(matched[i].StartTime)
		tj, _ := ticktick.ParseAPITime(matched[j].StartTime)
		return ti.Before(tj)
	})

	toDelete := matched
	if *keep > 0 {
		if *keep >= len(matched) {
			fmt.Printf("nothing to delete — only %d match(es), --keep %d\n", len(matched), *keep)
			return nil
		}
		toDelete = matched[*keep:]
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	for _, r := range toDelete {
		start, end := focusRecordClockRange(r)
		fmt.Fprintf(w, "  %s–%s\t%s\t%s\n", start, end, r.TaskTitle(), r.ID)
	}
	_ = w.Flush()

	if *dryRun {
		fmt.Printf("\n(dry-run) would delete %d pomodoro(s)\n", len(toDelete))
		return nil
	}

	if !*yes {
		fmt.Printf("\nDelete %d pomodoro(s)? [y/N] ", len(toDelete))
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			fmt.Println("cancelled")
			return nil
		}
	}

	var failed int
	for _, r := range toDelete {
		if err := c.DeletePomodoro(r.ID); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "delete %s (%s): %v\n", r.ID, r.TaskTitle(), err)
		}
	}
	deleted := len(toDelete) - failed
	fmt.Printf("✓ deleted %d pomodoro(s)", deleted)
	if failed > 0 {
		fmt.Printf(" (%d failed)", failed)
	}
	fmt.Println()
	return nil
}

func focusRecordClockRange(r ticktick.FocusRecord) (start, end string) {
	start = "??"
	end = "??"
	if t, err := ticktick.ParseAPITime(r.StartTime); err == nil {
		start = t.Local().Format("15:04")
	}
	if t, err := ticktick.ParseAPITime(r.EndTime); err == nil {
		end = t.Local().Format("15:04")
	}
	return start, end
}
