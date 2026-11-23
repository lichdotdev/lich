#!/usr/bin/env bash
# Writes material.json, the file entries of Material Icon Theme's manifest
# (fileNames, fileExtensions and their light overrides, keys lowercased as VS
# Code matches them), renders every icon it names into the PNGs beside this
# script and copies the theme's LICENSE. The package is installed into a
# temporary directory at the version the files were made from. Needs npm, jq
# and rsvg-convert (librsvg).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
pkgs=$(mktemp -d)
trap 'rm -rf "$pkgs"' EXIT
(cd "$pkgs" && npm install --silent --no-save --no-package-lock material-icon-theme@5.40.0)
pkg=$pkgs/node_modules/material-icon-theme
manifest=$pkg/dist/material-icons.json
cp "$pkg/LICENSE" "$here/LICENSE"
jq -S 'def lower: with_entries(.key |= ascii_downcase);
	{fileNames: (.fileNames | lower), fileExtensions: (.fileExtensions | lower),
	 light: {fileNames: (.light.fileNames | lower), fileExtensions: (.light.fileExtensions | lower)}}' \
	"$manifest" > "$here/material.json"
# 64px is four times a 16dp glyph: the draw scales down, never up, at any
# display scale up to 4x.
size=64
rm -f "$here"/*.png
jq -r --slurpfile m "$manifest" \
	'[.fileNames[], .fileExtensions[], .light.fileNames[], .light.fileExtensions[]] | unique | .[]
	 | "\(.)\t\($m[0].iconDefinitions[.].iconPath)"' "$here/material.json" |
	while IFS=$'\t' read -r name svg; do
		rsvg-convert -w "$size" -h "$size" -o "$here/$name.png" "$pkg/dist/$svg"
	done
