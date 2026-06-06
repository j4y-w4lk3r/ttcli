# ttcli

TickTick from the terminal — list your projects, tasks, and (soon) pomodoro/focus
records, designed to plug into a self-hosted planning + learning loop.

> ⚠️ **Personal tool, unofficial API.** `ttcli` drives TickTick's private web API
> (`api.ticktick.com`) using a captured browser session, not the official OAuth
> Open API. It is not affiliated with TickTick and may break when their web app
> changes. Use it for your own account.

## Install

```bash
go install github.com/j4y-w4lk3r/ttcli/cmd/ttcli@latest
```

Homebrew tap and AUR package are planned (same flow as `bmcctl`).

## Auth

The headless-friendly path: set your credentials and let `ttcli` mint and
refresh the session itself.

```bash
export TICKTICK_EMAIL=you@example.com
export TICKTICK_PASSWORD='…'
ttcli login          # writes ~/.ticktick_auth.json
```

With those env vars set, `ttcli` also **auto-refreshes** on a `401` — so a
long-running box (e.g. `ru0`/`nas0`) keeps working without manual re-login.

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
ttcli ls                                 # list projects/lists
ttcli tasks <project>                    # live tasks in a project (id or name)
ttcli add "buy milk" -p Home -P high     # create a task (flags before or after title)
ttcli done <project> <task-id>           # mark complete
ttcli rm <project> <task-id>             # delete
ttcli focus [YYYY-MM-DD]                 # pomodoro/focus summary for a day
ttcli raw /api/v2/...                    # GET an arbitrary API path (debug)
ttcli version
```

`add` priority accepts `none|low|medium|high`. `project` accepts an id, a
name (case-insensitive), or `inbox`.

## Status

Implemented: login/auto-refresh, `ls`, `tasks`, `add`, `done`, `rm`,
`focus`, `raw`.
Planned: goreleaser + Homebrew tap + AUR packaging, KOReader reading
telemetry, and a coaching daemon for `ru0`/`nas0`.
