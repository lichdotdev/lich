#!/usr/bin/env bash
# Renders the devicons frontend/src/components/FileIcon.tsx imports into the
# PNGs beside this script, from devicons-react's own markup so both windows
# draw the same marks. Needs frontend/node_modules (pnpm install) and
# rsvg-convert (librsvg).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
frontend=${FRONTEND:-$here/../../../../frontend}
# 64px is four times the tree's 14dp glyph plus room: the draw scales down,
# never up, at any display scale up to 4x.
size=64
icons=(Bash Css3 Go Html5 Javascript Json Kotlin Markdown Python Rust Typescript Xml Yaml)
for name in "${icons[@]}"; do
	(cd "$frontend" && node -e '
		const React = require("react")
		const { renderToStaticMarkup } = require("react-dom/server")
		const Icon = require(`devicons-react/icons/${process.argv[1]}Original`)
		process.stdout.write(renderToStaticMarkup(React.createElement(Icon, { size: 128 })))
	' "$name") | rsvg-convert -w "$size" -h "$size" -o "$here/${name,,}.png"
done
