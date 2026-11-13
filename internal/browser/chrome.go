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
	"sync/atomic"
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
func chromeLaunch(dir string, headed bool, scale float64) (Page, error) {
	exe, err := chromium.FindBrowser(exec.LookPath)
	if err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(exe), chromedp.UserDataDir(dir), chromedp.WSURLReadTimeout(launchWait),
		chromedp.Flag("disable-features", disabledFeatures), chromedp.Flag("remote-debugging-port", port),
		chromedp.Flag("remote-allow-origins", devtoolsOrigin(port)))
	if headed {
		opts = append(opts, chromedp.Flag("headless", false))
	} else {
		opts = append(opts, chromedp.Flag("force-device-scale-factor", strconv.FormatFloat(scale, 'f', -1, 64)),
			chromedp.Flag("blink-settings", mousePointer))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx, interceptFiles); err != nil {
		cancel()
		allocCancel()
		return nil, err
	}
	p := &chromePage{ctx: ctx, cancel: cancel, allocCancel: allocCancel, scale: scale, port: port}
	p.tabs.start(&tab{ctx: ctx, id: chromedp.FromContext(ctx).Target.TargetID})
	if err := p.allowDownloads(dir); err != nil {
		slog.Warn("browser: downloads stay off", "err", err)
	}
	return p, nil
}

// frameScale is the most a headless browser draws at, and so the most a pane's
// frames get; minFrameScale is the least. The scale is forced at the launch:
// the screencast copies the browser's own surface, which an emulated scale
// does not change. A busy page forced to 2 and drawn at 2x ran at 28 fps
// against 45 at 1x, and at 33 drawn at 1.5x (measured 2026-10-29): 1.5 trades
// sharpness for frame rate.
const (
	frameScale    = 1.5
	minFrameScale = 1
)

// launchScale is the scale a headless browser launched for a window drawing
// pages at scale draws at: the window's own, so the frames are the pane's
// pixels. A browser forced to 1.5 under a 1x pane reduced every frame to the
// pane's size, which blurred its text and sent a 1140x1300 pane 28 to 38
// frames a second where one drawn at 1x sent 49 to 60 (measured 2026-11-11).
// With no window heard from yet it is frameScale.
func launchScale(scale float64) float64 {
	if scale <= 0 {
		return frameScale
	}
	return min(max(scale, minFrameScale), frameScale)
}

// disabledFeatures replaces chromedp's own disable-features list, which it
// repeats, adding the optimization guide's model downloads. Those go through
// the profile's download manager with no page and no download event, so with
// downloads allowed Chrome saved them into the download folder as files named
// by a GUID, 4 KB to 33 MB each (Chrome on the Windows and macOS CI runners,
// 2026-10-27). They stay in the profile's staging folder now; these spare
// the bytes.
const disabledFeatures = "site-per-process,Translate,BlinkGenPropertyTrees," +
	"OptimizationGuideModelDownloading,OptimizationTargetPrediction"

// mousePointer tells the page it has a mouse. Headless Chrome finds no input
// device and answers (hover: none) and (pointer: none), so a page that asks
// before wiring its hover effects ignored the pointer the pane passes it. The
// values are Blink's HoverType and PointerType bits: 2 is hover, 4 is fine.
const mousePointer = "primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4"

type chromePage struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
	// scale is what the browser draws at, forced at its launch.
	scale float64
	// port is the browser's debugging port, which DevTools reaches it on.
	port   string
	listen listeners
	tabs   tabStack
	// dl names the files the page downloads; nil when downloads are off.
	dl *downloads
	// gate is held while the page has a JavaScript dialog open; pause while
	// a tab is stopped in its DevTools debugger.
	gate, pause dialogGate
	// castMu guards castLast, the last screencast start or stop queued, which
	// the next one waits for: they can run on past their caller
	// (castPastDialog). casting says one is running, on the target of castOn.
	castMu   sync.Mutex
	castLast chan struct{}
	casting  bool
	castOn   context.Context
	// shot: an agent's screenshot ran since the frames were last laid out
	// again after one (relayAfterShot).
	shot atomic.Bool
	// annotating counts the agent's labelled screenshots running, which Still
	// waits out.
	annotating atomic.Int32
	// leaveMu guards held, the navigation the open beforeunload holds, and
	// leave, the one an accepted beforeunload left for (chrome_leave.go).
	leaveMu sync.Mutex
	held    *leave
	leave   *leave
	// idxMu guards idxNext, the index pageInfoScript hands out next.
	idxMu   sync.Mutex
	idxNext int
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
// once while one is open, and as soon as one opens while it runs. A page
// stopped in its debugger fails it with errPaused the same way: it still
// answered a script, with the document as it stood, and took a click or a key
// as done and lost it (Chromium 157, measured 2026-11-12).
func (p *chromePage) run(wait context.Context, actions ...chromedp.Action) error {
	return p.runPast(wait, p.pause.held(), actions...)
}

