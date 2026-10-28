package browser

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// Temporary probe: why Page.stopScreencast hangs on a held page.
func TestProbeHeldScreencastSteps(t *testing.T) {
	for _, idle := range []time.Duration{0, 2 * time.Second} {
		t.Run(fmt.Sprint("idle", idle), func(t *testing.T) { probe(t, idle) })
	}
}

func probe(t *testing.T, idle time.Duration) {
	p, srv := liveChrome(t)
	t0 := time.Now()
	logf := func(f string, a ...any) { t.Logf("PROBE %6dms "+f, append([]any{time.Since(t0).Milliseconds()}, a...)...) }
	opened := make(chan struct{}, 1)
	p.Listen(func(string) {}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {}, func(d *api.BrowserDialog) {
		if d != nil {
			opened <- struct{}{}
		}
	})
	chromedp.ListenTarget(p.ctx, func(ev any) { logf("event %T", ev) })
	ctx := context.Background()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	logf("navigated")
	step := func(name string, d time.Duration, a chromedp.Action) {
		c, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		s := time.Now()
		err := p.runFree(c, a)
		logf("%s: err=%v in %s", name, err, time.Since(s))
	}
	step("screenshot", 10*time.Second, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.CaptureScreenshot().Do(ctx)
		return err
	}))
	time.Sleep(idle)
	step("evaluate", 10*time.Second, chromedp.Evaluate(`setTimeout(() => confirm('Delete?'), 0)`, nil))
	<-opened
	logf("dialog open")
	go step("stop", 40*time.Second, page.StopScreencast())
	time.Sleep(2 * time.Second)
	step("history", 3*time.Second, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, err := page.GetNavigationHistory().Do(ctx)
		return err
	}))
	c := chromedp.FromContext(p.ctx)
	step("version", 3*time.Second, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, _, _, _, err := browser.GetVersion().Do(cdp.WithExecutor(ctx, c.Browser))
		return err
	}))
	step("answer", 5*time.Second, page.HandleJavaScriptDialog(true))
	time.Sleep(3 * time.Second)
}
