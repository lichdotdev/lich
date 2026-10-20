#!/usr/bin/env bash
# Renders every icon devicons.json names into the PNGs beside this script,
# from devicons-react's components (lucide-react for a name it lacks). The
# packages are installed into a temporary directory at the versions the PNGs
# were drawn from. Needs node, npm and rsvg-convert (librsvg).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
pkgs=$(mktemp -d)
trap 'rm -rf "$pkgs"' EXIT
(cd "$pkgs" && npm install --silent --no-save --no-package-lock \
	react@18.3.1 react-dom@18.3.1 devicons-react@1.5.0 lucide-react@1.23.0)
# 64px is four times the tree's 14dp glyph plus room: the draw scales down,
# never up, at any display scale up to 4x.
size=64
rm -f "$here"/*.png
for name in $(jq -r '[.extensions[], .names[]] | unique | .[]' "$here/devicons.json"); do
	(cd "$pkgs" && node -e '
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
