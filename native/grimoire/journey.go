package grimoire

import "slices"

import "math"

// dimBook is the book a tone down with its mark unlit, the ribbon gone and the
// clasps' studs dark: the book asleep, before the straps go on or after they
// come off.
func dimBook(id string) Grid {
	g := Sprite(id, Ready)
	for y := range g {
		for x := range g[y] {
			switch {
			case dist(x, y, medalX, medalY) < 7.5:
				if g[y][x] == '4' {
					g[y][x] = '3'
				}
			case y >= 44:
				g[y][x] = '.'
			case (y == 42 || y == 43) && (x == 21 || x == 22):
				g[y][x] = '5'
				if y == 42 && x%3 != 0 {
					g[y][x] = '3'
				}
			case abs(x-17) <= 1 && (abs(y-8) <= 1 || abs(y-34) <= 1):
				g[y][x] = '1'
			default:
				g[y][x] = shift(down, g[y][x])
			}
		}
	}
	return g
}

// strapStart is where a strap starts at each phase: closed and with its pin
// out it crosses the cover, then its end draws back into the buckle.
var strapStart = [...]int{12, 12, 18, 24, 29}

// strap draws the strap across row y at phase ph: 0 buckled, 1 the pin out, 2
// and 3 the end drawing back, 4 the buckle slipping off, 5 and on gone.
func strap(g Grid, y, ph int) {
	if ph >= 5 {
		return
	}
	buckle := 24
	if ph == 4 {
		buckle = 30
	}
	for x := strapStart[ph]; x < Cols; x++ {
		g[y-1][x], g[y][x], g[y+1][x] = '2', '3', '5'
	}
	for yy := y - 2; yy <= y+2; yy++ {
		for x := buckle; x <= min(Cols-1, buckle+4); x++ {
			switch {
			case x == buckle || x == buckle+4 || yy == y-2 || yy == y+2 || ph == 1:
				g[yy][x] = '4'
			default:
				g[yy][x] = '1'
			}
		}
	}
	if ph == 0 {
		g[y][buckle+2], g[y][buckle+3] = '4', '4'
	}
}

// strapFrames open both clasps, the top one first: from buckled to gone.
func strapFrames(id string) []Grid {
	dim := dimBook(id)
	out := make([]Grid, 0, 9)
	for t := 0; t <= 8; t++ {
		g := dim.clone()
		strap(g, 8, min(5, t))
		strap(g, 34, max(0, min(5, t-3)))
		out = append(out, g)
	}
	return out
}

// Awaken wakes the asleep book: the medallion's ring lights around and the
// mark fills from the foot up, a wave of light crosses the cover a tone above
// each pixel, the gems light as it reaches them, and the ribbon comes down last.
func Awaken(id string) []Grid {
	dim, ok := dimBook(id), Sprite(id, Ready)
	noRibbon := ok.clone()
	for y := range noRibbon {
		for x := range noRibbon[y] {
			switch {
			case y >= 44:
				noRibbon[y][x] = '.'
			case (y == 42 || y == 43) && (x == 21 || x == 22):
				noRibbon[y][x] = '3'
				if y == 42 && x%3 != 0 {
					noRibbon[y][x] = '2'
				}
			}
		}
	}
	out := []Grid{dim, dim}
	const ringSteps = 7
	for i := 1; i <= ringSteps; i++ {
		share := float64(i) / ringSteps
		fill := float64(medalY+6) - share*13
		g := dim.clone()
		for y := range g {
			for x := range g[y] {
				d := dist(x, y, medalX, medalY)
				switch {
				case d >= 7.5 && d <= 9.2:
					if clockwise(x, y, medalX, medalY) <= share*2*math.Pi {
						g[y][x] = noRibbon[y][x]
					}
				case d < 7.5 && ok[y][x] == '4':
					g[y][x] = fillSymbol(float64(y), fill)
				}
			}
		}
		out = append(out, g)
	}
	lit := out[len(out)-1]
	const waveSteps, waveReach = 9, 34.0
	for i := 1; i <= waveSteps; i++ {
		r := 9 + float64(i)/waveSteps*(waveReach-9)
		g := lit.clone()
		for y := range g {
			for x := range g[y] {
				d := dist(x, y, medalX, medalY)
				switch {
				case d <= 9.2:
				case d < r-4:
					g[y][x] = noRibbon[y][x]
				case d < r && noRibbon[y][x] == '.':
					g[y][x] = '.'
				case d < r:
					g[y][x] = shift(up, noRibbon[y][x])
				}
			}
		}
		out = append(out, g)
	}
	for k := 1; k <= 4; k++ {
		g := noRibbon.clone()
		for y := 42; y < 42+k; y++ {
			g[y][21], g[y][22] = '4', '4'
		}
		out = append(out, g)
	}
	return append(out, ok)
}

// fillSymbol is a mark pixel at row y with the light filled up to row fill:
// lit below it, a bright edge two rows high, dim above.
func fillSymbol(y, fill float64) byte {
	switch {
	case y >= fill:
		return '4'
	case y >= fill-2:
		return '2'
	}
	return '3'
}

