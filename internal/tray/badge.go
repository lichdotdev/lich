package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"math"
)

// badge is the dot on the tray icon's corner: what the sessions behind it are
// doing, so a closed window still shows that one is waiting on you.
type badge int

const (
	badgeNone badge = iota
	badgeBusy
	badgeDone
	badgeWaiting
	badgeCount
)

// badgeFills are the window's own tones for the same states (native/ui/theme.go,
// the dark palette): ToneWait for a session waiting on you, TonePass for a turn
// left unread, and MutedForeground for one still running: a grey that reads on a
// light bar and a dark one, which the window's white spinner would not.
var badgeFills = [badgeCount]color.NRGBA{
	badgeBusy:    {R: 0x9f, G: 0x9f, B: 0xa9, A: 0xff},
	badgeDone:    {R: 0x00, G: 0xbc, B: 0x7d, A: 0xff},
	badgeWaiting: {R: 0xfe, G: 0x9a, B: 0x00, A: 0xff},
}

// The badge's size and the clear ring around it, as fractions of the icon's
// width: large enough to read on a 16px tray, the ring so the dot stands off
// the icon whatever colour the bar behind it is.
const (
	badgeRadius = 0.24
	badgeRing   = 0.07
)

// badgedIcons is icons under every badge. An icon that cannot be badged is
// shown without one rather than not at all.
func badgedIcons(icons Icons) [badgeCount]Icons {
	var out [badgeCount]Icons
	out[badgeNone] = icons
	for b := badgeBusy; b < badgeCount; b++ {
		out[b] = icons
		paint := func(frame []byte) ([]byte, error) { return badgePNG(frame, badgeFills[b]) }
		if len(icons.PNG) > 0 {
			if png, err := paint(icons.PNG); err == nil {
				out[b].PNG = png
			} else {
				slog.Warn("tray: badge the icon", "err", err)
			}
		}
		if len(icons.ICO) > 0 {
			if ico, err := mapICO(icons.ICO, paint); err == nil {
				out[b].ICO = ico
			} else {
				slog.Warn("tray: badge the icon", "err", err)
			}
		}
	}
	return out
}

// badgePNG is icon with a dot of fill in its top right corner, a clear ring
// cut around it.
func badgePNG(icon []byte, fill color.NRGBA) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, fmt.Errorf("decode tray icon: %w", err)
	}
	b := src.Bounds()
	out := image.NewNRGBA(b)
	draw.Draw(out, b, src, b.Min, draw.Src)
	size := float64(b.Dx())
	r, ring := size*badgeRadius, size*badgeRing
	cx, cy := float64(b.Max.X)-r, float64(b.Min.Y)+r
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			if d > r+ring+1 {
				continue
			}
			out.SetNRGBA(x, y, badgePixel(out.NRGBAAt(x, y), fill, coverage(r-d), coverage(r+ring-d)))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("encode tray icon: %w", err)
	}
	return buf.Bytes(), nil
}

// badgePixel is under, its alpha cut by hole, with fill laid over it by dot.
func badgePixel(under, fill color.NRGBA, dot, hole float64) color.NRGBA {
	base := float64(under.A) / 255 * (1 - hole)
	alpha := dot + base*(1-dot)
	if alpha == 0 {
		return color.NRGBA{}
	}
	mix := func(f, u uint8) uint8 {
		return uint8(math.Round((float64(f)*dot + float64(u)*base*(1-dot)) / alpha))
	}
	return color.NRGBA{R: mix(fill.R, under.R), G: mix(fill.G, under.G), B: mix(fill.B, under.B), A: uint8(math.Round(alpha * 255))}
}

// coverage is how much of a pixel an edge inside it by d pixels covers: the
// edge's antialiasing.
func coverage(d float64) float64 {
	return math.Max(0, math.Min(1, d+0.5))
}
