# ttcli roadmap

## TUI

| Phase | Status | Features |
|-------|--------|----------|
| **1** | done | List tree · task pane · views 1–4 · `?` help overlay |
| **2** | done | Pomodoro timer · calendar · habits · nerd font icons · responsive layout |
| **3** | planned | Habit check-in · mouse · Open API OAuth option |

Run: `ttcli tui`

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
