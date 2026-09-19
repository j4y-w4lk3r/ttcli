package ticktick

import (
	"fmt"
	"sync"
	"time"
)

const (
	treeCacheTTL         = 5 * time.Minute
	taskCacheTTL         = 45 * time.Second
	completedTodayTTL    = 45 * time.Second
	completedPastTTL     = 24 * time.Hour
	focusTodayTTL        = 30 * time.Second
	focusPastTTL         = 24 * time.Hour
	focusHistoryCacheTTL = 15 * time.Minute
	habitsCacheTTL       = time.Minute
)

type repositoryFlight struct {
	done chan struct{}
	val  any
	err  error
}

// Repository provides read-through memory and disk snapshots for TUI data.
// Writes still go directly through Client and invalidate affected snapshots.
type Repository struct {
	client *Client
	path   string

	mu        sync.RWMutex
	persistMu sync.Mutex
	snapshot  dataSnapshot
	flights   map[string]*repositoryFlight
}

// NewRepository loads the default persistent TUI snapshot.
func NewRepository(client *Client) *Repository {
	path, _ := defaultSnapshotPath()
	return newRepository(client, path)
}

func newRepository(client *Client, path string) *Repository {
	return &Repository{
		client:   client,
		path:     path,
		snapshot: loadDataSnapshot(path),
		flights:  map[string]*repositoryFlight{},
	}
}

func (r *Repository) persist() {
	r.persistMu.Lock()
	defer r.persistMu.Unlock()
	r.mu.RLock()
	defer r.mu.RUnlock()
	_ = writeDataSnapshot(r.path, r.snapshot)
}

func (r *Repository) fetch(key string, fn func() (any, error)) (any, error) {
	r.mu.Lock()
	if flight, ok := r.flights[key]; ok {
		r.mu.Unlock()
		<-flight.done
		return flight.val, flight.err
	}
	flight := &repositoryFlight{done: make(chan struct{})}
	r.flights[key] = flight
	r.mu.Unlock()

	flight.val, flight.err = fn()

	r.mu.Lock()
	delete(r.flights, key)
	close(flight.done)
	r.mu.Unlock()
	return flight.val, flight.err
}

func cacheMeta[T any](e cacheEntry[T], ttl time.Duration) CacheMeta {
	return CacheMeta{
		SavedAt:   e.SavedAt,
		FromCache: true,
		Stale:     !e.fresh(time.Now(), ttl),
	}
}

func remoteMeta(savedAt time.Time) CacheMeta {
	return CacheMeta{SavedAt: savedAt}
}

func cloneTasks(tasks []Task) []Task {
	return append([]Task(nil), tasks...)
}

