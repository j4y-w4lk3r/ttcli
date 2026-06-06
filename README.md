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

`ttcli` reads a captured session from the first of:

1. `$TICKTICK_AUTH_FILE`
2. `~/.ticktick_auth.json`
3. `./ticktick_auth.json`

The file is shaped like:

```json
{
  "cookies":  { "t": "...", "_csrf_token": "..." },
  "headers":  { },
  "saved_at": "2026-04-14T10:38:00Z"
}
```

Cookies are sent as a `Cookie` header; `_csrf_token` is echoed back as
`x-csrftoken`. (Capture/login tooling is being ported from the original
`ticktick-automator`.)

## Usage

```bash
ttcli ls                 # list projects/lists
ttcli tasks <project>    # live tasks in a project (id or name)
ttcli raw /api/v2/...    # GET an arbitrary API path (debug)
ttcli version
```

## Status

Early. Implemented: session auth, `ls`, `tasks`, `raw`.
Planned: add/complete tasks, focus/pomodoro records, KOReader reading telemetry,
and a coaching daemon for `ru0`/`nas0`.
