package icons

import (
	"strconv"
	"strings"

	"gioui.org/f32"
)

// SVGPath is one of an icon's shapes as SVG path data, for a page to draw:
// filled, stroked SVGStroke wide, or both.
type SVGPath struct {
	D      string `json:"d"`
	Fill   bool   `json:"fill"`
	Stroke bool   `json:"stroke"`
}

// SVGBox and SVGStroke are the side of an Icon's viewBox and its stroke
// width, in the units of its SVGPaths; BibataArrow's box is BibataArrowBox.
const (
	SVGBox    = viewBox
	SVGStroke = lucideStroke
)

// SVGPaths are the icon's shapes as SVG path data, in its viewBox units.
func (ic *Icon) SVGPaths() []SVGPath {
	paths := make([]SVGPath, len(ic.shapes))
	for i, sh := range ic.shapes {
		var d strings.Builder
		for _, s := range sh.segs {
			d.WriteByte(s.op)
			switch s.op {
			case 'M', 'L':
				writePoints(&d, s.to)
			case 'C':
				writePoints(&d, s.c1, s.c2, s.to)
			}
		}
		paths[i] = SVGPath{D: d.String(), Fill: sh.fill, Stroke: sh.stroke}
	}
	return paths
}

func writePoints(d *strings.Builder, points ...f32.Point) {
	for i, p := range points {
		if i > 0 {
			d.WriteByte(' ')
		}
		d.WriteString(strconv.FormatFloat(float64(p.X), 'f', -1, 32))
		d.WriteByte(' ')
		d.WriteString(strconv.FormatFloat(float64(p.Y), 'f', -1, 32))
	}
}
