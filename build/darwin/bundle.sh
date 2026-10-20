#!/bin/sh
# Wraps an already-built darwin binary in Lich.app — the bundle is what puts
# lich in /Applications, Launchpad and Spotlight and gives it its icon; a bare
# binary on PATH appears in none of them.
#
#   build/darwin/bundle.sh bin/lich-darwin-arm64 0.30.0 bin bin/lich-native
#
# The native window goes beside lich in Contents/MacOS, where macOS counts a
# process as the app's own (its Dock tile, its icon, its name in the menu
# bar): it is the process holding the window.
#
# macOS host only: sips and iconutil are Apple's, so the icon is generated at
# build time from build/appicon-mac.png instead of committing an .icns — one
# source of the mark for every platform. Cross-compiling the binary still works
# anywhere (task build:mac); only this wrapping step needs a Mac.
set -eu

usage="usage: bundle.sh <binary> <version> <outdir> <lich-native>"
bin="${1:?$usage}"
version="${2:?$usage}"
outdir="${3:?$usage}"
native="${4:?$usage}"

root="$(cd "$(dirname "$0")/../.." && pwd)"
app="$outdir/Lich.app"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin" "$app/Contents/MacOS/lich"
chmod +x "$app/Contents/MacOS/lich"
cp "$native" "$app/Contents/MacOS/lich-native"
chmod +x "$app/Contents/MacOS/lich-native"

sed "s/@VERSION@/${version}/g" "$root/build/darwin/Info.plist.tpl" \
  > "$app/Contents/Info.plist"

iconset="$(mktemp -d)/lich.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$root/build/appicon-mac.png" \
    --out "$iconset/icon_${size}x${size}.png" >/dev/null
  sips -z "$((size * 2))" "$((size * 2))" "$root/build/appicon-mac.png" \
    --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/lich.icns"
rm -rf "$(dirname "$iconset")"

# Ad-hoc signature: arm64 refuses to run an executable carrying none, and a
# bundle whose seal does not cover Info.plist and the icon reads as damaged.
# Innermost first, as codesign wants it. This is not notarization —
# Gatekeeper still calls the developer unidentified.
codesign --force --sign - "$app/Contents/MacOS/lich-native"
codesign --force --sign - "$app"

echo "built $app"
