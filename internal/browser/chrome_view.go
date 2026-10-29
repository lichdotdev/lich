package browser

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
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
	// taken are the tabs followTabs already brought in.
	taken          sync.Map
	onFrame        func(data string)
	onInfo         func(url, title string)
	onConsoleError func(text string)
	onDialog       func(*api.BrowserDialog)
	onTabs         func([]api.BrowserTab)
}

func (p *chromePage) Listen(onFrame func(data string), onInfo func(url, title string), onDownload func(guid string, d api.BrowserDownload), onConsoleError func(text string), onDialog func(*api.BrowserDialog), onTabs func([]api.BrowserTab)) {
	p.listen.mu.Lock()
	p.listen.onFrame, p.listen.onInfo = onFrame, onInfo
	p.listen.onConsoleError, p.listen.onDialog, p.listen.onTabs = onConsoleError, onDialog, onTabs
	p.listen.mu.Unlock()
	p.listenTab(p.tabs.frontTab())
	p.followTabs()
	p.watchDownloads(onDownload)
}

// listenTab hands on the events of t while it is the front tab, and its
// dialogs from behind too.
func (p *chromePage) listenTab(t *tab) {
	chromedp.ListenTarget(t.ctx, func(ev any) {
		switch e := ev.(type) {
		case *page.EventJavascriptDialogOpening:
			p.tabs.setDialog(t)
			// The service holds the dialog before a call the gate releases
			// asks it which one.
			p.dialogShown(&api.BrowserDialog{Type: string(e.Type), Message: e.Message, Default: e.DefaultPrompt, URL: e.URL})
			p.gate.open()
			return
		case *page.EventJavascriptDialogClosed:
			if !p.tabs.dialogClosed(t) {
				return
			}
			if v := p.gate.close(); v != nil {
				go p.layOut(*v)
			}
			p.dialogShown(nil)
			return
		}
		if p.tabs.frontTab() == t {
			p.frontEvent(ev)
		}
	})
}

func (p *chromePage) frontEvent(ev any) {
	switch e := ev.(type) {
	case *page.EventScreencastFrame:
		go p.deliver(e)
	case *page.EventFrameNavigated:
		if e.Frame.ParentID == "" {
			go p.reportInfo()
		}
	case *page.EventNavigatedWithinDocument, *page.EventDomContentEventFired:
		go p.reportInfo()
	default:
		if text, ok := consoleError(ev); ok {
			p.listen.mu.Lock()
			onConsoleError := p.listen.onConsoleError
			p.listen.mu.Unlock()
			onConsoleError(text)
		}
	}
}

func (p *chromePage) dialogShown(d *api.BrowserDialog) {
	p.listen.mu.Lock()
	onDialog := p.listen.onDialog
	p.listen.mu.Unlock()
	onDialog(d)
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
	front := p.tabs.frontTab()
	var info struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := p.run(nil, chromedp.Evaluate(`({url: location.href, title: document.title})`, &info)); err != nil {
		if p.Alive() && !errors.Is(err, errDialogOpen) {
			slog.Warn("browser: read the page's URL and title", "err", err)
		}
		return
	}
	onInfo(info.URL, info.Title)
	// Chrome's target events never brought the title of a tab lich opened
	// itself, which the header named by its URL (measured 2026-10-29).
	if p.tabs.setInfo(front, info.URL, info.Title) {
		p.tabsChanged()
	}
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
	if err := p.runFree(nil, ack); err != nil && p.Alive() {
		slog.Warn("browser: ack a screencast frame", "err", err)
	}
}

// Screencast starts the frames at v, or restarts them there: Chrome keeps
// the size a screencast started with.
func (p *chromePage) Screencast(ctx context.Context, v Viewport) error {
	return p.castPastDialog(func() error { return p.startCast(ctx, v) })
}

// startCast is Screencast's step, run in its turn among the others.
func (p *chromePage) startCast(ctx context.Context, v Viewport) error {
	v = v.atFrameScale()
	w, h := v.devicePixels()
	if err := p.stopCast(ctx); err != nil {
		return err
	}
	if err := p.Resize(ctx, v); err != nil {
		return err
	}
	p.casting = true
	return p.runFree(ctx, page.StartScreencast().
		WithFormat(page.ScreencastFormatJpeg).
		WithQuality(screencastQuality).
		WithMaxWidth(w).
		WithMaxHeight(h))
}

func (p *chromePage) StopScreencast(ctx context.Context) error {
	return p.castPastDialog(func() error { return p.stopCast(ctx) })
}

// stopCast stops the frames. With none running it sends nothing, so a pane
// opened on a held page starts its frames without a stop waiting first.
func (p *chromePage) stopCast(ctx context.Context) error {
	if !p.casting {
		return nil
	}
	if err := p.runFree(ctx, page.StopScreencast()); err != nil {
		return err
	}
	p.casting = false
	return nil
}

// castPastDialog runs step, a start or stop of the screencast, after the ones
// before it, and returns once it is done or the page holds a dialog. Chrome up
// to 153 answers Page.startScreencast and Page.stopScreencast only once the
// renderer has seen them, and a renderer holding a dialog sees nothing until
// it closes, though the browser starts or stops the frames at once: a pane
// opened on the dialog waited the whole pageWait and lost the browser (Chrome
// 151, macos-15-intel runner, measured 2026-10-28). step runs on after that.
func (p *chromePage) castPastDialog(step func() error) error {
	p.castMu.Lock()
	prev, mine := p.castLast, make(chan struct{})
	p.castLast = mine
	p.castMu.Unlock()
	done := make(chan error, 1)
	go func() {
		defer close(mine)
		if prev != nil {
			<-prev
		}
		done <- step()
	}()
	select {
	case err := <-done:
		return err
	case <-p.gate.held():
		go func() {
			if err := <-done; err != nil && p.Alive() {
				slog.Warn("browser: start or stop the screencast under a dialog", "err", err)
			}
		}()
		return nil
	}
}

// Resize lays the page out at the pane's size, as a window of that size would.
// A page holding a dialog open cannot lay out, and the call waited on it for
// the whole pageWait: a pane opened then stayed black, its screencast never
// started (measured 2026-10-28). That size is laid out once the dialog closes;
// until then the frames come at the size the page had.
func (p *chromePage) Resize(ctx context.Context, v Viewport) error {
	v = v.atFrameScale()
	p.tabs.setView(v)
	err := p.run(ctx, emulation.SetDeviceMetricsOverride(int64(v.W), int64(v.H), v.Scale, false))
	if !errors.Is(err, errDialogOpen) {
		return err
	}
	if p.gate.deferView(v) {
		return nil
	}
	return p.Resize(ctx, v)
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

// atFrameScale is v at no more than frameScale: a page laid out past the
// scale its browser draws at reads a devicePixelRatio its frames do not have.
func (v Viewport) atFrameScale() Viewport {
	v.Scale = min(v.Scale, frameScale)
	return v
}

func (v Viewport) devicePixels() (w, h int64) {
	return int64(math.Round(float64(v.W) * v.Scale)), int64(math.Round(float64(v.H) * v.Scale))
}
