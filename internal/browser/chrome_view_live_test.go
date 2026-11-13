package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// resizeCounter counts the resize events its document saw, the trace of
// Chrome laying the page out again.
const resizeCounter = `data:text/html,<script>window.n = 0; addEventListener("resize", () => window.n++)</script>`

// Chrome on Linux keeps the page whole after a screenshot, so the relayout
// itself is what is pinned: equal device metrics alone would change nothing.
func TestChromeFirstDocumentAfterAScreenshotIsLaidOutAgain(t *testing.T) {
	p, frames, ctx := castRig(t)
	if _, _, err := p.Screenshot(ctx, filepath.Join(t.TempDir(), "shot.png")); err != nil {
		t.Fatal(err)
	}
	if err := p.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for resizes(t, p, ctx) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the document after the screenshot was never laid out again")
		}
		time.Sleep(50 * time.Millisecond)
	}
	waitFrameSize(t, frames, 400, 600)
}

func TestChromeReloadWithoutAScreenshotIsNotLaidOutAgain(t *testing.T) {
	p, _, ctx := castRig(t)
	if err := p.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	// A relayout would land right after DOMContentLoaded, which the reload
	// already waited for.
	time.Sleep(time.Second)
	if n := resizes(t, p, ctx); n != 0 {
		t.Fatalf("%d resizes after a plain reload, want 0", n)
	}
}

func castRig(t *testing.T) (*chromePage, chan string, context.Context) {
	t.Helper()
	return castRigAt(t, 1)
}

