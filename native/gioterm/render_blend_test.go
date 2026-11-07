package gioterm

import (
	"image"
	"image/color"
	"math"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/glyphs"
)

// stemProbe is text heavy in vertical stems, round bowls and serifs, whose
// antialiased edges show how a renderer blends coverage.
const stemProbe = "lHEximno"

// The bundled themes' terminal colors, themes/dark.json and themes/light.json.
var (
	darkTerminal  = Colors{Foreground: color.NRGBA{R: 0xf1, G: 0xf1, B: 0xf4, A: 0xff}, Background: color.NRGBA{R: 0x10, G: 0x10, B: 0x12, A: 0xff}}
	lightTerminal = Colors{Foreground: color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff}, Background: color.NRGBA{R: 0xe8, G: 0xe8, B: 0xea, A: 0xff}}
)

// maxEdgeError is how far, in 8-bit sRGB levels, an edge pixel may land from
// the linear-light blend: the rounding of the GPU's sRGB encode and decode.
const maxEdgeError = 2

// Gio blends every fill, its own text included, in linear light: an edge
// pixel of a glyph rasterized here must be its coverage of the foreground
// over the background blended the same way, or terminal text weighs other
// than the window's.
func TestGlyphEdgesBlendInLinearLight(t *testing.T) {
	for _, kind := range []string{RenderRows, RenderAtlas} {
		for _, theme := range []struct {
			name   string
			colors Colors
		}{{"dark", darkTerminal}, {"light", lightTerminal}} {
			for _, scale := range []float32{1, 2} {
				t.Run(kind+"/"+theme.name+"/"+scaleName(scale), func(t *testing.T) {
					m := measureEdges(t, kind, theme.colors, scale)
					t.Logf("edge pixels %d, max error %d, mean error %.2f levels; ink %.1f%% of coverage", m.edges, m.maxErr, m.meanErr, 100*m.ink)
					if m.maxErr > maxEdgeError {
						t.Errorf("an edge pixel is %d levels from the linear-light blend, want at most %d", m.maxErr, maxEdgeError)
					}
				})
			}
		}
	}
}

func scaleName(s float32) string {
	if s == 1 {
		return "1x"
	}
	return "2x"
}

type edgeMeasure struct {
	edges   int
	maxErr  int
	meanErr float64
	// ink is the probe's weight in linear light as a fraction of the weight
	// its coverage gives, 1 for a correct blend.
	ink float64
}

func measureEdges(t *testing.T, kind string, colors Colors, scale float32) edgeMeasure {
	t.Helper()
	r, err := NewRenderer(kind, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(len(stemProbe)+2, 2, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	if err := vt.SetColors(colors); err != nil {
		t.Fatal(err)
	}
	vt.Write([]byte("\x1b[?25l" + stemProbe))
	got := renderHeadless(t, r, vt, scale)
	return compareEdges(t, got, rasterizerOf(r), colors)
}

func rasterizerOf(r Renderer) *glyphs.Rasterizer {
	switch r := r.(type) {
	case *rowRenderer:
		return r.r
	case *atlasRenderer:
		return r.r
	}
	return nil
}

func renderHeadless(t *testing.T, r Renderer, vt *Terminal, scale float32) *image.RGBA {
	t.Helper()
	const w, h = 400, 120
	win, err := headless.NewWindow(w, h)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: scale, PxPerSp: scale}, Constraints: layout.Exact(image.Pt(w, h))}
	if err := r.Measure(gtx); err != nil {
		t.Fatal(err)
	}
	s, err := vt.Update()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Draw(gtx, s); err != nil {
		t.Fatal(err)
	}
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

// compareEdges checks every partly covered pixel of the probe against its
// coverage blended in linear light.
func compareEdges(t *testing.T, got *image.RGBA, r *glyphs.Rasterizer, colors Colors) edgeMeasure {
	t.Helper()
	fg, bg := colors.Foreground, colors.Background
	var m edgeMeasure
	var sumErr, inkGot, inkWant float64
	for i, c := range stemProbe {
		mask, err := r.Mask(glyphs.Key{R: c})
		if err != nil {
			t.Fatal(err)
		}
		origin := image.Pt(i*r.CellW(), 0).Add(mask.Off)
		b := mask.Img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				a := float64(mask.Img.AlphaAt(x, y).A) / 255
				px := got.RGBAAt(origin.X+x, origin.Y+y)
				inkGot += inkOf(px.G, fg.G, bg.G)
				inkWant += a
				if a == 0 || a == 1 {
					continue
				}
				d := channelError(px, linearBlend(fg, bg, a))
				m.edges++
				m.maxErr = max(m.maxErr, d)
				sumErr += float64(d)
			}
		}
	}
	m.meanErr = sumErr / float64(m.edges)
	m.ink = inkGot / inkWant
	return m
}

// inkOf is how far v lies from the background toward the foreground, in
// linear light.
func inkOf(v, fg, bg uint8) float64 {
	return (toLinear(v) - toLinear(bg)) / (toLinear(fg) - toLinear(bg))
}

func linearBlend(fg, bg color.NRGBA, a float64) color.NRGBA {
	mix := func(f, b uint8) uint8 {
		return toSRGB(toLinear(f)*a + toLinear(b)*(1-a))
	}
	return color.NRGBA{R: mix(fg.R, bg.R), G: mix(fg.G, bg.G), B: mix(fg.B, bg.B), A: 0xff}
}

func channelError(got color.RGBA, want color.NRGBA) int {
	d := func(a, b uint8) int { return int(math.Abs(float64(a) - float64(b))) }
	return max(d(got.R, want.R), d(got.G, want.G), d(got.B, want.B))
}

func toLinear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func toSRGB(l float64) uint8 {
	if l <= 0.0031308 {
		return uint8(math.Round(255 * 12.92 * l))
	}
	return uint8(math.Round(255 * (1.055*math.Pow(l, 1/2.4) - 0.055)))
}

// A pixel that is not the ground the caller named, where a glyph overhangs
// another, blends to what the table gives over the same pixel.
func TestBlenderOffGroundMatchesItsTable(t *testing.T) {
	ground := color.RGBA{R: 0x10, G: 0x10, B: 0x12, A: 0xff}
	fg := color.NRGBA{R: 0xf1, G: 0xf1, B: 0xf4, A: 140}
	mask := image.NewAlpha(image.Rect(0, 0, 256, 1))
	for i := range mask.Pix {
		mask.Pix[i] = uint8(i)
	}
	draw := func(under color.RGBA) *image.RGBA {
		img := image.NewRGBA(mask.Rect)
		for x := range 256 {
			img.SetRGBA(x, 0, ground)
		}
		var b blender
		b.mask(img, img.Bounds(), fg, under, mask, image.Point{})
		return img
	}
	tabled, blended := draw(ground), draw(color.RGBA{})
	for x := range 256 {
		if tabled.RGBAAt(x, 0) != blended.RGBAAt(x, 0) {
			t.Fatalf("coverage %d: table %v, blended %v", x, tabled.RGBAAt(x, 0), blended.RGBAAt(x, 0))
		}
	}
}
