package webview

import "image"

// visible is r less cuts, as rectangles that do not overlap: the part of a
// page the window draws nothing over, which a system with no cut of its own
// (a layer's mask on macOS) shows instead.
func visible(r image.Rectangle, cuts []image.Rectangle) []image.Rectangle {
	out := []image.Rectangle{r}
	for _, c := range cuts {
		var next []image.Rectangle
		for _, o := range out {
			next = append(next, without(o, c)...)
		}
		out = next
	}
	return out
}

// without is r less c, in up to four rectangles: above c, below it, and
// either side of it between them.
func without(r, c image.Rectangle) []image.Rectangle {
	in := r.Intersect(c)
	if in.Empty() {
		return []image.Rectangle{r}
	}
	var out []image.Rectangle
	for _, part := range []image.Rectangle{
		{Min: r.Min, Max: image.Pt(r.Max.X, in.Min.Y)},
		{Min: image.Pt(r.Min.X, in.Max.Y), Max: r.Max},
		{Min: image.Pt(r.Min.X, in.Min.Y), Max: image.Pt(in.Min.X, in.Max.Y)},
		{Min: image.Pt(in.Max.X, in.Min.Y), Max: image.Pt(r.Max.X, in.Max.Y)},
	} {
		if !part.Empty() {
			out = append(out, part)
		}
	}
	return out
}
