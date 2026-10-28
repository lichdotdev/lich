package browser

import (
	"context"
	"log/slog"
	"math"
	"sync"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// screencastQuality is the JPEG quality of a pane's frames: text without
// visible ringing at a fraction of PNG's size, since every frame crosses
// /events as base64.
const screencastQuality = 85

// listeners are what a chromePage hands its CDP events to; the service sets
// them once, right after the launch.
type listeners struct {
	mu sync.Mutex
	// taken are the tabs this page opened that keepOneTab already took over.
	taken   sync.Map
	onFrame func(data string)
	onInfo  func(url, title string)
}

func (p *chromePage) Listen(onFrame func(data string), onInfo func(url, title string), onDownload func(guid string, d api.BrowserDownload)) {
	p.listen.mu.Lock()
	p.listen.onFrame, p.listen.onInfo = onFrame, onInfo
	p.listen.mu.Unlock()
	chromedp.ListenTarget(p.ctx, func(ev any) {
		switch e := ev.(type) {
		case *page.EventScreencastFrame:
			go p.deliver(e)
		case *page.EventFrameNavigated:
			if e.Frame.ParentID == "" {
				go p.reportInfo()
			}
		case *page.EventNavigatedWithinDocument, *page.EventDomContentEventFired:
			go p.reportInfo()
		}
	})
	p.keepOneTab()
	p.watchDownloads(onDownload)
}

// watchDownloads hands on each download the page starts and each step it
// takes, in Chrome's order: a progress report never overtakes its begin.
func (p *chromePage) watchDownloads(onDownload func(guid string, d api.BrowserDownload)) {
	if p.dl == nil {
		return
	}
	chromedp.ListenBrowser(p.ctx, func(ev any) {
		switch e := ev.(type) {
		case *browser.EventDownloadWillBegin:
			onDownload(e.GUID, p.dl.begin(e.GUID, e.SuggestedFilename))
		case *browser.EventDownloadProgress:
			d, ok := p.dl.progress(e.GUID, e.ReceivedBytes, string(e.State))
			if !ok {
				return
			}
			if d.State != api.DownloadCompleted {
				onDownload(e.GUID, d)
				return
			}
			// Every browser event waits behind this listener, and a move
			// across disks takes as long as the file.
			go func() { onDownload(e.GUID, p.dl.place(e.GUID, d)) }()
		}
	})
}

// keepOneTab loads, in this page, what a link or a script opens in a new tab
// or window, and closes that tab. The pane shows only this page, and Chrome
// puts the one a new tab covers in the background, where it takes no input
// and draws no frames: one click on a target=_blank link froze the pane.
func (p *chromePage) keepOneTab() {
	c := chromedp.FromContext(p.ctx)
	self := c.Target.TargetID
	discover := chromedp.ActionFunc(func(ctx context.Context) error {
		return target.SetDiscoverTargets(true).Do(cdp.WithExecutor(ctx, c.Browser))
	})
	if err := p.run(nil, discover); err != nil {
		slog.Warn("browser: watch for new tabs", "err", err)
		return
	}
	chromedp.ListenBrowser(p.ctx, func(ev any) {
		var info *target.Info
		switch e := ev.(type) {
		case *target.EventTargetCreated:
			info = e.TargetInfo
		case *target.EventTargetInfoChanged:
			info = e.TargetInfo
		}
		// A new tab is created at about:blank and gets its URL after.
		if info == nil || info.OpenerID != self || info.Type != "page" || info.URL == "" || info.URL == "about:blank" {
			return
		}
		// Both the created and the info-changed event can name the tab.
		if _, taken := p.listen.taken.LoadOrStore(info.TargetID, true); !taken {
			go p.takeOver(info.TargetID, info.URL)
		}
	})
}

// takeOver closes the tab id this page opened and loads its url here.
func (p *chromePage) takeOver(id target.ID, url string) {
	c := chromedp.FromContext(p.ctx)
	closeTab := chromedp.ActionFunc(func(ctx context.Context) error {
		return target.CloseTarget(id).Do(cdp.WithExecutor(ctx, c.Browser))
	})
	if err := p.run(nil, closeTab); err != nil {
		slog.Warn("browser: close a tab the page opened", "url", url, "err", err)
	}
	if err := p.run(nil, page.BringToFront()); err != nil {
		slog.Warn("browser: bring the page back to the front", "err", err)
	}
	if _, err := allowURL(url); err != nil {
		slog.Warn("browser: a new tab's URL is not one the page may load", "url", url, "err", err)
		return
	}
	if err := p.Navigate(context.Background(), url); err != nil && p.Alive() {
		slog.Warn("browser: load a new tab's URL in the page", "url", url, "err", err)
	}
}

// reportInfo reads the URL and title after a navigation: a link followed, a
// history step, a single-page app's pushState, or the document's title
// arriving with its DOM.
func (p *chromePage) reportInfo() {
	p.listen.mu.Lock()
	onInfo := p.listen.onInfo
	p.listen.mu.Unlock()
	if onInfo == nil {
		return
	}
	var info struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := p.run(nil, chromedp.Evaluate(`({url: location.href, title: document.title})`, &info)); err != nil {
		if p.Alive() {
			slog.Warn("browser: read the page's URL and title", "err", err)
		}
		return
	}
	onInfo(info.URL, info.Title)
}

// deliver hands one frame on, then acks it. Chrome sends no next frame until
// the ack, so a window slow to take frames slows the stream instead of
// piling it up.
func (p *chromePage) deliver(frame *page.EventScreencastFrame) {
	p.listen.mu.Lock()
	onFrame := p.listen.onFrame
	p.listen.mu.Unlock()
	if onFrame != nil {
		onFrame(frame.Data)
	}
	ack := chromedp.ActionFunc(func(ctx context.Context) error {
		return page.ScreencastFrameAck(frame.SessionID).Do(ctx)
	})
	if err := p.run(nil, ack); err != nil && p.Alive() {
		slog.Warn("browser: ack a screencast frame", "err", err)
	}
}

// Screencast starts the frames at v, or restarts them there: Chrome keeps
// the size a screencast started with.
func (p *chromePage) Screencast(ctx context.Context, v Viewport) error {
	if err := p.run(ctx, page.StopScreencast()); err != nil {
		return err
	}
	if err := p.Resize(ctx, v); err != nil {
		return err
	}
	w, h := v.devicePixels()
	return p.run(ctx, page.StartScreencast().
		WithFormat(page.ScreencastFormatJpeg).
		WithQuality(screencastQuality).
		WithMaxWidth(w).
		WithMaxHeight(h))
}

func (p *chromePage) StopScreencast(ctx context.Context) error {
	return p.run(ctx, page.StopScreencast())
}

// Resize lays the page out at the pane's size, as a window of that size would.
func (p *chromePage) Resize(ctx context.Context, v Viewport) error {
	return p.run(ctx, emulation.SetDeviceMetricsOverride(int64(v.W), int64(v.H), v.Scale, false))
}

func (p *chromePage) Mouse(ctx context.Context, m api.BrowserMouseOptions) error {
	kind, ok := mouseTypes[m.Type]
	if !ok {
		return errUnknownInput
	}
	ev := input.DispatchMouseEvent(kind, m.X, m.Y).WithModifiers(input.Modifier(m.Modifiers))
	switch kind {
	case input.MouseWheel:
		ev = ev.WithDeltaX(m.DX).WithDeltaY(m.DY)
	case input.MousePressed, input.MouseReleased:
		ev = ev.WithButton(input.MouseButton(m.Button)).WithClickCount(int64(max(m.Clicks, 1)))
	}
	if kind != input.MouseReleased {
		// buttons is what a drag reads: which button is still held as it moves.
		ev = ev.WithButtons(mouseButtons[m.Button])
	}
	return p.run(ctx, ev)
}

// Key sends a key that types no text. Enter is the exception CDP needs: its
// default action (submit, newline) runs only on a keyDown carrying "\r".
func (p *chromePage) Key(ctx context.Context, k api.BrowserKeyOptions) error {
	var ev *input.DispatchKeyEventParams
	switch {
	case k.Type == "up":
		ev = input.DispatchKeyEvent(input.KeyUp)
	case k.Type != "down":
		return errUnknownInput
	case k.Key == "Enter":
		ev = input.DispatchKeyEvent(input.KeyDown).WithText("\r")
	default:
		ev = input.DispatchKeyEvent(input.KeyRawDown)
	}
	ev = ev.WithKey(k.Key).WithCode(k.Code).
		WithWindowsVirtualKeyCode(int64(k.KeyCode)).
		WithNativeVirtualKeyCode(int64(k.KeyCode)).
		WithModifiers(input.Modifier(k.Modifiers))
	return p.run(ctx, ev)
}

func (p *chromePage) InsertText(ctx context.Context, text string) error {
	return p.run(ctx, input.InsertText(text))
}

// selectionScript reads the selection of a text field, which getSelection
// does not see, else of the document. A password field gives nothing, as
// Chrome refuses to copy one.
const selectionScript = `(() => {
	const el = document.activeElement;
	if (el && typeof el.selectionStart === 'number' && typeof el.value === 'string') {
		return el.type === 'password' ? '' : el.value.slice(el.selectionStart, el.selectionEnd);
	}
	return String(getSelection());
})()`

func (p *chromePage) Selection(ctx context.Context) (string, error) {
	var text string
	return text, p.run(ctx, chromedp.Evaluate(selectionScript, &text))
}

var mouseTypes = map[string]input.MouseType{
	"press":   input.MousePressed,
	"release": input.MouseReleased,
	"move":    input.MouseMoved,
	"wheel":   input.MouseWheel,
}

// mouseButtons is CDP's buttons bit set: left 1, right 2, middle 4.
var mouseButtons = map[string]int64{"left": 1, "right": 2, "middle": 4}

func (v Viewport) devicePixels() (w, h int64) {
	return int64(math.Round(float64(v.W) * v.Scale)), int64(math.Round(float64(v.H) * v.Scale))
}
