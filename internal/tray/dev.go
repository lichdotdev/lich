package tray

import (
	"bytes"
	"encoding/binary"
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

// icoHeader and icoEntry are the sizes of an ICO's header and of each image's
// entry in the directory after it.
const (
	icoHeader = 6
	icoEntry  = 16
)

// DevICO is icon, an ICO whose images are all PNG, with each image painted as
// DevPNG paints one. An ICO image stored as a bitmap is an error.
func DevICO(icon []byte) ([]byte, error) {
	if len(icon) < icoHeader {
		return nil, fmt.Errorf("tray icon: %d bytes is no ICO header", len(icon))
	}
	count := int(binary.LittleEndian.Uint16(icon[4:]))
	dirEnd := icoHeader + count*icoEntry
	if len(icon) < dirEnd {
		return nil, fmt.Errorf("tray icon: directory of %d images runs past %d bytes", count, len(icon))
	}
	out := bytes.NewBuffer(append([]byte(nil), icon[:dirEnd]...))
	for i := range count {
		entry := out.Bytes()[icoHeader+i*icoEntry:]
		size := int(binary.LittleEndian.Uint32(entry[8:]))
		offset := int(binary.LittleEndian.Uint32(entry[12:]))
		if offset+size > len(icon) {
			return nil, fmt.Errorf("tray icon: image %d runs past %d bytes", i, len(icon))
		}
		frame, err := DevPNG(icon[offset : offset+size])
		if err != nil {
			return nil, fmt.Errorf("tray icon image %d: %w", i, err)
		}
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(frame)))
		binary.LittleEndian.PutUint32(entry[12:], uint32(out.Len()))
		out.Write(frame)
	}
	return out.Bytes(), nil
}