func filterProjectTasks(tasks []Task, projectID string) []Task {
	filtered := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if task.ProjectID != "" && task.ProjectID != projectID {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}

func cloneProjects(projects []Project) []Project {
	return append([]Project(nil), projects...)
}

func cloneGroups(groups []ProjectGroup) []ProjectGroup {
	return append([]ProjectGroup(nil), groups...)
}

func cloneTreeSnapshot(tree TreeSnapshot) TreeSnapshot {
	tree.Groups = cloneGroups(tree.Groups)
	tree.Projects = cloneProjects(tree.Projects)
	return tree
}

func cloneFocusStats(stats FocusStats) *FocusStats {
	out := stats
	out.Records = append([]FocusRecord(nil), stats.Records...)
	return &out
}

func cloneHabitsSnapshot(s HabitsSnapshot) HabitsSnapshot {
	s.Habits = append([]Habit(nil), s.Habits...)
	checkins := make(map[string]HabitCheckin, len(s.Checkins))
	for id, checkin := range s.Checkins {
		checkins[id] = checkin
	}
	s.Checkins = checkins
	return s
}

// CachedTree returns the last project tree snapshot without network I/O.
func (r *Repository) CachedTree() (TreeSnapshot, CacheMeta, bool) {
	r.mu.RLock()
	entry := r.snapshot.Tree
	r.mu.RUnlock()
	if !entry.present() {
		return TreeSnapshot{}, CacheMeta{}, false
	}
	return cloneTreeSnapshot(entry.Value), cacheMeta(entry, treeCacheTTL), true
}

// Tree returns folders, projects, and Inbox metadata.
func (r *Repository) Tree(force bool) (TreeSnapshot, CacheMeta, error) {
	r.mu.RLock()
	entry := r.snapshot.Tree
	r.mu.RUnlock()
	if !force && entry.fresh(time.Now(), treeCacheTTL) {
		return cloneTreeSnapshot(entry.Value), cacheMeta(entry, treeCacheTTL), nil
	}
	value, err := r.fetch("tree", func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		groups, err := r.client.ListProjectGroups()
		if err != nil {
			return nil, err
		}
		projects, err := r.client.ListProjects()
		if err != nil {
			return nil, err
		}
		inboxID, inboxErr := r.client.InboxID()
		if inboxErr != nil {
			inboxID = ""
		}
		tree := TreeSnapshot{Groups: groups, Projects: projects, InboxID: inboxID}
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.Tree = cacheEntry[TreeSnapshot]{Value: cloneTreeSnapshot(tree), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return tree, nil
	})
	if err == nil {
		return cloneTreeSnapshot(value.(TreeSnapshot)), remoteMeta(time.Now()), nil
	}
	if entry.present() {
		meta := cacheMeta(entry, treeCacheTTL)
		meta.Stale = true
		return cloneTreeSnapshot(entry.Value), meta, err
	}
	return TreeSnapshot{}, CacheMeta{}, err
}

// CachedProjectTasks returns one list's last task snapshot.
func (r *Repository) CachedProjectTasks(projectID string) ([]Task, CacheMeta, bool) {
	r.mu.RLock()
	entry, ok := r.snapshot.ProjectTasks[projectID]
	r.mu.RUnlock()
	if !ok || !entry.present() {
		return nil, CacheMeta{}, false
	}
	return cloneTasks(filterProjectTasks(entry.Value, projectID)), cacheMeta(entry, taskCacheTTL), true
}

// ProjectTasks returns a list's tasks with short-lived read-through caching.
func (r *Repository) ProjectTasks(projectID string, force bool) ([]Task, CacheMeta, error) {
	r.mu.RLock()
	entry, ok := r.snapshot.ProjectTasks[projectID]
	r.mu.RUnlock()
	if ok && !force && entry.fresh(time.Now(), taskCacheTTL) {
		return cloneTasks(filterProjectTasks(entry.Value, projectID)), cacheMeta(entry, taskCacheTTL), nil
	}
	value, err := r.fetch("tasks:"+projectID, func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		tasks, err := r.client.ProjectTasks(projectID)
		if err != nil {
			return nil, err
		}
		tasks = filterProjectTasks(tasks, projectID)
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.ProjectTasks[projectID] = cacheEntry[[]Task]{Value: cloneTasks(tasks), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return tasks, nil
	})
	if err == nil {
		return cloneTasks(value.([]Task)), remoteMeta(time.Now()), nil
	}
	if ok && entry.present() {
		meta := cacheMeta(entry, taskCacheTTL)
		meta.Stale = true
		return cloneTasks(filterProjectTasks(entry.Value, projectID)), meta, err
	}
	return nil, CacheMeta{}, err
}

// CachedOpenTasks returns the account-wide open-task snapshot.
func (r *Repository) CachedOpenTasks() ([]Task, CacheMeta, bool) {
	r.mu.RLock()
	entry := r.snapshot.OpenTasks
	r.mu.RUnlock()
	if !entry.present() {
		return nil, CacheMeta{}, false
	}
	return cloneTasks(entry.Value), cacheMeta(entry, taskCacheTTL), true
}

// OpenTasks returns account-wide open tasks for Calendar and focus pickers.
func (r *Repository) OpenTasks(force bool) ([]Task, CacheMeta, error) {
	r.mu.RLock()
	entry := r.snapshot.OpenTasks
	r.mu.RUnlock()
	if !force && entry.fresh(time.Now(), taskCacheTTL) {
		return cloneTasks(entry.Value), cacheMeta(entry, taskCacheTTL), nil
	}
	value, err := r.fetch("open-tasks", func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		tasks, err := r.client.AllOpenTasks()
		if err != nil {
			return nil, err
		}
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.OpenTasks = cacheEntry[[]Task]{Value: cloneTasks(tasks), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return tasks, nil
	})
	if err == nil {
		return cloneTasks(value.([]Task)), remoteMeta(time.Now()), nil
	}
	if entry.present() {
		meta := cacheMeta(entry, taskCacheTTL)
		meta.Stale = true
		return cloneTasks(entry.Value), meta, err
	}
	return nil, CacheMeta{}, err
}

func dayTTL(day time.Time, todayTTL, pastTTL time.Duration) time.Duration {
	if localDateOnly(day).Equal(localDateOnly(time.Now())) {
		return todayTTL
	}
	return pastTTL
}

// CachedCompletedOn returns one local day's completed task snapshot.
func (r *Repository) CachedCompletedOn(day time.Time) ([]Task, CacheMeta, bool) {
	key := localDateOnly(day).Format("2006-01-02")
	r.mu.RLock()
	entry, ok := r.snapshot.CompletedDays[key]
	r.mu.RUnlock()
	if !ok || !entry.present() {
		return nil, CacheMeta{}, false
	}
	ttl := dayTTL(day, completedTodayTTL, completedPastTTL)
	return cloneTasks(entry.Value), cacheMeta(entry, ttl), true
}

// CompletedOn returns tasks completed on one local day.
func (r *Repository) CompletedOn(day time.Time, force bool) ([]Task, CacheMeta, error) {
	day = localDateOnly(day)
	key := day.Format("2006-01-02")
	ttl := dayTTL(day, completedTodayTTL, completedPastTTL)
	r.mu.RLock()
	entry, ok := r.snapshot.CompletedDays[key]
	r.mu.RUnlock()
	if ok && !force && entry.fresh(time.Now(), ttl) {
		return cloneTasks(entry.Value), cacheMeta(entry, ttl), nil
	}
	value, err := r.fetch("completed:"+key, func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		tasks, err := r.client.TasksCompletedOn(day)
		if err != nil {
			return nil, err
		}
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.CompletedDays[key] = cacheEntry[[]Task]{Value: cloneTasks(tasks), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return tasks, nil
	})
	if err == nil {
		return cloneTasks(value.([]Task)), remoteMeta(time.Now()), nil
	}
	if ok && entry.present() {
		meta := cacheMeta(entry, ttl)
		meta.Stale = true
		return cloneTasks(entry.Value), meta, err
	}
	return nil, CacheMeta{}, err
}

// CompletedBetween returns completed tasks for an inclusive local date range.
// Successful range responses are split into per-day snapshots for reuse by the
// Pomodoro day view.
func (r *Repository) CompletedBetween(startDay, endDay time.Time, force bool) ([]Task, CacheMeta, error) {
	startDay = localDateOnly(startDay)
	endDay = localDateOnly(endDay)
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}
	var cached []Task
	allFresh := !force
	haveAny := false
	oldest := time.Time{}
	r.mu.RLock()
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		entry, ok := r.snapshot.CompletedDays[key]
		ttl := dayTTL(day, completedTodayTTL, completedPastTTL)
		if !ok || !entry.present() {
			allFresh = false
			continue
		}
		haveAny = true
		cached = append(cached, entry.Value...)
		if oldest.IsZero() || entry.SavedAt.Before(oldest) {
			oldest = entry.SavedAt
		}
		if !entry.fresh(time.Now(), ttl) {
			allFresh = false
		}
	}
	r.mu.RUnlock()
	if allFresh {
		return cloneTasks(cached), CacheMeta{SavedAt: oldest, FromCache: true}, nil
	}
	flightKey := "completed-range:" + startDay.Format("2006-01-02") + ":" + endDay.Format("2006-01-02")
	value, err := r.fetch(flightKey, func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		tasks, err := r.client.TasksCompletedBetween(startDay, endDay)
		if err != nil {
			return nil, err
		}
		byDay := make(map[string][]Task)
		for _, task := range tasks {
			completedAt, parseErr := ParseAPITime(task.CompletedT)
			if parseErr != nil {
				continue
			}
			key := completedAt.In(time.Local).Format("2006-01-02")
			byDay[key] = append(byDay[key], task)
		}
		savedAt := time.Now()
		r.mu.Lock()
		for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
			key := day.Format("2006-01-02")
			r.snapshot.CompletedDays[key] = cacheEntry[[]Task]{
				Value: cloneTasks(byDay[key]), SavedAt: savedAt,
			}
		}
		r.mu.Unlock()
		r.persist()
		return tasks, nil
	})
	if err == nil {
		return cloneTasks(value.([]Task)), remoteMeta(time.Now()), nil
	}
	if haveAny {
		return cloneTasks(cached), CacheMeta{SavedAt: oldest, FromCache: true, Stale: true}, err
	}
	return nil, CacheMeta{}, err
}

