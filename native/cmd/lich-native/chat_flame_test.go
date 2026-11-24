package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

func TestTheFlameTakesItsColorsFromTheThemesTokens(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	th.Foreground = color.NRGBA{R: 250, G: 240, B: 230, A: 255}
	th.Background = color.NRGBA{R: 20, G: 20, B: 30, A: 255}
	th.Primary = color.NRGBA{R: 10, G: 200, B: 30, A: 255}
	for symbol, want := range map[byte]color.NRGBA{
		'4': ui.MixOKLab(th.Primary, th.Foreground, 0.5),
		'2': th.Primary,
		'3': ui.MixOKLab(th.Background, th.Primary, 0.65),
		'1': ui.MixOKLab(th.Background, th.Primary, 0.35),
	} {
		if got, ok := chatFlameColor(th, symbol); !ok || got != want {
			t.Errorf("symbol %c is %v (%v), want %v", symbol, got, ok, want)
		}
	}
	if _, ok := chatFlameColor(th, '.'); ok {
		t.Error("an empty pixel is drawn")
	}
}

// The turn's line is as tall as the spinner and the verb sits at its middle.
func TestTheSpinnerCentersOnTheVerbAndTheLineIsNoTaller(t *testing.T) {
	var line, label D
	r := newFrameRig(image.Pt(400, 200), func(gtx C, th *ui.Theme) {
		gtx.Constraints.Min = image.Point{}
		label = chatLiveLabel(gtx, th, chatVerb("s1", 1), th.MutedForeground, time.Time{})
		line = chatLive(gtx, th, "s1", chatRow{turn: 1})
	})
	r.frame()
	const foot = 22 // 11 rows of 1.5dp pixels, two device pixels each at 1x
	if line.Size.Y != foot {
		t.Fatalf("the line is %dpx tall, want the spinner's, %dpx", line.Size.Y, foot)
	}
	// Both verbs are drawn from the top of the screen: the bare one, then the
	// line's, which the spinner pushes down by half of what it is taller.
	var tops []int
	for _, n := range r.nodes() {
		if n.Desc.Label == chatVerb("s1", 1) {
			tops = append(tops, n.Desc.Bounds.Min.Y)
		}
	}
	if len(tops) != 2 {
		t.Fatalf("%d verbs drawn, want the bare one and the line's:\n%s", len(tops), r.drawn())
	}
	if got, want := tops[1]-tops[0], (foot-label.Size.Y)/2; got != want {
		t.Errorf("the verb sits %dpx down the line, want %dpx, the middle of the spinner", got, want)
	}
}