// castRigAt is castRig with the pane at scale.
func castRigAt(t *testing.T, scale float64) (*chromePage, chan string, context.Context) {
	t.Helper()
	p, _ := liveChrome(t)
	frames := make(chan string, 64)
	p.Listen(func(_, data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {}, func(*api.BrowserDialog) {}, func([]api.BrowserTab) {})
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	t.Cleanup(cancel)
	if err := p.Navigate(ctx, resizeCounter); err != nil {
		t.Fatal(err)
	}
	if err := p.Screencast(ctx, Viewport{W: 400, H: 600, Scale: scale}); err != nil {
		t.Fatal(err)
	}
	return p, frames, ctx
}

func resizes(t *testing.T, p *chromePage, ctx context.Context) int {
	t.Helper()
	var n int
	if err := p.run(ctx, chromedp.Evaluate(`window.n`, &n)); err != nil {
		t.Fatal(err)
	}
	return n
}

// waitFrameSize waits for the frames to settle at w x h: the ones before can
// predate the last layout. A relaunch comes first for some, which took 2.3 s
// at the median on the macos-latest runner and 12 s once in 150, when the
// runner was slow to launch the browser at all (Chrome 156, measured
// 2026-11-13): the wait is a page call's, not ten seconds.
func waitFrameSize(t *testing.T, frames <-chan string, w, h int) {
	t.Helper()
	deadline := time.After(pageWait)
	last := "none"
	for {
		select {
		case data := <-frames:
			raw, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width == w && cfg.Height == h {
				return
			}
			last = fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
		case <-deadline:
			t.Fatalf("last frame %s, want %dx%d", last, w, h)
		}
	}
}

// A capture draws a frame. frameSink relies on that frame being the one before
// it, byte for byte: any other would start the settle over and capture again,
// for as long as the pane is open. A pane at a scale under the one the browser
// draws at gets one frame a pixel narrower after the first capture, and equal
// ones after it.
func TestChromeStillDrawsNoNewFrameAndMatchesTheFrames(t *testing.T) {
	p, frames, ctx := castRigAt(t, frameScale)
	last, size := quietFrames(t, frames)
	for range 5 {
		_, data, err := p.Still(ctx)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			t.Fatal(err)
		}
		still, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if still.Width != size.Width || still.Height != size.Height {
			t.Fatalf("still %dx%d, frames %dx%d", still.Width, still.Height, size.Width, size.Height)
		}
		select {
		case f := <-frames:
			if f != last {
				t.Fatal("the still capture drew a frame unlike the one before it")
			}
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// firstFrameWait bounds the wait for a screencast's first frame. Chrome on
// Linux sends it about 15 ms after the start; on the macos-latest runner it
// took 22 ms to 2 s (Chrome 156, 20 runs, measured 2026-11-13).
const firstFrameWait = 10 * time.Second

// quietFrames waits for the first frame, then until the page sends no frame
// for a second, and returns its last one and that one's size.
func quietFrames(t *testing.T, frames <-chan string) (string, image.Config) {
	t.Helper()
	var last string
	var size image.Config
	quiet := firstFrameWait
	deadline := time.After(firstFrameWait + 10*time.Second)
	for {
		select {
		case last = <-frames:
			raw, err := base64.StdEncoding.DecodeString(last)
			if err != nil {
				t.Fatal(err)
			}
			if size, err = jpeg.DecodeConfig(bytes.NewReader(raw)); err != nil {
				t.Fatal(err)
			}
			quiet = time.Second
		case <-time.After(quiet):
			if last == "" {
				t.Fatal("no frame came")
			}
			return last, size
		case <-deadline:
			t.Fatal("the page never stopped sending frames")
		}
	}
}

// pngPrefix is how a base64 PNG starts.
const pngPrefix = "iVBORw0KGgo"

func TestChromeAStillPaneGetsAPNGAndKeepsIt(t *testing.T) {
	frames := stillPane(t, frameScale)
	if n := countPNGs(frames); n != 1 || !strings.HasPrefix(frames[len(frames)-1].Data, pngPrefix) {
		t.Fatalf("frames %s; want one PNG, the last frame", describeFrames(frames))
	}
}

// A pane drawn under the browser's scale: the first capture's frame is a pixel
// narrower than the one before it, and on Apple silicon every capture's frame
// differs from the one before it (CI, macos-latest, 2026-11-12). The page gets
// no more captures than its tries and then stops, PNG or not.
func TestChromeAStillPaneUnderTheBrowsersScaleStopsCapturing(t *testing.T) {
	frames := stillPane(t, 1)
	if n := countPNGs(frames); n > stillTries {
		t.Fatalf("frames %s; want %d PNGs at most", describeFrames(frames), stillTries)
	}
}

// stillPane is the frames a still page sent a pane at scale until it sent
// none for two settles in a row.
func stillPane(t *testing.T, scale float64) []api.BrowserFrame {
	t.Helper()
	_, srv := liveChrome(t)
	s := newService(chromeLaunch, func(string, string) (string, error) {
		return t.TempDir(), nil
	}, time.Now, 5*time.Minute)
	t.Cleanup(func() { _ = s.Cleanup() })
	out := &emitted{}
	s.emit = out.emit
	if _, err := s.Open("s", srv.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 640, H: 480, Scale: scale}); err != nil {
		t.Fatal(err)
	}
	name := api.BrowserFrameEvents.Of("s").Name
	quiet := 2 * (stillAfter + stillEcho + stillWait/4)
	deadline := time.Now().Add(15 * time.Second)
	for n := -1; ; {
		time.Sleep(quiet)
		var frames []api.BrowserFrame
		for _, f := range out.named(name) {
			frames = append(frames, f.(api.BrowserFrame))
		}
		if len(frames) == n {
			return frames
		}
		if time.Now().After(deadline) {
			t.Fatalf("a still page kept sending frames: %s", describeFrames(frames))
		}
		n = len(frames)
	}
}

func countPNGs(frames []api.BrowserFrame) int {
	n := 0
	for _, f := range frames {
		if strings.HasPrefix(f.Data, pngPrefix) {
			n++
		}
	}
	return n
}

// describeFrames names each frame's format and size, what a failure on a
// runner nobody here can reach has to go on.
func describeFrames(frames []api.BrowserFrame) string {
	var b strings.Builder
	for _, f := range frames {
		raw, _ := base64.StdEncoding.DecodeString(f.Data)
		cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			format = err.Error()
		}
		fmt.Fprintf(&b, "[%s %dx%d %d B] ", format, cfg.Width, cfg.Height, len(f.Data))
	}
	return b.String()
}

// The probe page's field, clicked into from the pane, blinks its caret.
func TestChromeABlinkingCaretGetsNoStill(t *testing.T) {
	_, srv := liveChrome(t)
	s := newService(chromeLaunch, func(string, string) (string, error) {
		return t.TempDir(), nil
	}, time.Now, 5*time.Minute)
	t.Cleanup(func() { _ = s.Cleanup() })
	out := &emitted{}
	s.emit = out.emit
	if _, err := s.Open("s", srv.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach("s", Viewport{W: 640, H: 480, Scale: frameScale}); err != nil {
		t.Fatal(err)
	}
	if err := s.current("s").page.Focus(nil); err != nil {
		t.Fatal(err)
	}
	for _, ty := range []string{"press", "release"} {
		if _, err := s.MouseInput(context.Background(), api.BrowserMouseOptions{ID: "s", Type: ty, X: 100, Y: 25, Button: "left", Clicks: 1}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(time.Second)
	name := api.BrowserFrameEvents.Of("s").Name
	before := len(out.named(name))
	time.Sleep(3 * time.Second)
	after := out.named(name)[before:]
	if len(after) < 3 {
		t.Fatalf("%d frames in 3 s: the caret is not blinking, and the test proves nothing", len(after))
	}
	for _, f := range after {
		if strings.HasPrefix(f.(api.BrowserFrame).Data, pngPrefix) {
			t.Fatal("a blink of the caret was captured as a still")
		}
	}
}

// A caret frame late past stillAfter, which a loaded runner sends, must not
// get its PNG either: Still itself refuses while a caret blinks.
func TestChromeStillRefusesWhileACaretBlinks(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	// A page whose window has no focus blinks no caret.
	if err := p.Focus(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Still(ctx); err != nil {
		t.Fatalf("Still with no caret = %v, want a capture", err)
	}
	if err := p.run(ctx, chromedp.Evaluate(`document.getElementById("f").focus()`, nil)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Still(ctx); !errors.Is(err, errCaret) {
		t.Fatalf("Still with a caret in the field = %v, want errCaret", err)
	}
	if err := p.run(ctx, chromedp.Evaluate(`(() => { const f = document.getElementById("f"); f.value = "abc"; f.select() })()`, nil)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Still(ctx); err != nil {
		t.Fatalf("Still with the field's text selected = %v, want a capture", err)
	}
}
