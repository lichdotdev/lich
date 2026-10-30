package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/internal/chromium"
)

// domSettle is how long a navigation waits for the document to become
// interactive. chromedp.Navigate waits for the load event instead, which is
// every image and tracker: the page is already on screen and the tool is
// still held, up to pageWait. Past this, the agent reads whatever is there.
const domSettle = 1500 * time.Millisecond

// closeWait bounds a graceful Browser.close: long enough for Chrome to flush
// the profile a promotion reopens, short enough that a hung one is killed.
const closeWait = 5 * time.Second

// launchWait bounds how long a starting Chrome may take to print its DevTools
// URL. chromedp's default of 20s is shorter than a cold start: on a GitHub
// Ubuntu runner the first launch of the suite passed 20s, the second took 6s
// and the third 1s, each before Chrome's own sandbox check, so the time goes to
// loading the browser off a cold disk, which a slow machine pays too.
const launchWait = 60 * time.Second

// chromeLaunch starts a Chromium-family browser via CDP. ExecPath is
// chromium.FindBrowser (system Chrome/Edge/… — never lich-shell); UserDataDir
// is this ephemeral dir, never lich/chromium-profile. Attaching to the UI
// window would expose the page and the session token; this process must stay
// a sidecar.
func chromeLaunch(dir string, headed bool) (Page, error) {
	exe, err := chromium.FindBrowser(exec.LookPath)
	if err != nil {
		return nil, err
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(exe), chromedp.UserDataDir(dir), chromedp.WSURLReadTimeout(launchWait),
		chromedp.Flag("disable-features", disabledFeatures))
	if headed {
		opts = append(opts, chromedp.Flag("headless", false))
	} else {
		opts = append(opts, chromedp.Flag("force-device-scale-factor", strconv.FormatFloat(frameScale, 'f', -1, 64)))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx, interceptFiles); err != nil {
		cancel()
		allocCancel()
		return nil, err
	}
	p := &chromePage{ctx: ctx, cancel: cancel, allocCancel: allocCancel}
	p.tabs.start(&tab{ctx: ctx, id: chromedp.FromContext(ctx).Target.TargetID})
	if err := p.allowDownloads(dir); err != nil {
		slog.Warn("browser: downloads stay off", "err", err)
	}
	return p, nil
}

// frameScale is the scale a headless browser draws at, forced at its launch,
// and the most a pane's frames get. The screencast copies the browser's own
// surface, which an emulated scale does not change: a 2x pane got 1x frames,
// blurred. A busy page forced to 2 and drawn at 2x ran at 28 fps against 45 at
// 1x, and at 33 drawn at 1.5x (measured 2026-10-29): 1.5 trades sharpness
// for frame rate.
const frameScale = 1.5

// disabledFeatures replaces chromedp's own disable-features list, which it
// repeats, adding the optimization guide's model downloads. Those go through
// the profile's download manager with no page and no download event, so with
// downloads allowed Chrome saved them into the download folder as files named
// by a GUID, 4 KB to 33 MB each (Chrome on the Windows and macOS CI runners,
// 2026-10-27). They stay in the profile's staging folder now; these spare
// the bytes.
const disabledFeatures = "site-per-process,Translate,BlinkGenPropertyTrees," +
	"OptimizationGuideModelDownloading,OptimizationTargetPrediction"

type chromePage struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
	listen      listeners
	tabs        tabStack
	// dl names the files the page downloads; nil when downloads are off.
	dl *downloads
	// gate is held while the page has a JavaScript dialog open.
	gate dialogGate
	// castMu guards castLast, the last screencast start or stop queued, which
	// the next one waits for: they can run on past their caller
	// (castPastDialog). casting says one is running.
	castMu   sync.Mutex
	castLast chan struct{}
	casting  bool
	// leaveMu guards held, the navigation the open beforeunload holds, and
	// leave, the one an accepted beforeunload left for (chrome_leave.go).
	leaveMu sync.Mutex
	held    *leave
	leave   *leave
}

// downloadFolder is downloadDir, which the live tests point at a folder of
// their own: on Windows, Chrome would not start with USERPROFILE moved.
var downloadFolder = downloadDir

// allowDownloads saves what the page downloads into the user's Downloads
// folder, with events, by way of a staging folder in the profile dir. A
// headless Chrome denies downloads by default, and a click on a download
// button did nothing.
func (p *chromePage) allowDownloads(profile string) error {
	dir, err := downloadFolder()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	staging := filepath.Join(profile, stagingDir)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	c := chromedp.FromContext(p.ctx)
	err = p.runFree(nil, chromedp.ActionFunc(func(ctx context.Context) error {
		return browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(staging).
			WithEventsEnabled(true).
			Do(cdp.WithExecutor(ctx, c.Browser))
	}))
	if err != nil {
		return err
	}
	p.dl = newDownloads(staging, dir)
	return nil
}

