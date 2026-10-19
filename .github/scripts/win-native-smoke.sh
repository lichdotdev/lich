#!/bin/bash
# Runs the Windows native window once against a real backend, the check
# mac-native-smoke.sh is for macOS: a windowless lich, one shell session, and
# lich-native on top of it. The window has to come up, connect and draw (-shot,
# the frame it renders), still be alive afterwards and close when lich quits;
# the shot and the logs land in $RUNNER_TEMP/diag for the workflow to upload.
# Runs in git-bash.
#
# usage: win-native-smoke.sh <lich.exe> <lich-native.exe>
set -eu

lich="$1"
native="$2"
export LICH_LISTEN_PORT=47898
config="$(cygpath -u "$APPDATA")/lich"
runtime="$config/runtime.json"
diag="$RUNNER_TEMP/diag"
proj="$RUNNER_TEMP/proj"
mkdir -p "$diag" "$proj"

fail() {
  echo "::error::$1"
  cat "$diag/native.out" 2>/dev/null || true
  cp "$config/lich.log" "$config/lich-native.log" "$diag/" 2>/dev/null || true
  cat "$config/lich.log" "$config/lich-native.log" 2>/dev/null || true
  exit 1
}

"$lich" --no-window > "$diag/backend.out" 2>&1 &
for _ in $(seq 1 30); do
  sleep 1
  test -s "$runtime" && break
done
test -s "$runtime" || fail "lich never wrote runtime.json"
"$lich" open --project "$(cygpath -w "$proj")" --kind shell || fail "lich open exited $?"

"$native" -runtime "$(cygpath -w "$runtime")" -shot "$(cygpath -w "$diag/shot.png")" -shot-after 8s > "$diag/native.out" 2>&1 &
pid=$!
for _ in $(seq 1 40); do
  sleep 1
  test -s "$diag/shot.png" && break
  kill -0 "$pid" 2>/dev/null || fail "lich-native exited before it drew"
done
test -s "$diag/shot.png" || fail "lich-native never wrote its shot"
sleep 2
kill -0 "$pid" 2>/dev/null || fail "lich-native died after its shot"
cp "$config/lich-native.log" "$diag/" 2>/dev/null || true

"$lich" quit || fail "lich quit exited $?"
for _ in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
kill -0 "$pid" 2>/dev/null && fail "lich-native outlived lich quit"
echo "lich-native came up, connected, drew and closed with lich; see the uploaded shot"
