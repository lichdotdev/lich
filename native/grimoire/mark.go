package grimoire

// markR is the reach of the provider's mark from the medallion's center: the
// medallion's ring starts past it.
const markR = 8

// Mark is provider id's mark as its book's medallion carries it, without the
// medallion's ring: its Foreground pixels (4) and the half-lit ones (2) inside
// them, cropped to the box of the 4s, nil for a provider with no book. It is
// the sidebar's provider icon.
func Mark(id string) Grid {
	book := Sprite(id, Ready)
	if book == nil {
		return nil
	}
	x0, y0, x1, y1 := Cols, Rows, -1, -1
	for y := medalY - markR; y <= medalY+markR; y++ {
		for x := medalX - markR; x <= medalX+markR; x++ {
			if book[y][x] != '4' || dist(x, y, medalX, medalY) > markR {
				continue
			}
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
		}
	}
	if x1 < 0 {
		return nil
	}
	g := make(Grid, y1-y0+1)
	for y := range g {
		g[y] = make([]byte, x1-x0+1)
		for x := range g[y] {
			g[y][x] = markCell(book[y0+y][x0+x], dist(x0+x, y0+y, medalX, medalY))
		}
	}
	return g
}

// markCell is a book's symbol at r from the medallion's center as the mark
// keeps it. A half-lit cell counts a pixel short of markR: the medallion's
// ring, of the same symbol, runs that close.
func markCell(c byte, r float64) byte {
	switch {
	case c == '4' && r <= markR, c == '2' && r <= markR-1:
		return c
	}
	return '.'
}