// Unseal is a signed-in book's journey: the clasps open, then it awakens.
func Unseal(id string) []Grid { return append(strapFrames(id), Awaken(id)...) }

// The page's lines of text, by row, and their span.
var inkRows = [...]int{25, 27, 29, 31}

const inkFrom, inkTo = 4, 22

// Written is the loose page with the install p (0 to 1) along: a lit nib runs
// each line and leaves the text written behind it; from a quarter on the
// seal's ring closes clockwise and the mark fills from the foot up.
func Written(id string, p float64) Grid {
	g := Sprite(id, NotInstalled)
	var ink [][2]int
	for _, y := range inkRows {
		for x := inkFrom; x < inkTo; x++ {
			if g[y][x] == '3' {
				ink = append(ink, [2]int{x, y})
			}
		}
	}
	n := int(roundHalfUp(float64(len(ink)) * min(1, p/0.8)))
	for i, at := range ink {
		switch {
		case i >= n:
			g[at[1]][at[0]] = '5'
		case i >= n-2 && n < len(ink):
			g[at[1]][at[0]] = '4'
		default:
			g[at[1]][at[0]] = '2'
		}
	}
	q := clamp01((p - 0.25) / 0.75)
	fill := float64(sealY+6) - q*13
	for y := 3; y <= 23; y++ {
		for x := 2; x <= 22; x++ {
			if g[y][x] != '3' {
				continue
			}
			d := dist(x, y, sealX, sealY)
			switch {
			case d > 7.5 && d <= 8.6:
				g[y][x] = '5'
				if clockwise(x, y, sealX, sealY) <= q*2*math.Pi {
					g[y][x] = '2'
				}
			case d <= 7.2 && q > 0:
				g[y][x] = fillSymbol(float64(y), fill)
			}
		}
	}
	return g
}

// Box is g centered in the book's box, where the window paints every frame.
func Box(g Grid) Grid {
	b := blank()
	ox, oy := (Cols-len(g[0]))/2, (Rows-len(g))/2
	for y, row := range g {
		for x, c := range row {
			if c != '.' {
				b[oy+y][ox+x] = c
			}
		}
	}
	return b
}

// The written page's place in the box, which Box gives the page sprite.
const pageX, pageY, pageCols, pageRows = 4, 5, 26, 36

// cover is the binding up to the closed cover: the finished page pulses out
// from its seal, gains leaves behind it until it is a block, and the cover
// closes over it from the spine.
func cover(id string) []Grid {
	page, dim := Box(Written(id, 1)), dimBook(id)
	cx, cy := pageX+sealX, pageY+sealY
	var out []Grid
	for i := 1; i <= 4; i++ {
		r := float64(8 + i*7)
		g := page.clone()
		for y := range g {
			for x, c := range g[y] {
				if d := dist(x, y, cx, cy); c != '.' && d > 8.6 && d < r && d >= r-5 {
					g[y][x] = shift(up, c)
				}
			}
		}
		out = append(out, g)
	}
	stack := func(k int) Grid {
		g := blank()
		for j := k; j >= 1; j-- {
			for y := pageY; y < pageY+pageRows; y++ {
				for x := pageX; x < pageX+pageCols; x++ {
					X, Y := x+j, y+j
					if page[y][x] == '.' || Y >= Rows || X >= Cols {
						continue
					}
					g[Y][X] = '5'
					if x == pageX+pageCols-1 || y == pageY+pageRows-1 {
						g[Y][X] = "32"[j%2]
					}
				}
			}
		}
		for y, row := range page {
			for x, c := range row {
				if c != '.' {
					g[y][x] = c
				}
			}
		}
		return g
	}
	for k := 1; k <= 3; k++ {
		out = append(out, stack(k))
	}
	st := stack(3)
	for _, cut := range [...]int{6, 12, 18, 24, 30, 34} {
		g := st.clone()
		for y := range g {
			for x := 0; x < min(cut, Cols); x++ {
				g[y][x] = dim[y][x]
				if x >= cut-1 && cut < Cols && dim[y][x] != '.' {
					g[y][x] = '4'
				}
			}
		}
		out = append(out, g)
	}
	return out
}

// Bind is an installed provider's journey when it still needs a login: the
// cover closes and both clasps buckle, the bottom one first, the reverse of
// opening, and it ends signed out.
func Bind(id string) []Grid {
	out := cover(id)
	straps := strapFrames(id)
	for _, strap := range slices.Backward(straps) {
		out = append(out, strap)
	}
	return append(out, Sprite(id, SignedOut))
}

// BindAwake is the journey of a provider that needs no login: the cover closes
// and the book awakens with no clasps at all.
func BindAwake(id string) []Grid { return append(cover(id), Awaken(id)...) }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// roundHalfUp is JavaScript's Math.round, which the mockup's counts follow.
func roundHalfUp(v float64) float64 { return float64(int(v + 0.5)) }
