#!/bin/bash
# Runs a freshly bundled Lich.app end to end on the macOS runner, the Windows
# job's check on the bundle: the seal verifies, lich finds the window beside
# itself in Contents/MacOS and opens it, macOS counts exactly one application
# in Lich.app (lsappinfo, which is what the Dock reads: a second entry is a
# process with a Dock tile of its own), the window closes on SIGTERM, a close
# request like the user's, lich keeps running without it, a launch opens a new
# window on it, and `lich quit` ends both. Its config lives under a HOME of its
# own.
# What a failure leaves behind goes to $RUNNER_TEMP/diag for the artifact the
# workflow uploads.
#
# usage: mac-e2e.sh <path to Lich.app>
set -eu

app="$(cd "$1" && pwd)"
export HOME="$RUNNER_TEMP/home" LICH_LISTEN_PORT=47899
config="$HOME/Library/Application Support/lich"
log="$config/lich.log"
diag="$RUNNER_TEMP/diag"
mkdir -p "$HOME" "$diag"

dump() {
  ps -eo pid,ppid,args | grep -i '[l]ich' > "$diag/ps.txt" || true
  lsappinfo list > "$diag/apps.txt" || true
  screencapture -x "$diag/screen.png" || true
  cp "$log" "$config/lich-native.log" "$diag/" 2>/dev/null || true
  # A process that died leaves its report with the system, keyed by the user,
  # not by HOME.
  cp /Users/runner/Library/Logs/DiagnosticReports/[lL]ich* "$diag/" 2>/dev/null || true
  cat "$diag/ps.txt"
}

fail() {
  echo "::error::$1"
  dump
  cat "$log" "$config/lich-native.log" 2>/dev/null || true
  exit 1
}

# One application in Lich.app however many of its processes run.
one_app() {
  lsappinfo list > "$RUNNER_TEMP/apps.txt"
  apps=$(grep -c 'bundle path=.*Lich.app' "$RUNNER_TEMP/apps.txt" || true)
  [ "$apps" = 1 ] || { grep -B1 -A3 'Lich.app' "$RUNNER_TEMP/apps.txt"; fail "macOS counts $apps applications in Lich.app, expected the window alone"; }
}

codesign --verify --deep --strict --verbose=2 "$app"
test -f "$app/Contents/MacOS/lich-native"

"$app/Contents/MacOS/lich" &
for _ in $(seq 1 30); do
  sleep 1
  test -s "$config/runtime.json" && break
done
test -s "$config/runtime.json" || fail "lich never wrote runtime.json"

for _ in $(seq 1 60); do sleep 1; grep -q 'native window attached' "$log" && break; done
grep -q 'native window attached' "$log" || fail "lich did not open its window"
sleep 10
window=$(pgrep -x lich-native || true)
test -n "$window" || fail "the window died"
screencapture -x "$diag/window.png" || true
one_app

# Closing the window leaves lich running with its sessions; with none running
# it would quit, so one is opened first, through the CLI as an agent would.
mkdir -p "$RUNNER_TEMP/proj"
"$app/Contents/MacOS/lich" open --project "$RUNNER_TEMP/proj" --kind shell || fail "lich open exited $?"
kill -TERM "$window"
for _ in $(seq 1 15); do sleep 1; pgrep -x lich-native > /dev/null || break; done
pgrep -x lich-native > /dev/null && fail "the window outlived its SIGTERM"
sleep 2
pgrep -x lich > /dev/null || fail "lich exited with its window"

# Launched again, lich asks the running one for its window, and that backend
# opens it.
"$app/Contents/MacOS/lich" || fail "the reopening launch exited $?"
for _ in $(seq 1 30); do sleep 1; pgrep -x lich-native > /dev/null && break; done
pgrep -x lich-native > /dev/null || fail "a second launch did not reopen the window on the running lich"
sleep 5
pgrep -x lich-native > /dev/null || fail "the reopened window died"
one_app
cp "$config/lich-native.log" "$diag/" 2>/dev/null || true

"$app/Contents/MacOS/lich" quit || fail "lich quit exited $?"
for _ in $(seq 1 10); do pgrep -x lich > /dev/null || break; sleep 1; done
pgrep -x lich > /dev/null && fail "lich is still running after lich quit"
for _ in $(seq 1 10); do pgrep -x lich-native > /dev/null || break; sleep 1; done
pgrep -x lich-native > /dev/null && fail "the window outlived lich quit"
grep 'Lich.app' "$RUNNER_TEMP/apps.txt"
