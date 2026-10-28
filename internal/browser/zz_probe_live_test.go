package browser

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// Temporary probe: which CDP call of Screencast hangs on a held page.
func TestProbeHeldScreencastSteps(t *testing.T) {
	p, srv := liveChrome(t)
	frames := make(chan string, 8)
	opened := make(chan struct{}, 1)
	p.Listen(func(data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {}, func(d *api.BrowserDialog) {
		if d != nil {
			opened <- struct{}{}
		}
	})
	ctx := context.Background()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	step := func(name string, a chromedp.Action) {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		s := time.Now()
		err := p.runFree(c, a)
		t.Logf("PROBE %s: err=%v in %s", name, err, time.Since(s))
	}
	step("screenshot", chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.CaptureScreenshot().Do(ctx)
		return err
	}))
	step("evaluate", chromedp.Evaluate(`setTimeout(() => confirm('Delete?'), 0)`, nil))
	<-opened
	step("stop", page.StopScreencast())
	step("metrics", emulation.SetDeviceMetricsOverride(640, 400, 1, false))
	step("start", page.StartScreencast().WithFormat(page.ScreencastFormatJpeg).WithQuality(85).WithMaxWidth(640).WithMaxHeight(400))
	s := time.Now()
	select {
	case <-frames:
		t.Logf("PROBE frame in %s", time.Since(s))
	case <-time.After(5 * time.Second):
		t.Logf("PROBE no frame in 5s")
	}
	step("answer", page.HandleJavaScriptDialog(true))
	select {
	case <-frames:
		t.Logf("PROBE frame after answer in %s", time.Since(s))
	case <-time.After(5 * time.Second):
		t.Logf("PROBE no frame after answer")
	}
}
