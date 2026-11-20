package grimoire

import (
	"testing"
	"time"
)

func TestLogoReadsEveryPixelAsASymbol(t *testing.T) {
	if len(Logo) != LogoFrames {
		t.Fatalf("%d frames", len(Logo))
	}
	painted := 0
	for _, g := range Logo {
		for _, row := range g {
			for _, c := range row {
				if c != '.' {
					painted++
				}
			}
		}
	}
	// The sheet's 40668 opaque pixels are all one of the five tones.
	if painted != 40668 {
		t.Errorf("%d painted pixels, want every opaque one of the sheet", painted)
	}
}

func TestLogoFrameLoops(t *testing.T) {
	start := time.Unix(0, 0)
	if f, next := LogoFrame(start); f != 0 || next.Sub(start) != LogoStep {
		t.Errorf("frame %d, next in %v", f, next.Sub(start))
	}
	if f, _ := LogoFrame(start.Add(LogoFrames * LogoStep)); f != 0 {
		t.Errorf("frame %d after a whole loop", f)
	}
}
