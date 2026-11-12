package browser

import (
	"bytes"
	"context"
	"encoding/base64"
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
// predate the last layout.
func waitFrameSize(t *testing.T, frames <-chan string, w, h int) {
	t.Helper()
	deadline := time.After(10 * time.Second)
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

// quietFrames waits until the page sends no frame for a second and returns
// its last one and that one's size.
func quietFrames(t *testing.T, frames <-chan string) (string, image.Config) {
	t.Helper()
	var last string
	var size image.Config
	deadline := time.After(10 * time.Second)
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
		case <-time.After(time.Second):
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

// The pane draws at scale 1 on a browser drawing at 1.5, where the first
// capture's frame is a pixel narrower than the one before it and so settles
// into one more capture.
func TestChromeAStillPaneGetsAPNGAndKeepsIt(t *testing.T) {
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
	if _, err := s.Attach("s", Viewport{W: 640, H: 480, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(4 * time.Second)
	frames := out.named(api.BrowserFrameEvents.Of("s").Name)
	stills := 0
	for _, f := range frames {
		if strings.HasPrefix(f.(api.BrowserFrame).Data, pngPrefix) {
			stills++
		}
	}
	if stills == 0 || stills > 2 || !strings.HasPrefix(frames[len(frames)-1].(api.BrowserFrame).Data, pngPrefix) {
		t.Fatalf("%d frames, %d of them PNG; want one or two PNGs, the last frame one", len(frames), stills)
	}
}
