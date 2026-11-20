package main

import (
	"image"
	"image/color"
	"math"

	"github.com/lichdotdev/lich/native/ui"
)

// chatStartLogo is lich's mark in pixel art as the start screen drew it before
// the backdrop: 48x68, row 0 on top, 1 the darkest tone to 5 the lightest,
// . nothing. The start screen now draws it in the backdrop's own dots, at
// half size (chatStartMark).
var chatStartLogo = [...]string{
	"................................................",
	"................................................",
	"................................................",
	"....................55..........................",
	"...................5555.........................",
	"...................5554.........................",
	"..................55555.........................",
	"..................45554.........................",
	".................455555.........................",
	".................555554.........................",
	".................4555554........................",
	".................45555544.......................",
	".................45555554.......................",
	".................455555544......................",
	".................4555555544.....................",
	".................45555554544....................",
	".................454555555544...................",
	".................4445555554444..................",
	"..................4445555554444.................",
	"..................44445555554444................",
	"...................4445555555444................",
	"....................4445555555444...............",
	"....................43445455544443..............",
	".....................3344445544443..............",
	"....55................3344545554433.......55....",
	"...5555................334444554443......5554...",
	"...5555.................334445544433.....5554...",
	"..4555...................33444554443......5544..",
	"..4555.........33.........33444544433.....5543..",
	".33455.......3333333......33344544343.....44433.",
	".34444......334333433......3334444433.....44433.",
	".33444.....33334443333......3344444433....34433.",
	".33444....333344433333......3334444343....33433.",
	"333433....33344433233.......3334444433....334323",
	"333433....334443323..........234444443....334333",
	"233433....33343322...........234444433....233333",
	"334333....3344433............233444443....334332",
	"233333....3334332...........2233443433....233332",
	"2333323...3343332...........2333444333...2233332",
	"2333232...2333332...........2333443323...2333332",
	"2333332...32333322.........2233443333....2333332",
	"2223332....23333222.......22233433332...22233322",
	".2333222...222333222.....222233332322...2233322.",
	".2233222....222332222222222233333222....2233322.",
	".22233222....2223322222222233333322....22233222.",
	".22233222....2222333333333333332222....22233222.",
	"..22233222....22223333333333332222....22233222..",
	"..222332222....222223333333332222....222233222..",
	"...222332222....2222233333322222....222233222...",
	"...2122332221.....1222233222121....2122333221...",
	"....2223322221.....2212222222.....2222333222....",
	"....12222322221......212221......21223232212....",
	".....12222322221.......121......21223232212.....",
	"......112222222111............111222232211......",
	".......112222222212..........122223222211.......",
	"........1112222222111......1112222222111........",
	"..........112222222211....112222222211..........",
	"...........111222222211..112222222111...........",
	".............1112222221..1222222111.............",
	"...............111222211112222111...............",
	".................11222222222211.................",
	"..................111212221111..................",
	"....................11222111....................",
	".....................112111.....................",
	"......................1111......................",
	".......................11.......................",
	"................................................",
	"................................................",
}

const (
	// chatStartMid is how far the logo's middle tone goes from the muted
	// foreground toward the foreground, and chatStartShade and chatStartDeep
	// how far its two dark ones go toward the background.
	chatStartMid   = 0.5
	chatStartShade = 0.35
	chatStartDeep  = 0.65
)

// The mark takes the field's motion as the user approved it in the mockup
// (logo-mix): a dot shows while markKeep + the field's value * markKeepGain
// passes its Bayer threshold, about a third of them out where the bands thin
// and all of them on a band, and the field's value past markFloor lifts it
// toward the foreground by markLift, never past markLiftMost, so its darkest
// tones still read.
const (
	markKeep     = 0.62
	markKeepGain = 1.6
	markFloor    = 0.05
	markLift     = 0.9
	markLiftMost = 0.45
)

