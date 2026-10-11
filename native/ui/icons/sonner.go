package icons

import (
	"fmt"

	"gioui.org/f32"
)

// sonnerGlyph is one of the type icons sonner 2.0.7 draws in a toast (its
// dist/index.mjs SuccessIcon, InfoIcon, WarningIcon, ErrorIcon): a single
// fill-rule="evenodd" path in its own square viewBox.
type sonnerGlyph struct {
	viewBox float32
	d       string
}

// sonnerGlyphs are Heroicons solid paths, as sonner vendors them.
var sonnerGlyphs = map[string]sonnerGlyph{
	"success": {20, "M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z"},
	"info":    {20, "M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z"},
	"warning": {24, "M9.401 3.003c1.155-2 4.043-2 5.197 0l7.355 12.748c1.154 2-.29 4.5-2.599 4.5H4.645c-2.309 0-3.752-2.5-2.598-4.5L9.4 3.003zM12 8.25a.75.75 0 01.75.75v3.75a.75.75 0 01-1.5 0V9a.75.75 0 01.75-.75zm0 8.25a.75.75 0 100-1.5.75.75 0 000 1.5z"},
	"error":   {20, "M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z"},
}

// Sonner returns sonner's type icon of that name ("success", "info",
// "warning", "error"), drawn filled and scaled to fill the icon square as its
// 20px svg does. It panics on any other name, like Lucide.
func Sonner(name string) *Icon {
	g, ok := sonnerGlyphs[name]
	if !ok {
		panic(fmt.Sprintf("icons: no sonner icon %q", name))
	}
	var b pathBuilder
	if err := b.path(g.d); err != nil {
		panic(fmt.Sprintf("icons: sonner %q: %v", name, err))
	}
	segs := evenOddAsNonzero(b.segs)
	scale := viewBox / g.viewBox
	for i := range segs {
		segs[i].c1, segs[i].c2, segs[i].to = segs[i].c1.Mul(scale), segs[i].c2.Mul(scale), segs[i].to.Mul(scale)
	}
	return &Icon{shapes: []shape{{segs: segs, fill: true}}}
}

// evenOddAsNonzero rewinds subpaths so Gio's nonzero fill draws what
// evenodd would: the largest subpath is the outline, every other one is a
// hole wound against it.
// ponytail: one level of holes; a hole holding an island would fill wrong,
// and none of sonner's four icons has one.
func evenOddAsNonzero(segs []segment) []segment {
	subs := subpaths(segs)
	outer := 0
	for i, s := range subs {
		if abs(signedArea(s)) > abs(signedArea(subs[outer])) {
			outer = i
		}
	}
	outerSign := signedArea(subs[outer]) > 0
	var out []segment
	for i, s := range subs {
		if i != outer && (signedArea(s) > 0) == outerSign {
			s = reversed(s)
		}
		out = append(out, s...)
	}
	return out
}

// subpaths splits segs at each MoveTo.
func subpaths(segs []segment) [][]segment {
	var subs [][]segment
	for _, s := range segs {
		if s.op == 'M' {
			subs = append(subs, nil)
		}
		subs[len(subs)-1] = append(subs[len(subs)-1], s)
	}
	return subs
}

// signedArea is the shoelace area of the polygon through a subpath's end
// and control points: enough for the sign of its winding.
func signedArea(sub []segment) float32 {
	var pts []f32.Point
	for _, s := range sub {
		switch s.op {
		case 'C':
			pts = append(pts, s.c1, s.c2, s.to)
		case 'M', 'L':
			pts = append(pts, s.to)
		}
	}
	var area float32
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		area += p.X*q.Y - q.X*p.Y
	}
	return area / 2
}

// reversed is a closed subpath walked the other way.
func reversed(sub []segment) []segment {
	var ends []f32.Point
	for _, s := range sub {
		if s.op != 'Z' {
			ends = append(ends, s.to)
		}
	}
	out := []segment{{op: 'M', to: ends[len(ends)-1]}}
	for i := len(sub) - 1; i >= 1; i-- {
		s := sub[i]
		if s.op == 'Z' {
			continue
		}
		prev := ends[i-1]
		if s.op == 'C' {
			out = append(out, segment{op: 'C', c1: s.c2, c2: s.c1, to: prev})
			continue
		}
		out = append(out, segment{op: 'L', to: prev})
	}
	return append(out, segment{op: 'Z'})
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
