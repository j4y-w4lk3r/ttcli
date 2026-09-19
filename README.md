# ttcli

TickTick from the terminal — browse projects, tasks, calendars, pomodoro/focus
records, and habits from a responsive TUI.

> ⚠️ **Personal tool, unofficial API.** `ttcli` drives TickTick's private web API
> (`api.ticktick.com`) using a captured browser session, not the official OAuth
> Open API. It is not affiliated with TickTick and may break when their web app
> changes. Use it for your own account.

## Install

**Release binary** (versioned builds from GitHub releases):

```bash
# Homebrew (macOS)
brew install --cask j4y-w4lk3r/ttcli/ttcli

# Or download a tarball from https://github.com/j4y-w4lk3r/ttcli/releases
```

**From source** (stamps git describe into the binary):

```bash
git clone https://github.com/j4y-w4lk3r/ttcli.git && cd ttcli
make install          # → ~/.local/bin/ttcli
# or: make build && ./ttcli version
```

Plain `go install github.com/j4y-w4lk3r/ttcli/cmd/ttcli@latest` also works; `ttcli
version` falls back to the module pseudo-version and embedded git metadata when
release ldflags are not set.

Check version: `ttcli version` (also `ttcli -v`, `ttcli --version`).

## Releases

Versioning is **semver tags** (`v0.1.0`, `v0.2.0`, …):

1. Tag on `main`: `git tag v0.1.0 && git push origin v0.1.0`
2. GitHub Actions runs [GoReleaser](.goreleaser.yaml): cross-compiled tarballs,
   GitHub release, Homebrew cask bump, AUR `ttcli-bin` push.
3. Local dry-run: `make release-snapshot` (writes to `dist/`).

CI on every push/PR runs tests plus `goreleaser build --snapshot` so release
config stays valid before you tag.

## Auth

Credentials come from **1Password** (same pattern as `bmcctl`):

```bash
brew install --cask 1password-cli
op signin                    # or enable Touch ID unlock

# ttcli searches all vaults for a Login item titled "TickTick"
ttcli login                  # finds Employee/TickTick automatically
```

Override vault or item if needed:

```bash
ttcli login --vault Employee --item TickTick
# or env (item location only — not the password):
export TTCLI_OP_VAULT=Employee
export TTCLI_OP_ITEM=TickTick
```

**Note:** if you have an older Homebrew `ttcli` on PATH, rebuild from source:
`make install` then run `~/go/bin/ttcli login` (or `hash -r` so go/bin wins).

With a valid session on disk, `ttcli` **auto-refreshes** on `401` by re-reading
1Password — so a long-running box keeps working without manual re-login.

A captured session is read from the first of:

1. `$TICKTICK_AUTH_FILE`
2. `~/.ticktick_auth.json`
3. `./ticktick_auth.json`

The file is shaped like:

```json
{
  "cookies":  { "t": "...", "_csrf_token": "..." },
  "headers":  { "x-csrftoken": "..." },
  "saved_at": "2026-06-06T19:00:00Z"
}
```

Cookies are sent as a `Cookie` header; `_csrf_token` is echoed back as
`x-csrftoken`. The session file is written `0600`.

## Usage

```bash
ttcli login                              # mint/refresh a session
ttcli                                    # tasks · calendar · pomo · habits (press ? for keys)
ttcli tui                                # explicit equivalent
ttcli ls                                 # list projects/lists
ttcli tasks <project>                    # live tasks in a project (id or name)
ttcli add "buy milk" -p Home -P high     # create a task (flags before or after title)
ttcli done <project> <task-id>           # mark complete
ttcli rm <project> <task-id>             # delete
ttcli focus [YYYY-MM-DD]                 # pomodoro/focus summary for a day
ttcli focus start --duration 5 --title "test" -p Inbox   # live 5m timer
ttcli focus status | pause | stop        # control timer (stop logs to TickTick)
ttcli raw /api/v2/...                    # GET an arbitrary API path (debug)
ttcli version
```

`add` priority accepts `none|low|medium|high`. `project` accepts an id, a
name (case-insensitive), or `inbox`.

### TUI cache and daily check-ins

The TUI renders the last successful snapshot immediately, then refreshes it in
the background. Press `r` to bypass cache TTLs and force a network refresh.
Snapshots are stored as a versioned `0600` file at
`$XDG_CACHE_HOME/ttcli/data-v1.json` (default
`~/.cache/ttcli/data-v1.json`). If TickTick is unavailable, cached reads remain
visible with a `cached … ago` footer marker. Remote writes are not queued while
offline.

Task actions deliberately distinguish completion from a daily check-in:

- `d` / `Enter` completes or reopens an ordinary task in TickTick. Recurring
  tasks are guarded from this action; use `x`.
- Press `c`/`C` to move forward/backward through the persisted
  `Open → Done → Trash → All → Archive` scopes. To undo an accidental
  completion, use Done, select the task, then press `d` or `Enter`.
- `x` checks the selected task in for today. On a recurring task this completes
  the current TickTick occurrence, allowing TickTick to advance the series. On
  an ordinary task this records local history without changing its status or
  due date.
