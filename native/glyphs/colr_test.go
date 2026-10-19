package glyphs

import (
	"bytes"
	"image"
	"math"
	"testing"

	gotext "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func turns(degrees float64) tables.Coord { return tables.Coord(math.Round(degrees / 180 * (1 << 14))) }

func TestColorLineExtendsPastItsStops(t *testing.T) {
	black, white := rgba{a: 1}, rgba{1, 1, 1, 1}
	line := func(extend tables.Extend) colorLine {
		return colorLine{stops: []colorStop{{0, black}, {1, white}}, extend: extend}
	}
	cases := []struct {
		name   string
		extend tables.Extend
		t      float64
		want   float64
	}{
		{"between the stops", tables.ExtendPad, 0.25, 0.25},
		{"pad keeps the nearest stop", tables.ExtendPad, 1.5, 1},
		{"repeat starts over", tables.ExtendRepeat, 1.25, 0.25},
		{"reflect runs back", tables.ExtendReflect, 1.25, 0.75},
		{"reflect below the first stop", tables.ExtendReflect, -0.25, 0.25},
	}
	for _, c := range cases {
		if got := line(c.extend).at(c.t).r; !near(got, c.want) {
			t.Errorf("%s: red at %v = %v, want %v", c.name, c.t, got, c.want)
		}
	}
}

func TestLinearGradientRunsPerpendicularToItsRotationPoint(t *testing.T) {
	// p2 straight above p0 keeps p0→p1 as is; p1 off the axis is projected
	// onto it, so its own y does not count.
	at := linearAt(tables.PaintLinearGradient{X0: 0, Y0: 0, X1: 100, Y1: 40, X2: 0, Y2: 100})
	if got, ok := at(50, 30); !ok || !near(got, 0.5) {
		t.Errorf("t at (50, 30) = %v, %v; want 0.5", got, ok)
	}
}

func TestRadialGradientGrowsFromItsStartCircle(t *testing.T) {
	at := radialAt(tables.PaintRadialGradient{X0: 0, Y0: 0, Radius0: 0, X1: 0, Y1: 0, Radius1: 100})
	if got, ok := at(30, 40); !ok || !near(got, 0.5) {
		t.Errorf("t at distance 50 = %v, %v; want 0.5", got, ok)
	}
}

func TestSweepGradientTurnsCounterClockwise(t *testing.T) {
	// Angles are stored as degrees/180 - 1: -1 is 0°, 1 is 360°.
	at := sweepAt(tables.PaintSweepGradient{StartAngle: turns(-180), EndAngle: turns(180)})
	if got, ok := at(0, 10); !ok || !near(got, 0.25) {
		t.Errorf("t straight above the center = %v, %v; want 0.25", got, ok)
	}
}

func TestCompositeModes(t *testing.T) {
	red, halfBlue := rgba{r: 1, a: 1}, rgba{b: 0.5, a: 0.5}
	if got := compositePixel(red, halfBlue, tables.CompositeSrcIn); got != (rgba{r: 0.5, a: 0.5}) {
		t.Errorf("src-in = %+v, want the source at the backdrop's alpha", got)
	}
	if got := compositePixel(red, halfBlue, tables.CompositeDestOut); got != (rgba{}) {
		t.Errorf("dest-out under an opaque source = %+v, want nothing", got)
	}
	gray := rgba{0.5, 0.5, 0.5, 1}
	if got := compositePixel(gray, gray, tables.CompositeMultiply); !near(got.r, 0.25) || got.a != 1 {
		t.Errorf("multiply = %+v, want 0.25 gray", got)
	}
}

func TestAffineThenAppliesTheInnerTransformFirst(t *testing.T) {
	m := scaleBy(2, 2).then(translate(10, 0))
	if x, y := m.apply(1, 1); !near(x, 22) || !near(y, 2) {
		t.Errorf("scale after translate maps (1, 1) to (%v, %v), want (22, 2)", x, y)
	}
	inv, ok := m.invert()
	if x, y := inv.apply(22, 2); !ok || !near(x, 1) || !near(y, 1) {
		t.Errorf("inverse maps (22, 2) to (%v, %v), want (1, 1)", x, y)
	}
	if x, y := rotate(0.5).apply(1, 0); !near(x, 0) || !near(y, 1) {
		t.Errorf("a quarter turn maps (1, 0) to (%v, %v), want (0, 1)", x, y)
	}
}

// A PaintGlyph fills its glyph's outline, placed by the transforms above it,
// and nothing outside it.
func TestPainterFillsAGlyphWhereItsTransformsPutIt(t *testing.T) {
	r := sizedRasterizer(t)
	face, err := gotext.ParseTTF(bytes.NewReader(r.styles[0].file.data))
	if err != nil {
		t.Fatal(err)
	}
	gid, ok := face.NominalGlyph('I')
	if !ok {
		t.Fatal("the monospace face has no I")
	}
	cp := colrPainter{
		face:    face,
		palette: []tables.ColorRecord{{Red: 0xff, Alpha: 0xff}},
		bounds:  image.Rect(0, 0, 64, 64),
	}
	upem := float64(face.Upem())
	toPixels := affine{xx: 32 / upem, yy: -32 / upem, dy: 32}
	glyph := tables.PaintGlyph{GlyphID: uint16(gid), Paint: tables.PaintSolid{PaletteIndex: 0, Alpha: 1 << 14}}
	moved := tables.PaintTranslate{Paint: glyph, Dx: int16(upem), Dy: -int16(upem / 2)}

	at := image.NewRGBA(cp.bounds)
	if err := cp.paint(at, moved, toPixels, cp.opaque(), 0); err != nil {
		t.Fatal(err)
	}
	painted := image.Rectangle{}
	for y := range 64 {
		for x := range 64 {
			if p := at.RGBAAt(x, y); p.A > 0 {
				if p.G != 0 || p.B != 0 {
					t.Fatalf("pixel (%d, %d) = %v, want palette red", x, y, p)
				}
				painted = painted.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	// The I sits on the baseline at y 32 before the move; one em right and
	// half an em down puts it between x 32 and 64, under y 48.
	if painted.Empty() || painted.Min.X < 32 || painted.Max.Y > 49 || painted.Min.Y < 16 {
		t.Errorf("the glyph painted %v, want it right of x 32 and resting on y 48", painted)
	}
}

func TestPainterStopsOnACycle(t *testing.T) {
	cp := colrPainter{bounds: image.Rect(0, 0, 1, 1)}
	var cycle tables.PaintTable = tables.PaintSolid{}
	for range maxPaintDepth + 2 {
		cycle = tables.PaintTranslate{Paint: cycle}
	}
	if err := cp.paint(image.NewRGBA(cp.bounds), cycle, affine{xx: 1, yy: 1}, cp.opaque(), 0); err == nil {
		t.Error("a paint graph deeper than maxPaintDepth drew instead of failing")
	}
}
