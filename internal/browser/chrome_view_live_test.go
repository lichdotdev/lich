package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"path/filepath"
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
	if err := p.Screencast(ctx, Viewport{W: 400, H: 600, Scale: 1}); err != nil {
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
