#!/bin/bash
# Runs the macOS native window once against a real backend, with no Mac here
# to look at it: a windowless lich under a HOME of its own, one shell session,
# and lich-native on top of it. The window has to come up, connect and draw
# (-shot, the frame it renders), still be alive afterwards and close when lich
# quits; the shot and a screen capture of the real window land in
# $RUNNER_TEMP/diag for the workflow to upload.
#
# usage: mac-native-smoke.sh <lich> <lich-native>
set -eu

lich="$1"
native="$2"
export HOME="$RUNNER_TEMP/home" LICH_LISTEN_PORT=47898
config="$HOME/Library/Application Support/lich"
runtime="$config/runtime.json"
diag="$RUNNER_TEMP/diag"
proj="$RUNNER_TEMP/proj"
mkdir -p "$HOME" "$diag" "$proj"

fail() {
  echo "::error::$1"
  cat "$diag/native.out" 2>/dev/null || true
  screencapture -x "$diag/screen.png" || true
  cp "$config/lich.log" "$config/lich-native.log" "$diag/" 2>/dev/null || true
  cp /Users/runner/Library/Logs/DiagnosticReports/lich-native* "$diag/" 2>/dev/null || true
  cat "$config/lich.log" "$config/lich-native.log" 2>/dev/null || true
  exit 1
}

# Stands in for the web window: holds until lich exits and closes its stdin.
printf '#!/bin/sh\ncat > /dev/null\n' > "$RUNNER_TEMP/no-window"
chmod +x "$RUNNER_TEMP/no-window"

LICH_SHELL="$RUNNER_TEMP/no-window" "$lich" > "$diag/backend.out" 2>&1 &
for _ in $(seq 1 30); do
  sleep 1
  test -s "$runtime" && break
done
test -s "$runtime" || fail "lich never wrote runtime.json"
"$lich" open --project "$proj" --kind shell || fail "lich open exited $?"

"$native" -runtime "$runtime" -shot "$diag/shot.png" -shot-after 8s > "$diag/native.out" 2>&1 &
pid=$!
for _ in $(seq 1 40); do
  sleep 1
  test -s "$diag/shot.png" && break
  kill -0 "$pid" 2>/dev/null || fail "lich-native exited before it drew"
done
test -s "$diag/shot.png" || fail "lich-native never wrote its shot"
sleep 2
kill -0 "$pid" 2>/dev/null || fail "lich-native died after its shot"
screencapture -x "$diag/screen.png" || true
cp "$config/lich-native.log" "$diag/" 2>/dev/null || true

"$lich" quit || fail "lich quit exited $?"
for _ in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
if kill -0 "$pid" 2>/dev/null; then
  # SIGQUIT makes Go print every goroutine's stack into native.out.
  kill -QUIT "$pid"
  sleep 2
  fail "lich-native outlived lich quit"
fi
echo "lich-native came up, connected, drew and closed with lich; see the uploaded shot"