func (p *chromePage) Alive() bool { return p.ctx.Err() == nil }

// Close asks Chrome to quit instead of killing it, so cookies and storage
// reach the profile on disk before promote relaunches on it.
func (p *chromePage) Close() error {
	defer p.allocCancel()
	defer p.cancel()
	if p.ctx.Err() != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(p.ctx, closeWait)
	defer cancel()
	return chromedp.Cancel(ctx)
}

// run is runFree for an action the page's script or input takes part in,
// which waits on an open JavaScript dialog: it fails with errDialogOpen at
// once while one is open, and as soon as one opens while it runs.
func (p *chromePage) run(wait context.Context, actions ...chromedp.Action) error {
	held := p.gate.held()
	select {
	case <-held:
		return errDialogOpen
	default:
	}
	ctx, cancel := withPageDeadline(p.tabs.frontTab().ctx, wait)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- chromedp.Run(ctx, actions...) }()
	return awaitAction(done, held)
}

// dialogGrace is how long a call whose page just opened a dialog still waits
// for its own result. The reply and the dialog's event reach chromedp on two
// tabs' goroutines, so a call that finished before a timer opened an alert
// lost to it: on every Windows CI run, never on Linux (2026-10-29).
const dialogGrace = 250 * time.Millisecond

// awaitAction returns the call's result, or errDialogOpen when a dialog holds
// it past dialogGrace.
func awaitAction(done <-chan error, held <-chan struct{}) error {
	select {
	case err := <-done:
		return err
	case <-held:
	}
	select {
	case err := <-done:
		return err
	case <-time.After(dialogGrace):
		return errDialogOpen
	}
}

// runFree drives CDP on the front tab's context chromedp attached, applying
// wait's deadline. Service callers pass a timeout derived from Background;
// chromedp.Run on that context is "invalid context" and the headed window is
// then closed.
func (p *chromePage) runFree(wait context.Context, actions ...chromedp.Action) error {
	ctx, cancel := withPageDeadline(p.tabs.frontTab().ctx, wait)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

func (p *chromePage) Navigate(ctx context.Context, url string) error {
	return p.run(ctx, settleAfter(func(ctx context.Context) error {
		_, _, errorText, _, err := page.Navigate(url).Do(ctx)
		if err != nil {
			return err
		}
		if errorText == abortedText {
			return errNavAborted
		}
		if errorText != "" {
			return fmt.Errorf("page load error %s", errorText)
		}
		return nil
	}))
}

// abortedText is Chrome's answer to a navigation that did not load a page,
// among them one that became a download.
const abortedText = "net::ERR_ABORTED"

var errNavAborted = errors.New("page load error " + abortedText)

// navGrace is how long after an action a navigation it starts may take to
// replace the document: until then the old one reads as complete.
const navGrace = 150 * time.Millisecond

func (p *chromePage) Settle(ctx context.Context) error {
	p.awaitLeave(ctx)
	return p.run(ctx, settleAfter(func(ctx context.Context) error {
		t := time.NewTimer(navGrace)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}))
}

func (p *chromePage) Info(ctx context.Context) (PageInfo, error) {
	var info PageInfo
	if err := p.run(ctx, chromedp.Evaluate(pageInfoScript, &info)); err != nil {
		return PageInfo{}, err
	}
	return info, nil
}

func (p *chromePage) HasText(ctx context.Context, text string) (bool, error) {
	literal, err := json.Marshal(text)
	if err != nil {
		return false, err
	}
	var has bool
	script := fmt.Sprintf(hasTextScript, literal)
	return has, p.run(ctx, chromedp.Evaluate(script, &has))
}

func (p *chromePage) Click(ctx context.Context, t Target) (x, y float64, err error) {
	pt, err := p.resolve(ctx, t)
	if err != nil {
		return 0, 0, err
	}
	return pt.X, pt.Y, p.run(ctx, chromedp.MouseClickXY(pt.X, pt.Y))
}

func (p *chromePage) Type(ctx context.Context, text string, clear bool, t Target) error {
	if !t.empty() {
		if _, _, err := p.Click(ctx, t); err != nil {
			return err
		}
	}
	if clear {
		// Assigning el.value fires no input event, so a framework that owns the
		// field keeps its old value; a delete through the editing commands does.
		err := p.run(ctx, chromedp.Evaluate(`(() => {
			const el = document.activeElement;
			if (!el) return;
			if (typeof el.select === 'function') el.select();
			else document.execCommand('selectAll');
			document.execCommand('delete');
		})()`, nil))
		if err != nil {
			return err
		}
	}
	return p.run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.InsertText(text).Do(ctx)
	}))
}