- `Backspace` moves live tasks to TickTick Trash. In Trash, `d`/`Enter`
  restores a task; `Backspace` opens an explicit permanent-delete
  confirmation.
- Before permanent deletion, ttcli writes the full private-API task object to
  `$XDG_STATE_HOME/ttcli/task-archive.json`. Archive scope is searchable and
  `d`/`Enter` recreates a snapshot as a new TickTick task.
- `/` searches title, notes, and tags inside the current scope. Counts report
  total, matching, and shown rows separately.
- In Calendar Day or Week view, `x` checks in or undoes the selected entry. Future
  dates cannot be checked in.

Local check-in history is durable user state at
`$XDG_STATE_HOME/ttcli/task-checkins.json` (default
`~/.local/state/ttcli/task-checkins.json`). Calendar marks these entries
`local`; they do not sync to TickTick or another machine. Native recurring
completion does sync. The Calendar shows completion/check-in history on the
day it happened and the task's due date remains unchanged for local check-ins.
Standard `RRULE` recurrence and custom `ERULE:…;BYDATE=…` dates are expanded
across the visible Day, Week, Month, or Year range. Unsupported TickTick
extensions remain unchanged and show the current open occurrence.

The task form treats Time as the block's **start time** and computes its end
from Duration. Planned focus accepts minutes or pomodoros (`75m` or `3p`) and
writes TickTick's native `focusSummaries` estimate, so the budget syncs.
Times are entered and displayed in the local timezone without shifting the
wall-clock value through stale task timezone metadata.
Repeat supports `none`, `daily`, `weekdays`, `weekly`, or a validated custom
`RRULE`/`ERULE`, plus due-date or completion-date repeat basis.

Task rows show completed/planned pomodoros and remaining focus (for example
`1/3 · 50m left`). The selected-task detail includes invested/planned time,
while the list footer estimates total planned and remaining focus for all open
tasks. Lists and task rows containing fallback estimates are marked with `~`.

### Pomodoro designs

Press `v` in the Pomodoro view to cycle between three persisted focus-panel
designs: the 12-segment task-colored arc, a compact linear focus bar, and a
status card. Live designs show elapsed versus remaining time, paused sessions
use blue, and overtime uses red. Press `z` separately to change timeline
density. The status card shows focused duration once, then daily-goal context.
Use `j`/`k` for line-level scrolling, `J`/`K` to jump between sessions, and
`t` to center and follow now.

### Calendar planning

Calendar Day uses a wide agenda/coaching split and a compact stacked layout on
narrow terminals. The coach shows planned, logged, and remaining focus,
completed/planned pomodoros, capacity, slack, live pace, a deterministic next
move, and selected-task details. Native planned focus is preferred over a
calendar block; tasks without either use the configurable fallback.

Overdue rows are hidden by default; press `o` to show or hide them. Hidden
overdue work still counts in the coach and is disclosed there. Feasibility is
reported honestly as `On track`, `Tight`, or `Over capacity`, with a
`high`/`medium`/`low` evidence label—not a probability. Pace is expressed as
minutes ahead or behind a linear configured-workday baseline.

Calendar is consistently Monday-first, with Sunday as the seventh column.
Week keeps all-day work and per-day capacity visible above an adaptive
work-hours timeline; `j`/`k` selects tasks, `PgUp`/`PgDn` scrolls, `z` toggles
compact/stretch density, `t` centers now in Day/Week, and `x` checks tasks in. Timed work is
drawn proportionally to duration. Month cells reserve a footer for completed
pomodoros versus the daily goal, for example `3/30`.

Planning defaults live alongside other TUI preferences in
`$XDG_CONFIG_HOME/ttcli/tui.json` (default `~/.config/ttcli/tui.json`):

```json
{
  "workStart": "09:00",
  "workEnd": "18:00",
  "planningBufferMinutes": 30,
  "defaultTaskMinutes": 25,
  "weekStartsOn": "monday",
  "taskScope": "open",
  "calendarWeekDensity": "stretch",
  "pomoFocusDesign": "arc",
  "pomoDailyGoal": 30,
  "calendarDayShowOverdue": false
}
```

`workStart` and `workEnd` use local 24-hour `HH:MM` time. Invalid values safely
fall back to the defaults. Calendar normalizes `weekStartsOn` to `monday`.
In the Pomodoro view, `+` and `-` adjust and persist `pomoDailyGoal`.

Automated tests use local HTTP fixtures and never complete a task in a live
TickTick account.

## Status

Implemented: login/auto-refresh, `ls`, `tasks`, `add`, `done`, `rm`,
`focus` (summary + live timer with TickTick sync), `raw`, **`tui`**
(four views, cached startup/offline reads, daily task check-ins, habit
check-ins, scrollable `?` help, Nerd Font icons).
Planned: KOReader reading telemetry and an Open API OAuth option.
See [docs/ROADMAP.md](docs/ROADMAP.md) for TUI phases and API strategy.
