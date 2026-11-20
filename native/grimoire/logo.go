package grimoire

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"time"
)

// The logo's flames: LogoFrames frames of LogoCols x LogoRows, laid out on
// logoSheetCols columns of the sheet, each held LogoStep.
const (
	LogoCols      = 56
	LogoRows      = 84
	LogoFrames    = 32
	LogoStep      = 70 * time.Millisecond
	logoSheetCols = 8
)

//go:embed logo.png
var logoPNG []byte

// logoTones are the sheet's five grays, darkest first, as the symbols the
// window paints them in; the sheet was drawn on the zinc ramp the books'
// symbols name.
var logoTones = map[color.NRGBA]byte{
	{0x3f, 0x3f, 0x46, 0xff}: '1',
	{0x71, 0x71, 0x7a, 0xff}: '5',
	{0xa1, 0xa1, 0xaa, 0xff}: '3',
	{0xd4, 0xd4, 0xd8, 0xff}: '2',
	{0xf8, 0xf8, 0xf8, 0xff}: '4',
}

// Logo is the logo's frames, read off the sheet once.
var Logo = func() []Grid {
	img, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		panic(err)
	}
	frames := make([]Grid, LogoFrames)
	for i := range frames {
		frames[i] = logoFrame(img, image.Pt(i%logoSheetCols*LogoCols, i/logoSheetCols*LogoRows))
	}
	return frames
}()

func logoFrame(img image.Image, at image.Point) Grid {
	g := make(Grid, LogoRows)
	for y := range g {
		g[y] = make([]byte, LogoCols)
		for x := range g[y] {
			c := color.NRGBAModel.Convert(img.At(at.X+x, at.Y+y)).(color.NRGBA)
			g[y][x] = '.'
			if s, ok := logoTones[c]; ok {
				g[y][x] = s
			}
		}
	}
	return g
}

// LogoFrame is the frame showing at now, and when the next one starts.
func LogoFrame(now time.Time) (frame int, next time.Time) {
	tick := now.UnixNano() / int64(LogoStep)
	return int(tick % LogoFrames), time.Unix(0, (tick+1)*int64(LogoStep))
}
