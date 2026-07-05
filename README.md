# ttcli

TickTick from the terminal — list your projects, tasks, and (soon) pomodoro/focus
records, designed to plug into a self-hosted planning + learning loop.

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
make install          # → $(go env GOPATH)/bin/ttcli
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
Planned: KOReader reading telemetry and a coaching daemon for `ru0`/`nas0`.
