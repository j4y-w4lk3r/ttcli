#!/usr/bin/env bash
# TUI regression tests + optional frame dump / kitty smoke.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== go test: TUI frame integrity =="
go test ./internal/tui/ -count=1 "$@"

echo ""
echo "== ttcli debug frame: 120x40 tasks (layout check) =="
go run ./cmd/ttcli/ debug frame --width 120 --height 40 --view tasks --check >/dev/null

echo ""
echo "== ttcli debug frame: view switch sequence =="
for v in tasks calendar tasks; do
  go run ./cmd/ttcli/ debug frame --width 160 --height 45 --view "$v" --check >/dev/null
done

if [[ "${DUMP_FRAME:-}" == "1" ]]; then
  echo ""
  echo "== frame dump 160x45 =="
  go run ./cmd/ttcli/ debug frame --width 160 --height 45 --view tasks --dump 2>&1 | head -20
fi

if command -v kitty >/dev/null 2>&1 && [[ "${KITTY_SMOKE:-}" == "1" ]]; then
  echo ""
  echo "== kitty: termsize in 120x40 window =="
  kitty --override "initial_window_width=120c" --override "initial_window_height=40c" \
    bash -lc 'go run ./cmd/ttcli/ debug termsize 2>/dev/null || true; go run ./cmd/ttcli/ debug frame --width 120 --height 40 --check >/dev/null && echo kitty smoke ok'
fi

echo ""
echo "all TUI checks passed"