// CachedFocusDay returns one local day's pomodoro summary.
func (r *Repository) CachedFocusDay(day time.Time) (*FocusStats, CacheMeta, bool) {
	key := localDateOnly(day).Format("2006-01-02")
	r.mu.RLock()
	entry, ok := r.snapshot.FocusDays[key]
	r.mu.RUnlock()
	if !ok || !entry.present() {
		return nil, CacheMeta{}, false
	}
	ttl := dayTTL(day, focusTodayTTL, focusPastTTL)
	return cloneFocusStats(entry.Value), cacheMeta(entry, ttl), true
}

// FocusDay returns one local day's pomodoro summary.
func (r *Repository) FocusDay(day time.Time, force bool) (*FocusStats, CacheMeta, error) {
	day = localDateOnly(day)
	key := day.Format("2006-01-02")
	ttl := dayTTL(day, focusTodayTTL, focusPastTTL)
	r.mu.RLock()
	entry, ok := r.snapshot.FocusDays[key]
	r.mu.RUnlock()
	if ok && !force && entry.fresh(time.Now(), ttl) {
		return cloneFocusStats(entry.Value), cacheMeta(entry, ttl), nil
	}
	value, err := r.fetch("focus-day:"+key, func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		stats, err := r.client.FocusForDay(day)
		if err != nil {
			return nil, err
		}
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.FocusDays[key] = cacheEntry[FocusStats]{Value: *cloneFocusStats(*stats), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return *stats, nil
	})
	if err == nil {
		stats := value.(FocusStats)
		return cloneFocusStats(stats), remoteMeta(time.Now()), nil
	}
	if ok && entry.present() {
		meta := cacheMeta(entry, ttl)
		meta.Stale = true
		return cloneFocusStats(entry.Value), meta, err
	}
	return nil, CacheMeta{}, err
}