// runLayout is run for the page's layout, which a pause does not hold.
func (p *chromePage) runLayout(wait context.Context, actions ...chromedp.Action) error {
	return p.runPast(wait, nil, actions...)
}

func (p *chromePage) runPast(wait context.Context, paused <-chan struct{}, actions ...chromedp.Action) error {
	held := p.gate.held()
	select {
	case <-held:
		return errDialogOpen
	case <-paused:
		return errPaused
	default:
	}
	ctx, cancel := withPageDeadline(p.tabs.frontTab().ctx, wait)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- chromedp.Run(ctx, actions...) }()
	return awaitAction(done, held, paused)
}

// dialogGrace is how long a call whose page just opened a dialog still waits
// for its own result. The reply and the dialog's event reach chromedp on two
// tabs' goroutines, so a call that finished before a timer opened an alert
// lost to it: on every Windows CI run, never on Linux (2026-10-29).
const dialogGrace = 250 * time.Millisecond

// awaitAction returns the call's result, or errDialogOpen when a dialog holds
// it past dialogGrace, or errPaused when the debugger does.
func awaitAction(done <-chan error, held, paused <-chan struct{}) error {
	why := errDialogOpen
	select {
	case err := <-done:
		return err
	case <-held:
	case <-paused:
		why = errPaused
	}
	select {
	case err := <-done:
		return err
	case <-time.After(dialogGrace):
		return why
	}
}

// runFree drives CDP on the front tab's context chromedp attached, applying
// wait's deadline. Service callers pass a timeout derived from Background;
// chromedp.Run on that context is "invalid context" and the headed window is
// then closed.
func (p *chromePage) runFree(wait context.Context, actions ...chromedp.Action) error {
	return runOn(p.tabs.frontTab().ctx, wait, actions...)
}

