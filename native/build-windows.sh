#!/bin/sh
# Cross-builds lich-native.exe and lich-browser.exe for Windows (amd64) from
# Linux, with Zig as the
# C toolchain: Zig builds libghostty-vt for Windows at the commit in
# GHOSTTY_COMMIT, `zig cc` builds libjpeg-turbo (libjpeg-turbo.sh, which needs
# cmake and nasm), and `zig cc` is cgo's compiler and linker.
#
#   ./build-windows.sh [out.exe]     default cmd/lich-native/lich-native.exe
#
# lich-native.exe needs nothing beside it: libghostty-vt and libjpeg-turbo are
# linked in, and the rest is Windows' own (KERNEL32, the UCRT). `lich native`
# looks for lich-native beside lich, or at LICH_NATIVE. lich-browser.exe lands
# beside it with the WebView2Loader.dll it loads, taken from Microsoft's
# WebView2 NuGet package at the version third_party/webview2 pins.
#
# Env: LIBGHOSTTY_VT_WINDOWS  where the Windows libghostty-vt is built and kept
#                             (default ~/.cache/lich-native-slice/libghostty-vt-windows)
#      LIBJPEG_TURBO_WINDOWS  the same for the Windows libjpeg-turbo
#                             (default ~/.cache/lich-native-slice/libjpeg-turbo-windows)
set -eu
cd "$(dirname "$0")"
OUT=${1:-cmd/lich-native/lich-native.exe}
COMMIT=$(cat GHOSTTY_COMMIT)
PREFIX=${LIBGHOSTTY_VT_WINDOWS:-$HOME/.cache/lich-native-slice/libghostty-vt-windows}
JPEG=${LIBJPEG_TURBO_WINDOWS:-$HOME/.cache/lich-native-slice/libjpeg-turbo-windows}
WEBVIEW2=${WEBVIEW2_WINDOWS:-$HOME/.cache/lich-native-slice/webview2}
# The version third_party/webview2/README.md names, and its package's SHA-256.
WEBVIEW2_VERSION=1.0.4370.26
WEBVIEW2_SHA256=1c0dacac84b70109b3fd42f90c53a0ba942abc3a87590d7a4e0ccd86e4a06cf4
TARGET=x86_64-windows-gnu

if [ "$(cat "$PREFIX/.commit" 2>/dev/null)" != "$COMMIT" ]; then
  src=$(mktemp -d)
  trap 'rm -rf "$src"' EXIT
  git -C "$src" init -q
  git -C "$src" fetch -q --depth 1 https://github.com/ghostty-org/ghostty "$COMMIT"
  git -C "$src" checkout -q FETCH_HEAD
  (cd "$src" && zig build -Demit-lib-vt -Doptimize=ReleaseFast -Dtarget=$TARGET --prefix "$PREFIX")
  echo "$COMMIT" > "$PREFIX/.commit"
fi

CC="zig cc -target $TARGET" ./libjpeg-turbo.sh "$JPEG" -DCMAKE_SYSTEM_NAME=Windows -DCMAKE_SYSTEM_PROCESSOR=x86_64

# pkg-config names the static library by its path, a .lib that cgo's flag
# allowlist rejects unless told otherwise. The GUI subsystem keeps a console
# window from opening beside the app; under external linking -H=windowsgui
# alone does not reach zig's linker, hence the extldflags.
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC="zig cc -target $TARGET" CXX="zig c++ -target $TARGET" \
  CGO_LDFLAGS_ALLOW='.*\.lib' PKG_CONFIG_PATH="$JPEG/lib/pkgconfig:$PREFIX/share/pkgconfig" \
  go build -ldflags='-H=windowsgui -extldflags=-Wl,--subsystem,windows' -o "$OUT" ./cmd/lich-native

# lich-browser.exe is GUI-subsystem too, so the backend starting it opens no
# console; its wire is the pipes it inherits.
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC="zig cc -target $TARGET" CXX="zig c++ -target $TARGET" \
  go build -ldflags='-H=windowsgui -extldflags=-Wl,--subsystem,windows' -o "$(dirname "$OUT")/lich-browser.exe" ./cmd/lich-browser

pkg="$WEBVIEW2/microsoft.web.webview2.$WEBVIEW2_VERSION.nupkg"
if ! echo "$WEBVIEW2_SHA256  $pkg" | sha256sum -c --status 2>/dev/null; then
  mkdir -p "$WEBVIEW2"
  curl -fsSL -o "$pkg" "https://api.nuget.org/v3-flatcontainer/microsoft.web.webview2/$WEBVIEW2_VERSION/microsoft.web.webview2.$WEBVIEW2_VERSION.nupkg"
  echo "$WEBVIEW2_SHA256  $pkg" | sha256sum -c --status || { echo "WebView2 package: SHA-256 mismatch" >&2; exit 1; }
fi
unzip -p "$pkg" build/native/x64/WebView2Loader.dll > "$(dirname "$OUT")/WebView2Loader.dll"
echo "$OUT"
