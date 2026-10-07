# ttcli roadmap

## TUI

| Phase | Status | Features |
|-------|--------|----------|
| **1** | done | List tree · task pane · views 1–4 · `?` help overlay |
| **2** | done | Pomodoro timer · calendar · habits · habit check-in · nerd font icons · responsive layout |
| **3** | done | Pinned Completed, Won't Do, and Trash · list search · closed-list colors |
| **4** | done | Undo a deleted list · a project's Done view excludes Won't Do |
| **5** | done | `i` rewrites a task title and notes after a preview · Pushover on focus done · MCP tools |
| **6** | done | Undo the last task action · focus counts stay on the task id · Pushover when a reminder is due |
| **later** | planned | Open API OAuth · KOReader |

Run: `ttcli tui`

## MCP and Pushover

### MCP server

`ttcli mcp` speaks the Model Context Protocol on stdio. Tools: `search_tasks`, `get_task`, and `update_task_text`. In the TUI, `i` on a task asks for one instruction, shows the new title and notes, and saves on Enter. The model endpoint is `TTCLI_AI_API_KEY` and `TTCLI_AI_MODEL`, or a 1Password login named by `TTCLI_AI_OP_ITEM`.

Pushover is sent with the focus-finished desktop alert when `TTCLI_PUSHOVER_USER` and `TTCLI_PUSHOVER_TOKEN` are set, or when `TTCLI_PUSHOVER_OP_ITEM` names a 1Password item. A software-license field "license key" is the user key, and a field named "API token" is the application token. The pushover.net website login is not used.

### Pushover

The focus-finished alert also goes to Pushover when credentials are set. While the TUI is open, a task reminder does the same: a desktop notification and a Pushover message when the reminder time arrives. A timed reminder fires at the due time minus its offset. An all-day reminder fires at 09:00 local, minus that offset. One that is already more than a few minutes late is left alone.

## API strategy: private web API vs official Open API

ttcli intentionally uses **two layers**:

### Today — private web API (`api.ticktick.com`)

- Auth: browser session (1Password + `ttcli login`), auto-refresh on 401
- **Tasks/lists/folders**: full CRUD — what the web app uses
- **Focus/pomodoro read**: `GET /api/v2/pomodoros?from=&to=` (already in `FocusForDay`)
- **Pros**: no OAuth app registration, same data as the web UI, headless via 1Password
- **Cons**: unofficial, may break when TickTick changes the web client

### Official Open API (`developer.ticktick.com`)

- Auth: OAuth2 + `client_id` / `client_secret` (TickTick Developer portal)
- **Tasks/projects**: documented CRUD at `/open/v1/...`
- **Focus create**: `POST /open/v1/focus` (type `0` = Pomodoro, `1` = Timing)
- **Official npm CLI** (`@ticktick/ticktick-cli`): OAuth browser flow, Node.js
- **Pros**: supported, stable contract, focus **write** path
- **Cons**: separate OAuth app, rate limits, may lag behind web features

### Recommended hybrid (future)

| Feature | API |
|---------|-----|
| Task browser TUI, CRUD, tree | **Private** (current) |
| Read today's pomodoros, tmux `pomo` | **Private** `FocusForDay` |
| **Start/stop/log pomodoro** from CLI | **Private** `POST /api/v2/batch/pomodoro` (implemented) |
| CI / agents without 1Password | **Open API** OAuth token |
| Fallback when private API breaks | **Open API** read path |

Implementation sketch:

1. Add `internal/openapi/` client (OAuth token from env or `~/.config/ttcli/oauth.json`)
2. `ttcli focus start --task ID` → Open API create focus
3. Keep `ttcli focus` summary on private API until Open API exposes equivalent history
4. Optional: detect `@ticktick/ticktick-cli` as alternative for users who prefer npm

We do **not** need to replace ttcli with the official npm CLI — complementary:

- **ttcli**: Go, 1Password, homelab tmux integration, Bubble Tea TUI
- **@ticktick/ticktick-cli**: quick OAuth setup, AI agent docs