func focusStatsFromRecords(day time.Time, records []FocusRecord) FocusStats {
	day = localDateOnly(day)
	dayRecords := FocusRecordsOnDay(records, day)
	stats := FocusStats{
		Date:      day.Format("2006-01-02"),
		PomoCount: len(dayRecords),
		Records:   append([]FocusRecord(nil), dayRecords...),
	}
	stats.FullPomoCount = CountFullPomos(dayRecords)
	for _, record := range dayRecords {
		stats.TotalSeconds += int64(RecordDuration(record).Seconds())
	}
	return stats
}

func cloneFocusDayMap(days map[string]FocusStats) map[string]*FocusStats {
	out := make(map[string]*FocusStats, len(days))
	for key, stats := range days {
		out[key] = cloneFocusStats(stats)
	}
	return out
}

// FocusBetween returns one summary per local day using a single range request.
func (r *Repository) FocusBetween(startDay, endDay time.Time, force bool) (map[string]*FocusStats, CacheMeta, error) {
	startDay = localDateOnly(startDay)
	endDay = localDateOnly(endDay)
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}
	cached := make(map[string]FocusStats)
	allFresh := !force
	oldest := time.Time{}
	r.mu.RLock()
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		entry, ok := r.snapshot.FocusDays[key]
		ttl := dayTTL(day, focusTodayTTL, focusPastTTL)
		if !ok || !entry.present() {
			allFresh = false
			continue
		}
		cached[key] = *cloneFocusStats(entry.Value)
		if oldest.IsZero() || entry.SavedAt.Before(oldest) {
			oldest = entry.SavedAt
		}
		if !entry.fresh(time.Now(), ttl) {
			allFresh = false
		}
	}
	r.mu.RUnlock()
	if allFresh {
		return cloneFocusDayMap(cached), CacheMeta{SavedAt: oldest, FromCache: true}, nil
	}

	rangeKey := "focus-range:" + startDay.Format("2006-01-02") + ":" + endDay.Format("2006-01-02")
	value, err := r.fetch(rangeKey, func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		start, _ := LocalDayBounds(startDay)
		_, end := LocalDayBounds(endDay)
		records, err := r.client.FocusForRange(start, end)
		if err != nil {
			return nil, err
		}
		savedAt := time.Now()
		days := make(map[string]FocusStats)
		r.mu.Lock()
		for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
			stats := focusStatsFromRecords(day, records)
			key := stats.Date
			days[key] = stats
			r.snapshot.FocusDays[key] = cacheEntry[FocusStats]{Value: stats, SavedAt: savedAt}
		}
		r.mu.Unlock()
		r.persist()
		return days, nil
	})
	if err == nil {
		return cloneFocusDayMap(value.(map[string]FocusStats)), remoteMeta(time.Now()), nil
	}
	if len(cached) > 0 {
		return cloneFocusDayMap(cached), CacheMeta{SavedAt: oldest, FromCache: true, Stale: true}, err
	}
	return nil, CacheMeta{}, err
}

