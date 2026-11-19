package main

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

func TestEveryFlameFrameIsAGridOfFourteenBySeventeenOfKnownSymbols(t *testing.T) {
	if len(chatFlameFrames) != 16 {
		t.Fatalf("%d frames, want 16", len(chatFlameFrames))
	}
	for i, frame := range chatFlameFrames {
		rows := strings.Split(frame, " | ")
		if len(rows) != chatFlameRows {
			t.Errorf("frame %d has %d rows, want %d", i, len(rows), chatFlameRows)
		}
		for y, row := range rows {
			if len(row) != chatFlameCols || strings.Trim(row, ".1234") != "" {
				t.Errorf("frame %d row %d is %q, want %d of . 1 2 3 4", i, y, row, chatFlameCols)
			}
		}
	}
}

func TestTheFlamesCoreIsTheSameInEveryFrame(t *testing.T) {
	for i, frame := range chatFlameFrames {
		rows := strings.Split(frame, " | ")
		for y := 13; y <= 15; y++ {
			if rows[y][6:8] != "44" {
				t.Errorf("frame %d: columns 6 and 7 of row %d are %q, want the core", i, y, rows[y][6:8])
			}
		}
	}
}

func TestTheFlameTakesItsColorsFromTheThemesTokens(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	th.Foreground = color.NRGBA{R: 250, G: 240, B: 230, A: 255}
	th.MutedForeground = color.NRGBA{R: 100, G: 90, B: 80, A: 255}
	th.Accent = color.NRGBA{R: 10, G: 200, B: 30, A: 255}
	for symbol, want := range map[byte]color.NRGBA{
		'4': th.Foreground,
		'3': th.MutedForeground,
		'1': th.Accent,
		'2': ui.MixOKLab(th.MutedForeground, th.Foreground, 0.55),
	} {
		if got, ok := chatFlameColor(th, symbol); !ok || got != want {
			t.Errorf("symbol %c is %v (%v), want %v", symbol, got, ok, want)
		}
	}
	if _, ok := chatFlameColor(th, '.'); ok {
		t.Error("an empty pixel is drawn")
	}
}

func TestTheFlameAdvancesAFrameEveryStepAndLoops(t *testing.T) {
	start := time.Unix(1000, 0)
	seen := map[int]bool{}
	for i := range 2 * len(chatFlameFrames) {
		now := start.Add(time.Duration(i) * chatFlameStep)
		frame, next := chatFlameFrame(now)
		seen[frame] = true
		if wait := next.Sub(now); wait <= 0 || wait > chatFlameStep {
			t.Fatalf("at step %d the next frame is %v away, want within one step", i, wait)
		}
		if again, _ := chatFlameFrame(now.Add(chatFlameStep / 4)); again != frame {
			t.Fatalf("step %d moved to frame %d inside its step", i, again)
		}
	}
	if len(seen) != len(chatFlameFrames) {
		t.Errorf("%d frames shown, want all %d", len(seen), len(chatFlameFrames))
	}
}

// The flame is taller than the line it burns on: only its foot takes room, so
// the turn's line is as tall as the foot and the verb sits at the foot's middle.
func TestTheFlamesFootCentersOnTheVerbAndTheLineIsNoTaller(t *testing.T) {
	var line, label D
	r := newFrameRig(image.Pt(400, 200), func(gtx C, th *ui.Theme) {
		gtx.Constraints.Min = image.Point{}
		label = chatLiveLabel(gtx, th, chatVerb("s1", 1), th.MutedForeground, time.Time{})
		line = chatLive(gtx, th, "s1", chatRow{turn: 1})
	})
	r.frame()
	const foot = 22 // 11 rows of 1.5dp pixels, two device pixels each at 1x
	if line.Size.Y != foot {
		t.Fatalf("the line is %dpx tall, want the flame's foot, %dpx", line.Size.Y, foot)
	}
	// Both verbs are drawn from the top of the screen: the bare one, then the
	// line's, which the foot pushes down by half of what it is taller.
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
		t.Errorf("the verb sits %dpx down the line, want %dpx, the middle of the foot", got, want)
	}
}