func (p *chromePage) Press(ctx context.Context, key string, t Target) error {
	seq, err := keySequence(key)
	if err != nil {
		return err
	}
	if !t.empty() {
		if _, _, err := p.Click(ctx, t); err != nil {
			return err
		}
	}
	return p.run(ctx, chromedp.KeyEvent(seq))
}

// viewportJS is the viewport in CSS pixels, where the document is scrolled
// to: a screenshot's clip is in the document's coordinates.
const viewportJS = `({x: visualViewport.pageLeft, y: visualViewport.pageTop, w: innerWidth, h: innerHeight, dpr: devicePixelRatio})`

// Screenshot clips at 1/devicePixelRatio: at the device scale, a pane on a
// HiDPI monitor or at a page zoom gave an image up to 1.5x the viewport, and
// every point an agent read off it missed by as much.
func (p *chromePage) Screenshot(ctx context.Context, dest string) (w, h int, err error) {
	var v struct {
		X   float64 `json:"x"`
		Y   float64 `json:"y"`
		W   float64 `json:"w"`
		H   float64 `json:"h"`
		DPR float64 `json:"dpr"`
	}
	if err := p.run(ctx, chromedp.Evaluate(viewportJS, &v)); err != nil {
		return 0, 0, err
	}
	var buf []byte
	err = p.run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		buf, err = page.CaptureScreenshot().
			WithClip(&page.Viewport{X: v.X, Y: v.Y, Width: v.W, Height: v.H, Scale: 1 / v.DPR}).
			Do(ctx)
		return err
	}))
	if err != nil {
		return 0, 0, err
	}
	return int(v.W), int(v.H), os.WriteFile(dest, buf, 0o600)
}

func (p *chromePage) Reload(ctx context.Context) error {
	return p.run(ctx, settleAfter(func(ctx context.Context) error {
		return page.Reload().Do(ctx)
	}))
}

func (p *chromePage) Back(ctx context.Context) error {
	return p.run(ctx, history(-1))
}

func (p *chromePage) Forward(ctx context.Context) error {
	return p.run(ctx, history(1))
}

// history moves one step. chromedp.NavigateBack waits for the load event, the
// same wait Navigate used to.
func history(delta int) chromedp.Action {
	return settleAfter(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}
		next := int(cur) + delta
		if next < 0 || next >= len(entries) {
			return fmt.Errorf("no history in that direction")
		}
		return page.NavigateToHistoryEntry(entries[next].ID).Do(ctx)
	})
}

// settleAfter runs goThere, then returns once the document is interactive or
// domSettle has passed, whichever comes first.
func settleAfter(goThere func(context.Context) error) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := goThere(ctx); err != nil {
			return err
		}
		deadline := time.Now().Add(domSettle)
		for {
			var state string
			if err := chromedp.Evaluate(`document.readyState`, &state).Do(ctx); err == nil &&
				(state == "interactive" || state == "complete") {
				return nil
			}
			if time.Now().After(deadline) {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			t := time.NewTimer(40 * time.Millisecond)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	})
}

// Scroll is a real wheel turn rather than window.scrollBy, so it scrolls what
// is under the point (a list with its own scroll) as a person's would.
func (p *chromePage) Scroll(ctx context.Context, at *point, dy int) (x, y float64, err error) {
	if at == nil {
		at = new(point)
		if err := p.run(ctx, chromedp.Evaluate(`({x: innerWidth / 2, y: innerHeight / 2})`, at)); err != nil {
			return 0, 0, err
		}
	}
	wheel := input.DispatchMouseEvent(input.MouseWheel, at.X, at.Y).WithDeltaY(float64(dy))
	return at.X, at.Y, p.run(ctx, wheel)
}

// Focus raises the window. window.focus() from the page does not: the OS
// only lets the browser itself bring a window forward.
func (p *chromePage) Focus(ctx context.Context) error {
	return p.runFree(ctx, page.BringToFront())
}