// CachedFocusHistory returns the cached multi-day focus records.
func (r *Repository) CachedFocusHistory() ([]FocusRecord, CacheMeta, bool) {
	r.mu.RLock()
	entry := r.snapshot.FocusHistory
	r.mu.RUnlock()
	if !entry.present() {
		return nil, CacheMeta{}, false
	}
	return append([]FocusRecord(nil), entry.Value...), cacheMeta(entry, focusHistoryCacheTTL), true
}

// FocusHistory returns the configured focus-history window.
func (r *Repository) FocusHistory(force bool) ([]FocusRecord, CacheMeta, error) {
	r.mu.RLock()
	entry := r.snapshot.FocusHistory
	r.mu.RUnlock()
	if !force && entry.fresh(time.Now(), focusHistoryCacheTTL) {
		return append([]FocusRecord(nil), entry.Value...), cacheMeta(entry, focusHistoryCacheTTL), nil
	}
	value, err := r.fetch("focus-history", func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		now := time.Now()
		start := now.AddDate(0, 0, -FocusHistoryDays())
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999000000, now.Location())
		records, err := r.client.FocusForRange(start, end)
		if err != nil {
			return nil, err
		}
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.FocusHistory = cacheEntry[[]FocusRecord]{Value: append([]FocusRecord(nil), records...), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return records, nil
	})
	if err == nil {
		return append([]FocusRecord(nil), value.([]FocusRecord)...), remoteMeta(time.Now()), nil
	}
	if entry.present() {
		meta := cacheMeta(entry, focusHistoryCacheTTL)
		meta.Stale = true
		return append([]FocusRecord(nil), entry.Value...), meta, err
	}
	return nil, CacheMeta{}, err
}

