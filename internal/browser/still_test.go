package browser

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

const (
	jpegFrame  = "anBlZw=="
	stillFrame = "cG5n"
	stillTab   = "t1"
)

// stillRig is a pane drawing a page whose still capture is still.
func stillRig(t *testing.T, still string) (*Service, *fakePage, *emitted, string) {
	t.Helper()
	s, launch, out := viewService(t)
	if _, err := s.Attach("s", Viewport{W: 640, H: 480, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	page := launch.pages[0]
	page.mu.Lock()
	page.still, page.stillTab = still, stillTab
	page.mu.Unlock()
	return s, page, out, api.BrowserFrameEvents.Of("s").Name
}

// settled waits out a settle and the capture after it.
func settled() { time.Sleep(stillAfter + 300*time.Millisecond) }

func waitFrames(t *testing.T, out *emitted, name string, want ...string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var got []string
		for _, f := range out.named(name) {
			frame := f.(api.BrowserFrame)
			if frame.Tab != stillTab {
				t.Fatalf("a frame of tab %q, want %q", frame.Tab, stillTab)
			}
			got = append(got, frame.Data)
		}
		if len(got) >= len(want) {
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("frames = %v, want %v", got, want)
				}
			}
			if len(got) > len(want) {
				t.Fatalf("frames = %v, want %v", got, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("frames = %v, want %v", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func stills(p *fakePage) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stills
}

func TestAStillPageGetsOnePNGAfterItsLastFrame(t *testing.T) {
	_, page, out, name := stillRig(t, stillFrame)
	page.onFrame(stillTab, jpegFrame)
	waitFrames(t, out, name, jpegFrame, stillFrame)
	settled()
	if n := stills(page); n != 1 {
		t.Fatalf("%d captures of a page that stayed still, want 1", n)
	}
	waitFrames(t, out, name, jpegFrame, stillFrame)
}

func TestFramesInARowAreCapturedOnceTheyStop(t *testing.T) {
	_, page, out, name := stillRig(t, stillFrame)
	var want []string
	for i := range 5 {
		f := jpegFrame + strconv.Itoa(i)
		want = append(want, f)
		page.onFrame(stillTab, f)
		time.Sleep(stillAfter / 4)
	}
	if n := stills(page); n != 0 {
		t.Fatalf("captured %d times while frames still came", n)
	}
	waitFrames(t, out, name, append(want, stillFrame)...)
}

func TestAFrameDrawnDuringTheCaptureIsNotOverwrittenByIt(t *testing.T) {
	_, page, out, name := stillRig(t, stillFrame)
	page.mu.Lock()
	page.onStill = func() {
		page.mu.Lock()
		page.onStill = nil
		page.mu.Unlock()
		page.onFrame(stillTab, "bmV3")
	}
	page.mu.Unlock()
	page.onFrame(stillTab, jpegFrame)
	// The capture the newer frame lost to is dropped; the page is still
	// again after it, so the next one lands.
	waitFrames(t, out, name, jpegFrame, "bmV3", stillFrame)
	if n := stills(page); n != 2 {
		t.Fatalf("%d captures, want the dropped one and the one after", n)
	}
}

func TestAStillPastTheSocketLimitKeepsTheJPEG(t *testing.T) {
	_, page, out, name := stillRig(t, strings.Repeat("A", maxFrameData+1))
	jpeg := strings.Repeat("B", maxFrameData/2)
	page.onFrame(stillTab, jpeg)
	settled()
	if stills(page) != 1 {
		t.Fatal("the still page was not captured")
	}
	waitFrames(t, out, name, jpeg)
}

func TestAStillMuchHeavierThanItsJPEGKeepsTheJPEG(t *testing.T) {
	_, page, out, name := stillRig(t, strings.Repeat("A", stillGrowth*len(jpegFrame)+1))
	page.onFrame(stillTab, jpegFrame)
	settled()
	if stills(page) != 1 {
		t.Fatal("the still page was not captured")
	}
	waitFrames(t, out, name, jpegFrame)
}

func TestAStillUpToItsGrowthIsSent(t *testing.T) {
	still := strings.Repeat("A", stillGrowth*len(jpegFrame))
	_, page, out, name := stillRig(t, still)
	page.onFrame(stillTab, jpegFrame)
	waitFrames(t, out, name, jpegFrame, still)
}

func TestAStillOfAnotherTabThanTheFramesIsNotSent(t *testing.T) {
	_, page, out, name := stillRig(t, stillFrame)
	page.mu.Lock()
	page.stillTab = "t2"
	page.mu.Unlock()
	page.onFrame(stillTab, jpegFrame)
	settled()
	waitFrames(t, out, name, jpegFrame)
}

func TestAPageHoldingADialogIsNotStillSent(t *testing.T) {
	_, page, out, name := stillRig(t, "")
	page.mu.Lock()
	page.stillErr = errDialogOpen
	page.mu.Unlock()
	page.onFrame(stillTab, jpegFrame)
	settled()
	waitFrames(t, out, name, jpegFrame)
}

func TestADetachedPaneGetsNoStill(t *testing.T) {
	s, page, out, name := stillRig(t, stillFrame)
	page.onFrame(stillTab, jpegFrame)
	if err := s.Detach("s"); err != nil {
		t.Fatal(err)
	}
	settled()
	if n := stills(page); n != 0 {
		t.Fatalf("captured %d times with no pane drawing the page", n)
	}
	waitFrames(t, out, name, jpegFrame)
}

func TestAFrameEqualToTheLastIsNoChange(t *testing.T) {
	_, page, out, name := stillRig(t, stillFrame)
	page.mu.Lock()
	page.onStill = func() { page.onFrame(stillTab, jpegFrame) }
	page.mu.Unlock()
	page.onFrame(stillTab, jpegFrame)
	waitFrames(t, out, name, jpegFrame, stillFrame)
	settled()
	if n := stills(page); n != 1 {
		t.Fatalf("%d captures, want one: the frame the capture drew is the one before it", n)
	}
	waitFrames(t, out, name, jpegFrame, stillFrame)
}

func TestAPaneAttachedAgainGetsTheFrameItMayNotHold(t *testing.T) {
	s, page, out, name := stillRig(t, "")
	page.onFrame(stillTab, jpegFrame)
	if err := s.Detach("s"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 640, H: 480, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	page.onFrame(stillTab, jpegFrame)
	waitFrames(t, out, name, jpegFrame, jpegFrame)
}

// echoingRig is a page whose every capture draws a frame unlike any before
// it, during the capture or, when late, just after it.
func echoingRig(t *testing.T, late bool) (*fakePage, *emitted, string) {
	t.Helper()
	_, page, out, name := stillRig(t, stillFrame)
	n := 0
	page.mu.Lock()
	page.onStill = func() {
		n++
		frame := jpegFrame + strconv.Itoa(n)
		if !late {
			page.onFrame(stillTab, frame)
			return
		}
		go func() {
			time.Sleep(stillEcho / 5)
			page.onFrame(stillTab, frame)
		}()
	}
	page.mu.Unlock()
	return page, out, name
}

func TestACaptureDrawingANewFrameEachTimeStopsAtItsTries(t *testing.T) {
	for _, late := range []bool{false, true} {
		page, _, _ := echoingRig(t, late)
		page.onFrame(stillTab, jpegFrame)
		for range stillTries + 2 {
			settled()
		}
		if n := stills(page); n != stillTries {
			t.Fatalf("late=%v: %d captures of a still page, want %d", late, n, stillTries)
		}
	}
}

func TestAPageDrawingAgainAfterItsTriesIsCapturedAgain(t *testing.T) {
	page, _, _ := echoingRig(t, false)
	page.onFrame(stillTab, jpegFrame)
	for range stillTries + 1 {
		settled()
	}
	page.onFrame(stillTab, "bmV3")
	settled()
	if n := stills(page); n != stillTries+1 {
		t.Fatalf("%d captures, want %d and one for the page drawing again", n, stillTries+1)
	}
}
