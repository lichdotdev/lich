package tray

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// devFlame is the dev instance's flame, amber-500, so its tray icon reads
// apart from an installed lich's beside it.
var devFlame = color.NRGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff}

// DevPNG is icon with its light parts, the flame, painted devFlame by how
// light they are; the dark ground stays as it is.
func DevPNG(icon []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, fmt.Errorf("decode tray icon: %w", err)
	}
	bounds := src.Bounds()
	out := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			light := (299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000
			mix := func(from, to uint8) uint8 {
				return uint8((uint32(from)*(255-light) + uint32(to)*light) / 255)
			}
			out.SetNRGBA(x, y, color.NRGBA{R: mix(c.R, devFlame.R), G: mix(c.G, devFlame.G), B: mix(c.B, devFlame.B), A: c.A})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("encode tray icon: %w", err)
	}
	return buf.Bytes(), nil
}