// runOn is runFree on the target of the chromedp context target.
func runOn(target, wait context.Context, actions ...chromedp.Action) error {
	ctx, cancel := withPageDeadline(target, wait)
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

func (p *chromePage) IndexNext() int {
	p.idxMu.Lock()
	defer p.idxMu.Unlock()
	return p.idxNext
}

func (p *chromePage) SetIndexNext(n int) {
	p.idxMu.Lock()
	defer p.idxMu.Unlock()
	p.idxNext = n
}

func (p *chromePage) Info(ctx context.Context, within string) (PageInfo, error) {
	arg, err := json.Marshal(within)
	if err != nil {
		return PageInfo{}, err
	}
	p.idxMu.Lock()
	base := p.idxNext
	p.idxMu.Unlock()
	var info struct {
		PageInfo
		Next int `json:"next"`
	}
	if err := p.run(ctx, chromedp.Evaluate(pageInfoScript+"("+strconv.Itoa(base)+", "+string(arg)+")", &info)); err != nil {
		return PageInfo{}, err
	}
	if info.Error != "" {
		return PageInfo{}, errors.New(info.Error)
	}
	p.idxMu.Lock()
	p.idxNext = max(p.idxNext, info.Next)
	p.idxMu.Unlock()
	return info.PageInfo, nil
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

// shotMax is the longest side of the image a screenshot writes. The agent
// reads it as an image, priced by its pixels: a 2000x1626 pane's came to
// about 2,350 tokens, more than three page reads (measured 2026-11-07).
const shotMax = 1280

// shotScale is the image pixels per CSS pixel of a w by h viewport's
// screenshot, which the x and y read off it are divided by.
func shotScale(w, h int) float64 {
	if long := max(w, h); long > shotMax {
		return float64(shotMax) / float64(long)
	}
	return 1
}

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
			WithClip(&page.Viewport{X: v.X, Y: v.Y, Width: v.W, Height: v.H, Scale: shotScale(int(v.W), int(v.H)) / v.DPR}).
			Do(ctx)
		return err
	}))
	if err != nil {
		return 0, 0, err
	}
	p.shot.Store(true)
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
// the page laid out again under it. x and y are the image's pixels, scale
// per CSS pixel, and an outside point is reported in them.
const resolveJS = `(function(index, selector, x, y, shotW, shotH, scale) {
  var inView = function(px, py) { return px >= 0 && py >= 0 && px < innerWidth && py < innerHeight; };
  if (typeof x === 'number' && typeof y === 'number') {
    if (shotW > 0 && (shotW !== innerWidth || shotH !== innerHeight)) return {x: x, y: y, resized: true, w: innerWidth, h: innerHeight};
    if (!inView(x / scale, y / scale)) return {x: x, y: y, outside: true, w: Math.round(innerWidth * scale), h: Math.round(innerHeight * scale)};
    return {x: x / scale, y: y / scale};
  }
  var el = null;
  if (selector) el = document.querySelector(selector);
  else if (typeof index === 'number') {
    var reg = window.__lichIdx;
    el = Array.from(document.querySelectorAll('[data-lich-idx="' + index + '"]')).find(function(e) {
      return reg && reg.of.get(e) === index && e.getClientRects().length > 0;
    }) || null;
  }
  if (!el) return null;
  if (el.matches(':disabled') || el.getAttribute('aria-disabled') === 'true') return {disabled: true};
  // An inline element wrapped over lines has a bounding box whose center can be
  // on neither line, so it is clicked on its first non-empty line box instead.
  var box = function() {
    var rs = Array.from(el.getClientRects());
    return rs.length > 1 && rs.find(function(b) { return b.width > 0 && b.height > 0; }) || el.getBoundingClientRect();
  };
  var r = box();
  if (!inView(r.left + r.width / 2, r.top + r.height / 2)) {
    el.scrollIntoView({block: 'center', inline: 'center', behavior: 'instant'});
    r = box();
  }
  var cx = r.left + r.width / 2, cy = r.top + r.height / 2;
  var hit = document.elementFromPoint(cx, cy);
  // A label's click reaches its control, which is how a custom checkbox hides
  // the real input under a styled sibling.
  var lbl = hit && hit.closest('label');
  if (hit && hit !== el && !el.contains(hit) && !(lbl && lbl.control === el)) {
    var cls = typeof hit.className === 'string' ? hit.className.trim().split(/\s+/)[0] : '';
    return {x: cx, y: cy, covered: hit.tagName.toLowerCase() + (hit.id ? '#' + hit.id : cls ? '.' + cls : '')};
  }
  return {x: cx, y: cy};
})`

// resolved is resolveJS's answer: the point, or a point outside the
// viewport of w by h, or read off a screenshot of another size than w by h,
// or an element that is disabled or covered by another at its click point.
type resolved struct {
	point
	Disabled bool    `json:"disabled"`
	Covered  string  `json:"covered"`
	Outside  bool    `json:"outside"`
	Resized  bool    `json:"resized"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
}

func (p *chromePage) resolve(ctx context.Context, t Target) (point, error) {
	args, err := json.Marshal([]any{t.Index, t.Selector, t.X, t.Y, t.shotW, t.shotH, shotScale(t.shotW, t.shotH)})
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
	if r.Disabled {
		return point{}, fmt.Errorf("the element is disabled: a click on it does nothing; read the page again to see what enables it")
	}
	if r.Covered != "" {
		return point{}, coveredError(r.Covered)
	}
	if r.Resized {
		return point{}, fmt.Errorf("the viewport is %gx%g, and was %dx%d at your last screenshot: the page was resized since; take a new screenshot before clicking at x and y",
			r.W, r.H, t.shotW, t.shotH)
	}
	if r.Outside {
		return point{}, fmt.Errorf("%g,%g is outside the %gx%g viewport: scroll first, or click by index or selector, which scroll the element into view",
			r.X, r.Y, r.W, r.H)
	}
	return r.point, nil
}

func coveredError(by string) error {
	return fmt.Errorf("the element is covered by <%s> at its click point: dismiss that first, then read the page again", by)
}