// chatStartMark is the logo at half size, a dot per 2x2 of its pixels: a dot
// where at least two of the four are drawn, in the lightest of them, so the
// thin bright strokes survive.
var chatStartMark = func() [][]byte {
	rows := make([][]byte, 0, (len(chatStartLogo)+1)/2)
	for y := 0; y < len(chatStartLogo); y += 2 {
		row := make([]byte, 0, (len(chatStartLogo[y])+1)/2)
		for x := 0; x < len(chatStartLogo[y]); x += 2 {
			row = append(row, lightestOf(chatStartLogo[:], x, y))
		}
		rows = append(rows, row)
	}
	return rows
}()

// lightestOf is the dot for the 2x2 block at x, y of g.
func lightestOf(g []string, x, y int) byte {
	drawn, best := 0, byte('.')
	for _, p := range [...]image.Point{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}} {
		if p.Y >= len(g) || p.X >= len(g[p.Y]) || g[p.Y][p.X] == '.' {
			continue
		}
		drawn++
		if best == '.' || g[p.Y][p.X] > best {
			best = g[p.Y][p.X]
		}
	}
	if drawn < 2 {
		return '.'
	}
	return best
}

// chatStartMarkSize is the mark's size in cells.
var chatStartMarkSize = image.Pt(len(chatStartMark[0]), len(chatStartMark))

// chatStartColor is the color of a logo symbol, from the theme's own tokens
// so it follows a light theme and a dark one; false for nothing.
func chatStartColor(th *ui.Theme, symbol byte) (color.NRGBA, bool) {
	switch symbol {
	case '5':
		return th.Foreground, true
	case '4':
		return ui.MixOKLab(th.MutedForeground, th.Foreground, chatStartMid), true
	case '3':
		return th.MutedForeground, true
	case '2':
		return ui.MixOKLab(th.MutedForeground, th.Background, chatStartShade), true
	case '1':
		return ui.MixOKLab(th.MutedForeground, th.Background, chatStartDeep), true
	}
	return color.NRGBA{}, false
}

// markInks are the mark's tones on a ground, by symbol, and the foreground
// the field lifts them toward.
type markInks struct {
	tone [256]color.NRGBA
	fg   color.NRGBA
}

func newMarkInks(th *ui.Theme, ground color.NRGBA) (m markInks) {
	for _, s := range []byte("12345") {
		c, _ := chatStartColor(th, s)
		m.tone[s] = ui.Over(ground, c)
	}
	m.fg = ui.Over(ground, th.Foreground)
	return m
}

// backdropMark is where the start screen's mark lies in a pane's field, in
// cells, and how far it has faded in: 1 shown, 0 gone.
type backdropMark struct {
	at   image.Point
	fade float32
}

// paintMark draws the mark into the field's first rows rows of img, a pixel a
// cell, over the field: each of its dots in its tone, lit by the field and
// dropped where the field thins, and dark where it drops.
func (f *backdropField) paintMark(img *image.RGBA, rows int, m backdropMark, inks *markInks) {
	for j, line := range chatStartMark {
		y := m.at.Y + j
		if y < 0 || y >= rows {
			continue
		}
		for i, s := range line {
			x := m.at.X + i
			if s == '.' || x < 0 || x >= f.cols {
				continue
			}
			v := f.at(x, y)
			p := img.Pix[y*img.Stride+4*x : y*img.Stride+4*x+4 : y*img.Stride+4*x+4]
			if (markKeep+v*markKeepGain)*float64(m.fade) <= bayer8[y&7][x&7] {
				p[0], p[1], p[2], p[3] = 0, 0, 0, 0
				continue
			}
			c := mixSRGB(inks.tone[s], inks.fg, min(markLiftMost, max(0, v-markFloor)*markLift))
			p[0], p[1], p[2], p[3] = c.R, c.G, c.B, 255
		}
	}
}

// mixSRGB goes from a to b by w in sRGB, rounded, opaque.
func mixSRGB(a, b color.NRGBA, w float64) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*w)) }
	return color.NRGBA{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: 255}
}
