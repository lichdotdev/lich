#!/bin/bash
# Fails when a macOS lich-native or lich-browser asks more of a machine than
# the bundle promises: libghostty-vt or libjpeg-turbo linked dynamically
# (nothing ships a dylib beside it), or a minimum macOS above the 13.0 of
# Info.plist and the cask.
#
# usage: mac-native-floor.sh <lich-native or lich-browser>
set -eu

bin="$1"
name="$(basename "$bin")"
floor=13.0

if otool -L "$bin" | grep -qiE 'ghostty|turbojpeg'; then
  echo "::error::$name links libghostty-vt or libjpeg-turbo dynamically"; otool -L "$bin"; exit 1
fi
minos=$(otool -l "$bin" | awk '/LC_BUILD_VERSION/ { found = 1 } found && $1 == "minos" { print $2; exit }')
test -n "$minos" || { echo "::error::$name carries no LC_BUILD_VERSION"; exit 1; }
echo "$name asks for macOS $minos"
[ "$(printf '%s\n%s\n' "$minos" "$floor" | sort -V | tail -1)" = "$floor" ] || {
  echo "::error::$name asks for macOS $minos, newer than the $floor the bundle promises"; exit 1
}
