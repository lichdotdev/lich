package main

import (
	"image/color"
	"path"
	"strings"

	"github.com/omartelo/lich/native/ui"
)

// tailwindHue is a Tailwind color's 400, 500 and 600 shades, the three a
// language badge reads (bg-X-500/15 text-X-600 dark:text-X-400).
type tailwindHue struct{ c400, c500, c600 color.NRGBA }

func hue(l4, c4, h4, l5, c5, h5, l6, c6, h6 float64) tailwindHue {
	return tailwindHue{ui.OKLCH(l4, c4, h4, 1), ui.OKLCH(l5, c5, h5, 1), ui.OKLCH(l6, c6, h6, 1)}
}

// Tailwind v4's palette, the shades lang-badge.ts names.
var (
	twBlue    = hue(0.707, 0.165, 254.624, 0.623, 0.214, 259.815, 0.546, 0.245, 262.881)
	twYellow  = hue(0.852, 0.199, 91.936, 0.795, 0.184, 86.047, 0.681, 0.162, 75.834)
	twCyan    = hue(0.789, 0.154, 211.53, 0.715, 0.143, 215.221, 0.609, 0.126, 221.723)
	twSky     = hue(0.746, 0.16, 232.661, 0.685, 0.169, 237.323, 0.588, 0.158, 241.966)
	twOrange  = hue(0.75, 0.183, 55.934, 0.705, 0.213, 47.604, 0.646, 0.222, 41.116)
	twAmber   = hue(0.828, 0.189, 84.429, 0.769, 0.188, 70.08, 0.666, 0.179, 58.318)
	twViolet  = hue(0.702, 0.183, 293.541, 0.606, 0.25, 292.717, 0.541, 0.281, 293.009)
	twRose    = hue(0.712, 0.194, 13.428, 0.645, 0.246, 16.439, 0.586, 0.253, 17.585)
	twEmerald = hue(0.765, 0.177, 163.223, 0.696, 0.17, 162.48, 0.596, 0.145, 163.225)
	twTeal    = hue(0.777, 0.152, 181.912, 0.704, 0.14, 182.503, 0.6, 0.118, 184.704)
	twIndigo  = hue(0.673, 0.182, 276.935, 0.585, 0.233, 277.117, 0.511, 0.262, 276.966)
	twStone   = hue(0.709, 0.01, 56.259, 0.553, 0.013, 58.071, 0.444, 0.011, 73.639)
)

type langBadge struct {
	abbr string
	tint *tailwindHue // nil: bg-muted text-muted-foreground
}

// langBadges is lib/git/lang-badge.ts's BADGES.
var langBadges = map[string]langBadge{
	"ts": {"TS", &twBlue}, "tsx": {"TS", &twBlue},
	"js": {"JS", &twYellow}, "jsx": {"JS", &twYellow},
	"go": {"GO", &twCyan}, "css": {"CSS", &twSky}, "html": {"HTM", &twOrange},
	"json": {"{}", &twAmber}, "md": {"MD", &twViolet},
	"yaml": {"YML", &twRose}, "yml": {"YML", &twRose},
	"sh": {"SH", &twEmerald}, "py": {"PY", &twTeal}, "rs": {"RS", &twOrange},
	"sql": {"SQL", &twIndigo}, "toml": {"TML", &twStone},
}

// extname is a path's lowercased extension without the dot, "" for none or
// a dotfile.
func extname(p string) string {
	base := path.Base(p)
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(base[dot+1:])
}

// languageAbbr picks a path's badge: a known extension's, else its first
// letters, else a neutral dot.
func languageAbbr(p string) langBadge {
	ext := extname(p)
	if b, ok := langBadges[ext]; ok {
		return b
	}
	if ext == "" {
		return langBadge{abbr: "•"}
	}
	return langBadge{abbr: strings.ToUpper(ext[:min(len(ext), 3)])}
}

// badgeTintAlpha is bg-X-500/15.
const badgeTintAlpha = 0.15

// colors are the badge's fill and text on th.
func (b langBadge) colors(th *ui.Theme, light bool) (bg, fg color.NRGBA) {
	if b.tint == nil {
		return th.Over(th.Muted), th.MutedForeground
	}
	fg = b.tint.c400
	if light {
		fg = b.tint.c600
	}
	return th.Over(ui.Alpha(b.tint.c500, badgeTintAlpha)), fg
}