func (p *chromePage) Cookies(ctx context.Context) ([]*network.CookieParam, error) {
	var cookies []*network.Cookie
	err := p.runFree(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		cookies, err = storage.GetCookies().Do(ctx)
		return err
	}))
	if err != nil {
		return nil, err
	}
	out := make([]*network.CookieParam, 0, len(cookies))
	for _, c := range cookies {
		out = append(out, cookieParam(c))
	}
	return out, nil
}

func (p *chromePage) SetCookies(ctx context.Context, cookies []*network.CookieParam) error {
	return p.runFree(ctx, storage.SetCookies(cookies))
}

// cookieParam turns a cookie read back into one Chrome accepts. A session
// cookie keeps no expiry, so it stays a session cookie in the new process.
func cookieParam(c *network.Cookie) *network.CookieParam {
	param := &network.CookieParam{
		Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path,
		Secure: c.Secure, HTTPOnly: c.HTTPOnly, SameSite: c.SameSite, Priority: c.Priority,
		SourceScheme: c.SourceScheme, SourcePort: c.SourcePort, PartitionKey: c.PartitionKey,
	}
	if !c.Session {
		expires := cdp.TimeSinceEpoch(time.Unix(0, int64(c.Expires*float64(time.Second))))
		param.Expires = &expires
	}
	return param
}

// withPageDeadline hangs wait's timeout off the chromedp page context. A
// timeout from context.Background has no CDP target; wrapping it the other
// way around is how OpenVisible returned "invalid context" and closed the
// window it had just opened.
func withPageDeadline(pageCtx, waitCtx context.Context) (context.Context, context.CancelFunc) {
	timeout := pageWait
	if waitCtx != nil {
		if d, ok := waitCtx.Deadline(); ok {
			timeout = time.Until(d)
			if timeout < 0 {
				timeout = 0
			}
		}
	}
	return context.WithTimeout(pageCtx, timeout)
}

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// resolveJS finds a click's point in the viewport. An element out of view,
// which typing into a form scrolls the save button to, is scrolled to the
// middle first: a click at its old box lands outside the page and hits
// nothing. A point given outside the viewport is reported, not clicked, and
// so is one read off a screenshot of another viewport size: the person resized
// the pane, moved the window to another monitor or zoomed the page since, and
// the page laid out again under it.
const resolveJS = `(function(index, selector, x, y, shotW, shotH) {
  var inView = function(px, py) { return px >= 0 && py >= 0 && px < innerWidth && py < innerHeight; };
  if (typeof x === 'number' && typeof y === 'number') {
    if (shotW > 0 && (shotW !== innerWidth || shotH !== innerHeight)) return {x: x, y: y, resized: true, w: innerWidth, h: innerHeight};
    if (!inView(x, y)) return {x: x, y: y, outside: true, w: innerWidth, h: innerHeight};
    return {x: x, y: y};
  }
  var el = null;
  if (selector) el = document.querySelector(selector);
  else if (typeof index === 'number') {
    el = document.querySelector('[data-lich-idx="' + index + '"]');
  }
  if (!el) return null;
  var r = el.getBoundingClientRect();
  if (!inView(r.left + r.width / 2, r.top + r.height / 2)) {
    el.scrollIntoView({block: 'center', inline: 'center', behavior: 'instant'});
    r = el.getBoundingClientRect();
  }
  return {x: r.left + r.width / 2, y: r.top + r.height / 2};
})`

// resolved is resolveJS's answer: the point, or a point outside the
// viewport of w by h, or read off a screenshot of another size than w by h.
type resolved struct {
	point
	Outside bool    `json:"outside"`
	Resized bool    `json:"resized"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
}

func (p *chromePage) resolve(ctx context.Context, t Target) (point, error) {
	args, err := json.Marshal([]any{t.Index, t.Selector, t.X, t.Y, t.shotW, t.shotH})
	if err != nil {
		return point{}, err
	}
	expr := resolveJS + "(" + string(args[1:len(args)-1]) + ")"
	var r *resolved
	if err := p.run(ctx, chromedp.Evaluate(expr, &r)); err != nil {
		return point{}, err
	}
	if r == nil {
		return point{}, fmt.Errorf("no element matches that target: the page may have changed since it was read; read it again for fresh indexes")
	}
	if r.Resized {
		return point{}, fmt.Errorf("the viewport is %gx%g, and your last screenshot was %dx%d: the page was resized since; take a new screenshot before clicking at x and y",
			r.W, r.H, t.shotW, t.shotH)
	}
	if r.Outside {
		return point{}, fmt.Errorf("%g,%g is outside the %gx%g viewport: scroll first, or click by index or selector, which scroll the element into view",
			r.X, r.Y, r.W, r.H)
	}
	return r.point, nil
}
