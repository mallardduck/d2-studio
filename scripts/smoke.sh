#!/usr/bin/env bash
# Starts the built app for a few seconds and checks that it really launched:
# it must stay running and log its startup line. Used by CI on each platform
# (on Linux under xvfb-run); it can be run locally too.
#
#   scripts/smoke.sh <path-to-binary> [seconds]
set -euo pipefail

bin=${1:?usage: smoke.sh <path-to-binary> [seconds]}
secs=${2:-10}
log=$(mktemp)
trap 'rm -f "$log"' EXIT

D2STUDIO_LOG=info "$bin" >"$log" 2>&1 &
pid=$!

stop() {
  kill "$pid" 2>/dev/null || true
  sleep 1
  if kill -0 "$pid" 2>/dev/null; then
    kill -KILL "$pid" 2>/dev/null || true
    # A native Windows process started from bash may not respond to kill.
    command -v taskkill >/dev/null 2>&1 && taskkill //F //PID "$pid" >/dev/null 2>&1 || true
  fi
  wait "$pid" 2>/dev/null || true
}

sleep "$secs"
if ! kill -0 "$pid" 2>/dev/null; then
  echo "smoke: FAIL, the app exited within ${secs}s" >&2
  cat "$log" >&2
  exit 1
fi
stop

if ! grep -q "D2 Studio starting" "$log"; then
  echo "smoke: FAIL, the app ran but never logged its startup line" >&2
  cat "$log" >&2
  exit 1
fi
echo "smoke: ok"
grep "D2 Studio starting" "$log"
