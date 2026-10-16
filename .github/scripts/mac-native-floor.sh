#!/bin/bash
# Fails when a macOS lich-native asks more of a machine than the bundle
# promises: libghostty-vt linked dynamically (nothing ships the dylib beside
# it), or a minimum macOS above the 13.0 of Info.plist and the cask.
#
# usage: mac-native-floor.sh <lich-native>
set -eu

bin="$1"
floor=13.0

if otool -L "$bin" | grep -qi ghostty; then
  echo "::error::lich-native links libghostty-vt dynamically"; otool -L "$bin"; exit 1
fi
minos=$(otool -l "$bin" | awk '/LC_BUILD_VERSION/ { found = 1 } found && $1 == "minos" { print $2; exit }')
test -n "$minos" || { echo "::error::lich-native carries no LC_BUILD_VERSION"; exit 1; }
echo "lich-native asks for macOS $minos"
[ "$(printf '%s\n%s\n' "$minos" "$floor" | sort -V | tail -1)" = "$floor" ] || {
  echo "::error::lich-native asks for macOS $minos, newer than the $floor the bundle promises"; exit 1
}
