package grimoire

import (
	"math"
	"time"
)

// The default provider's book carries a small flame where its top gem was:
// FlameStep a frame while it burns, ChooseStep while it lights up and
// SnuffStep while it goes out when another book is chosen.
const (
	FlameStep  = 110 * time.Millisecond
	ChooseStep = 80 * time.Millisecond
	SnuffStep  = 70 * time.Millisecond
)

// wick is the flame's six frames, 5x7 with the foot on the gem's row: 4 the
// core, 2 the body, 3 the edge.
var wick = [...][7]string{
	{"..3..", ".....", "..3..", ".323.", ".242.", "32423", ".232."},
	{".....", "..3..", ".33..", ".323.", "3242.", "32423", ".232."},
	{".3...", ".....", "..33.", ".323.", ".2423", "32423", ".232."},
	{".....", "...3.", "..3..", ".323.", "3242.", "32423", ".232."},
	{"...3.", ".....", ".33..", ".323.", ".2423", "32423", ".232."},
	{".....", ".3...", "..33.", ".323.", ".242.", "32423", ".232."},
}

// The top gem, a 3x3 square, and the flame's place over it.
const (
	gemFrom, gemTo   = 16, 18
	gemTop, gemFoot  = 7, 9
	wickX, wickY     = 15, 4
	wickRows         = 7
	sparkEvery       = 20
	sparkRise        = 4
	chooseWaveSteps  = 8
	chooseWaveReach  = 5.2
	chooseWaveOrigin = 4.0
)

// flameRows is the ready book with the flame at frame f, its top rows of
// wickRows grown: a spark leaves the whole flame now and then, rises and goes
// out.
func flameRows(id string, f, rows int) Grid {
	g := Sprite(id, Ready)
	for y := gemTop; y <= gemFoot; y++ {
		for x := gemFrom; x <= gemTo; x++ {
			g[y][x] = '1'
		}
	}
	for y, row := range wick[f%len(wick)] {
		if y < wickRows-rows {
			continue
		}
		for x := range len(row) {
			if row[x] != '.' {
				g[wickY+y][wickX+x] = row[x]
			}
		}
	}
	if rows == wickRows {
		if a := f % sparkEvery; a < sparkRise {
			col := gemFrom + f/sparkEvery%3
			g[3-a][col] = '5'
			if a < 2 {
				g[3-a][col] = '3'
			}
		}
	}
	return g
}

// Flame is the default provider's book at frame f of its flame.
func Flame(id string, f int) Grid { return flameRows(id, f, wickRows) }

// Choose is a book made the default, from frame f of the flame: the gem
// flashes, the flame grows, and a wave of light runs down the cover from it a
// tone above each pixel.
func Choose(id string, f int) []Grid {
	flash := Sprite(id, Ready)
	for y := gemTop; y <= gemFoot; y++ {
		for x := gemFrom; x <= gemTo; x++ {
			flash[y][x] = '4'
		}
	}
	out := []Grid{flash}
	for _, rows := range [...]int{2, 3, 5, 7} {
		out = append(out, flameRows(id, f, rows))
	}
	for i := 1; i <= chooseWaveSteps; i++ {
		r := chooseWaveOrigin + float64(i)*chooseWaveReach
		g := Flame(id, f+i)
		for y := range g {
			for x, c := range g[y] {
				d := math.Hypot(float64(x-medalX), float64(y-8))
				if c != '.' && d > chooseWaveOrigin && d < r && d >= r-chooseWaveOrigin {
					g[y][x] = shift(up, c)
				}
			}
		}
		out = append(out, g)
	}
	return out
}

// Snuff is the old default's flame drawing back into its gem, from frame f,
// ending on the ready book.
func Snuff(id string, f int) []Grid {
	var out []Grid
	for _, rows := range [...]int{5, 3, 2} {
		out = append(out, flameRows(id, f, rows))
	}
	return append(out, Sprite(id, Ready))
}

// Hover is a ready book under the pointer: its top gem lit, a book that can be
// made the default.
func Hover(id string) Grid {
	g := Sprite(id, Ready)
	for y := gemTop; y <= gemFoot; y++ {
		for x := gemFrom; x <= gemTo; x++ {
			if g[y][x] != '1' {
				g[y][x] = '4'
			}
		}
	}
	return g
}
