package ui

import (
	"testing"
	"time"
)

func TestSegmentsLitRoundsAndKeepsAnyShareVisible(t *testing.T) {
	for _, tc := range []struct{ percent, want int }{{0, 0}, {1, 1}, {2, 1}, {4, 1}, {50, 10}, {57, 11}, {97, 19}, {98, 20}, {100, 20}, {140, 20}, {-5, 0}} {
		if got := SegmentsLit(tc.percent, 20); got != tc.want {
			t.Errorf("%d%% lights %d of 20, want %d", tc.percent, got, tc.want)
		}
	}
}

// TestWaveLevelSweepsAndRests: a segment rises to full tone and fades, the
// next one a step behind, and the whole bar rests dark before the next lap.
func TestWaveLevelSweepsAndRests(t *testing.T) {
	lap := time.Unix(0, 0).Add(10 * waveLap)
	peak := lap.Add(time.Duration(waveRise * float64(waveLap)))
	if got := WaveLevel(peak, 0); got != 1 {
		t.Errorf("segment 0 at its peak is %.2f lit, want 1", got)
	}
	if got := WaveLevel(peak.Add(waveStep), 1); got != 1 {
		t.Errorf("segment 1 a step later is %.2f lit, want 1", got)
	}
	if got := WaveLevel(peak, 1); got >= 1 || got <= 0 {
		t.Errorf("segment 1 at segment 0's peak is %.2f lit, want rising", got)
	}
	rest := lap.Add(waveLap - waveStep)
	for i := range 4 {
		if got := WaveLevel(rest, i); got != 0 {
			t.Errorf("segment %d at the lap's end is %.2f lit, want dark", i, got)
		}
	}
	if got := WaveLevel(lap.Add(waveLap), 0); got != WaveLevel(lap, 0) {
		t.Errorf("a lap later segment 0 reads %.2f, want %.2f", got, WaveLevel(lap, 0))
	}
}
