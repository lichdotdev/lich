package gioterm

import (
	"image"
	"image/color"
	"math"
)

// linearMax is full intensity in the 16-bit linear light the tables below
// work in: 8 bits would band the dark end of the sRGB curve.
const linearMax = 1<<16 - 1

// The sRGB transfer function, from IEC 61966-2-1: a linear segment near
// black, a power curve above it.
const (
	srgbDecodeKnee = 0.04045
	srgbLinearKnee = 0.0031308
	srgbSlope      = 12.92
	srgbScale      = 1.055
	srgbOffset     = 0.055
	srgbGamma      = 2.4
)

var (
	// toLinear16 decodes an sRGB level to linear light.
	toLinear16 [256]uint32
	// toSRGB8 encodes linear light back to the nearest sRGB level.
	toSRGB8 [linearMax + 1]uint8
)

func init() {
	for v := range toLinear16 {
		c := float64(v) / 255
		if c <= srgbDecodeKnee {
			c /= srgbSlope
		} else {
			c = math.Pow((c+srgbOffset)/srgbScale, srgbGamma)
		}
		toLinear16[v] = uint32(math.Round(c * linearMax))
	}
	for l := range toSRGB8 {
		c := float64(l) / linearMax
		if c <= srgbLinearKnee {
			c *= srgbSlope
		} else {
			c = srgbScale*math.Pow(c, 1/srgbGamma) - srgbOffset
		}
		toSRGB8[l] = uint8(math.Round(c * 255))
	}
}

// maxOverTables bounds the tables a blender keeps: text drawn in a gradient
// of true colors would otherwise add one for nearly every character.
const maxOverTables = 1024

// overTable is one color at every mask coverage over one pixel value.
type overTable [256][4]uint8

type overKey struct {
	c     color.NRGBA
	under color.RGBA
}

// blender composites glyph masks in linear light, the space Gio blends
// everything else in. Gio takes an image's channels as sRGB and decodes each
// one, premultiplied as it is, before it blends, so a pixel keeps the sRGB
// encoding of the premultiplied linear color: that decodes to what a Gio fill
// of the same coverage would have blended. A pixel holding the ground the
// caller names, which is most of them, is read from a table kept per color
// and ground; any other is blended on its own.
type blender struct {
	tables map[overKey]*overTable
}

// mask composites c through mask, placed with mp at r.Min, onto dst, whose
// pixels under the glyph are mostly under.
func (b *blender) mask(dst *image.RGBA, r image.Rectangle, c color.NRGBA, under color.RGBA, mask *image.Alpha, mp image.Point) {
	clipped := r.Intersect(dst.Bounds())
	mp = mp.Add(clipped.Min.Sub(r.Min))
	t := b.table(c, under)
	fg := linearOf(c)
	for y := clipped.Min.Y; y < clipped.Max.Y; y++ {
		row := dst.Pix[dst.PixOffset(clipped.Min.X, y):]
		cov := mask.Pix[mask.PixOffset(mp.X, mp.Y+y-clipped.Min.Y):]
		for x := range clipped.Dx() {
			m := cov[x]
			if m == 0 {
				continue
			}
			p := row[4*x : 4*x+4 : 4*x+4]
			if p[0] == under.R && p[1] == under.G && p[2] == under.B && p[3] == under.A {
				copy(p, t[m][:])
				continue
			}
			blendPixel(p, fg, uint32(m)*uint32(c.A)/0xff)
		}
	}
}

func (b *blender) table(c color.NRGBA, under color.RGBA) *overTable {
	k := overKey{c: c, under: under}
	if t, ok := b.tables[k]; ok {
		return t
	}
	if b.tables == nil || len(b.tables) >= maxOverTables {
		b.tables = map[overKey]*overTable{}
	}
	t := new(overTable)
	fg := linearOf(c)
	for m := range t {
		t[m] = [4]uint8{under.R, under.G, under.B, under.A}
		blendPixel(t[m][:], fg, uint32(m)*uint32(c.A)/0xff)
	}
	b.tables[k] = t
	return t
}

func linearOf(c color.NRGBA) [3]uint32 {
	return [3]uint32{toLinear16[c.R], toLinear16[c.G], toLinear16[c.B]}
}

// blendPixel lays fg at coverage a (0..255) over the premultiplied pixel p.
func blendPixel(p []uint8, fg [3]uint32, a uint32) {
	rest := 0xff - a
	p[3] = uint8(a + uint32(p[3])*rest/0xff)
	p[0] = toSRGB8[(fg[0]*a+toLinear16[p[0]]*rest)/0xff]
	p[1] = toSRGB8[(fg[1]*a+toLinear16[p[1]]*rest)/0xff]
	p[2] = toSRGB8[(fg[2]*a+toLinear16[p[2]]*rest)/0xff]
}
