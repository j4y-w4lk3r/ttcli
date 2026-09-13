package focus

import "time"

// EnrichPauseWork fills WorkBefore/WorkAfter on spells using the active session file.
func EnrichPauseWork(spells []PauseSpell, sess *Session, now time.Time) []PauseSpell {
	if sess == nil || !sess.Active() || sess.TaskTitle == "" {
		return spells
	}
	workStart := sess.segmentStartedAt()
	if workStart.IsZero() {
		workStart = sess.StartedAt
	}
	for i := range sess.PauseIntervals {
		iv := sess.PauseIntervals[i]
		if iv.Start.IsZero() {
			continue
		}
		for j := range spells {
			if !tasksMatch(spells[j].TaskTitle, sess.TaskTitle) {
				continue
			}
			if !spells[j].Start.Equal(iv.Start) {
				continue
			}
			if spells[j].WorkBefore == 0 {
				before := iv.Start.Sub(workStart)
				if before > 0 {
					spells[j].WorkBefore = before
				}
			}
			if spells[j].End.IsZero() && !iv.End.IsZero() {
				spells[j].End = iv.End
			}
		}
		if !iv.End.IsZero() {
			workStart = iv.End
		}
	}
	if sess.State == StatePaused && sess.PausedAt != nil {
		for j := range spells {
			if !tasksMatch(spells[j].TaskTitle, sess.TaskTitle) || !spells[j].End.IsZero() {
				continue
			}
			if !spells[j].Start.Equal(*sess.PausedAt) && !spells[j].Start.IsZero() {
				continue
			}
			if spells[j].WorkBefore == 0 {
				before := sess.PausedAt.Sub(workStart)
				if before > 0 {
					spells[j].WorkBefore = before
				}
			}
		}
		return spells
	}
	for j := range spells {
		if !tasksMatch(spells[j].TaskTitle, sess.TaskTitle) || spells[j].End.IsZero() || spells[j].WorkAfter > 0 {
			continue
		}
		if !workStart.Before(now) {
			continue
		}
		after := now.UTC().Sub(workStart)
		if after > 0 {
			spells[j].WorkAfter = after
		}
	}
	return spells
}
