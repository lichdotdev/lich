#!/bin/sh
# Builds a static libturbojpeg into a prefix, for a lich-native that needs no
# libjpeg-turbo beside it: CI and the release builds. A window built on a
# developer's machine links the distribution's own libturbojpeg and needs none
# of this.
#
#   ./libjpeg-turbo.sh <prefix> [cmake options]
#
# Then put <prefix>/lib/pkgconfig first on PKG_CONFIG_PATH. Needs cmake, and
# nasm on x86-64: REQUIRE_SIMD fails the build rather than ship a decoder
# without the SIMD code that makes it worth linking.
set -eu
PREFIX=$1
shift
VERSION=3.2.1
COMMIT=7b4993786d856cbccabfc0ed2f923ba000e326cc

[ "$(cat "$PREFIX/.commit" 2>/dev/null)" != "$COMMIT" ] || exit 0
src=$(mktemp -d)
trap 'rm -rf "$src"' EXIT
git -C "$src" init -q
git -C "$src" fetch -q --depth 1 https://github.com/libjpeg-turbo/libjpeg-turbo "$COMMIT"
git -C "$src" checkout -q FETCH_HEAD
# Nothing is installed, but cross-building for Windows the default prefix is
# a C: path that cmake refuses on this host.
cmake -S "$src" -B "$src/build" -DCMAKE_BUILD_TYPE=Release -DENABLE_SHARED=0 -DREQUIRE_SIMD=1 \
  -DCMAKE_INSTALL_PREFIX="$PREFIX" "$@"
cmake --build "$src/build" --target turbojpeg-static --parallel 2

mkdir -p "$PREFIX/lib/pkgconfig" "$PREFIX/include"
cp "$src/build/libturbojpeg.a" "$PREFIX/lib/"
cp "$src/src/turbojpeg.h" "$PREFIX/include/"
# The library's own .pc is written by an install, which also builds its tools.
# This prefix holds no shared library, so -lturbojpeg can only be the archive.
cat > "$PREFIX/lib/pkgconfig/libturbojpeg.pc" <<PC
Name: libturbojpeg
Description: libjpeg-turbo's TurboJPEG API, static
Version: $VERSION
Libs: -L$PREFIX/lib -lturbojpeg
Cflags: -I$PREFIX/include
PC
echo "$COMMIT" > "$PREFIX/.commit"
