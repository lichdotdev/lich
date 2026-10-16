#!/usr/bin/env bash
# Renders every icon devicons.json names into the PNGs beside this script,
# from the same React components frontend/src/components/FileIcon.tsx draws
# (devicons-react, or lucide-react for a name it lacks), so both windows show
# the same marks. Needs frontend/node_modules (pnpm install) and rsvg-convert
# (librsvg).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
frontend=${FRONTEND:-$here/../../../../frontend}
# 64px is four times the tree's 14dp glyph plus room: the draw scales down,
# never up, at any display scale up to 4x.
size=64
rm -f "$here"/*.png
for name in $(jq -r '[.extensions[], .names[]] | unique | .[]' "$here/devicons.json"); do
	(cd "$frontend" && node -e '
		const React = require("react")
		const { renderToStaticMarkup } = require("react-dom/server")
		const name = process.argv[1]
		let Icon
		try {
			Icon = require(`devicons-react/icons/${name}`)
		} catch {
			Icon = require("lucide-react")[name]
		}
		if (!Icon) throw new Error(`no icon ${name} in devicons-react or lucide-react`)
		process.stdout.write(renderToStaticMarkup(React.createElement(Icon, { size: 128 })))
	' "$name") | rsvg-convert -w "$size" -h "$size" -o "$here/$name.png"
done