// CachedHabits returns today's habit and check-in snapshot.
func (r *Repository) CachedHabits(day time.Time) (HabitsSnapshot, CacheMeta, bool) {
	key := localDateOnly(day).Format("2006-01-02")
	r.mu.RLock()
	entry := r.snapshot.Habits
	r.mu.RUnlock()
	if !entry.present() || entry.Value.Day != key {
		return HabitsSnapshot{}, CacheMeta{}, false
	}
	return cloneHabitsSnapshot(entry.Value), cacheMeta(entry, habitsCacheTTL), true
}

// HabitsForDay returns active habits and the selected day's done check-ins.
func (r *Repository) HabitsForDay(day time.Time, force bool) (HabitsSnapshot, CacheMeta, error) {
	day = localDateOnly(day)
	key := day.Format("2006-01-02")
	r.mu.RLock()
	entry := r.snapshot.Habits
	r.mu.RUnlock()
	if entry.Value.Day == key && !force && entry.fresh(time.Now(), habitsCacheTTL) {
		return cloneHabitsSnapshot(entry.Value), cacheMeta(entry, habitsCacheTTL), nil
	}
	value, err := r.fetch("habits:"+key, func() (any, error) {
		if r.client == nil {
			return nil, fmt.Errorf("TickTick client unavailable")
		}
		habits, err := r.client.ListHabits()
		if err != nil {
			return nil, err
		}
		ids := make([]string, len(habits))
		for i, habit := range habits {
			ids[i] = habit.ID
		}
		checkins, err := r.client.HabitCheckinsForDay(ids, day)
		if err != nil {
			return nil, err
		}
		snapshot := HabitsSnapshot{Day: key, Habits: habits, Checkins: checkins}
		savedAt := time.Now()
		r.mu.Lock()
		r.snapshot.Habits = cacheEntry[HabitsSnapshot]{Value: cloneHabitsSnapshot(snapshot), SavedAt: savedAt}
		r.mu.Unlock()
		r.persist()
		return snapshot, nil
	})
	if err == nil {
		return cloneHabitsSnapshot(value.(HabitsSnapshot)), remoteMeta(time.Now()), nil
	}
	if entry.present() && entry.Value.Day == key {
		meta := cacheMeta(entry, habitsCacheTTL)
		meta.Stale = true
		return cloneHabitsSnapshot(entry.Value), meta, err
	}
	return HabitsSnapshot{}, CacheMeta{}, err
}

// InvalidateTasks marks task-derived snapshots stale while retaining offline data.
func (r *Repository) InvalidateTasks(projectID string) {
	r.mu.Lock()
	if entry, ok := r.snapshot.ProjectTasks[projectID]; ok {
		entry.Dirty = true
		r.snapshot.ProjectTasks[projectID] = entry
	}
	r.snapshot.OpenTasks.Dirty = true
	for key, entry := range r.snapshot.CompletedDays {
		entry.Dirty = true
		r.snapshot.CompletedDays[key] = entry
	}
	r.mu.Unlock()
	r.persist()
}

// InvalidateTree marks folders/projects stale while retaining the snapshot.
func (r *Repository) InvalidateTree() {
	r.mu.Lock()
	r.snapshot.Tree.Dirty = true
	r.mu.Unlock()
	r.persist()
}

// InvalidateFocus marks the selected day and long-range focus index stale.
func (r *Repository) InvalidateFocus(day time.Time) {
	key := localDateOnly(day).Format("2006-01-02")
	r.mu.Lock()
	if entry, ok := r.snapshot.FocusDays[key]; ok {
		entry.Dirty = true
		r.snapshot.FocusDays[key] = entry
	}
	r.snapshot.FocusHistory.Dirty = true
	r.mu.Unlock()
	r.persist()
}

// InvalidateHabits marks habit/check-in data stale.
func (r *Repository) InvalidateHabits() {
	r.mu.Lock()
	r.snapshot.Habits.Dirty = true
	r.mu.Unlock()
	r.persist()
}
