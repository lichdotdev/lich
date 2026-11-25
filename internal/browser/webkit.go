package browser

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

// webkitPage is one session's page in lich-browser, driven over the wire: the
// page the agent drives is the one the window shows, an X window of
// lich-browser's it embeds, so it sends no frames and takes no input from the
// pane (docs/lich-browser.md).
type webkitPage struct {
	conn *wireConn
	id   string
	// died is lichBrowser.gone, run once when the page dies under the backend.
	died func()
	indexCounter
	gate     dialogGate
	loads    loadTrack
	requests fetches

	mu           sync.Mutex
	dead, closed bool
	// ended: lich-browser ended the page itself, its window closed.
	ended          bool
	zoom           float64
	tabs           []api.BrowserTab
	front          string
	url, title     string
	onInfo         func(url, title string)
	onDownload     func(guid string, d api.BrowserDownload)
	onConsoleError func(text string)
	onDialog       func(*api.BrowserDialog)
	onTabs         func([]api.BrowserTab)
	onFiles        func(page.FileChooserOpenedMode) ([]string, error)
}

func newWebkitPage(c *wireConn, id string, zoom float64, died func()) *webkitPage {
	return &webkitPage{conn: c, id: id, zoom: zoom, died: died, loads: loadTrack{wake: make(chan struct{})}}
}

// event takes one of lich-browser's events for this page, on the
// connection's reader: what may block runs on a goroutine of its own.
func (p *webkitPage) event(name string, raw json.RawMessage) {
	var err error
	switch name {
	case browserwire.EventTabs:
		var ev browserwire.TabsEvent
		if err = json.Unmarshal(raw, &ev); err == nil {
			p.setTabs(ev.Tabs)
		}
	case browserwire.EventLoad:
		var ev browserwire.LoadEvent
		if err = json.Unmarshal(raw, &ev); err == nil && p.isFront(ev.Tab) {
			p.loads.note(ev, time.Now())
		}
	case browserwire.EventRequest:
		var ev browserwire.RequestEvent
		if err = json.Unmarshal(raw, &ev); err == nil && p.isFront(ev.Tab) {
			p.noteRequest(ev, time.Now())
		}
	case browserwire.EventConsole:
		var ev browserwire.ConsoleEvent
		if err = json.Unmarshal(raw, &ev); err == nil {
			p.consoleError(ev.Text)
		}
	case browserwire.EventDialog:
		var ev browserwire.DialogEvent
		if err = json.Unmarshal(raw, &ev); err == nil {
			p.dialog(ev.Dialog)
		}
	case browserwire.EventFiles:
		var ev browserwire.FilesEvent
		if err = json.Unmarshal(raw, &ev); err == nil {
			go p.chooseFiles(ev.Multiple)
		}
	default:
		err = p.otherEvent(name, raw)
	}
	if err != nil {
		slog.Warn("browser: an event from lich-browser", "event", name, "err", err)
	}
}

func (p *webkitPage) otherEvent(name string, raw json.RawMessage) error {
	switch name {
	case browserwire.EventDownload:
		var ev browserwire.DownloadEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			return err
		}
		p.mu.Lock()
		onDownload := p.onDownload
		p.mu.Unlock()
		if onDownload != nil {
			onDownload(ev.GUID, api.BrowserDownload{Path: ev.Path, Bytes: ev.Received, State: ev.State})
		}
	case browserwire.EventCrashed:
		p.gone()
	case browserwire.EventClosed:
		p.mu.Lock()
		p.ended = true
		p.mu.Unlock()
		p.gone()
	}
	return nil
}

func (p *webkitPage) isFront(tab string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return tab == "" || tab == p.front
}

// setTabs keeps the page's tabs, and tells the service the front tab's URL
// and title when they changed: a link followed, a history step, a
// pushState.
func (p *webkitPage) setTabs(tabs []browserwire.Tab) {
	out := make([]api.BrowserTab, 0, len(tabs))
	var front *browserwire.Tab
	for i, t := range tabs {
		out = append(out, api.BrowserTab{ID: t.ID, URL: t.URL, Title: t.Title, Front: t.Front, Loading: t.Loading, Icon: tabIcon(t), XID: t.XID})
		if t.Front {
			front = &tabs[i]
		}
	}
	p.mu.Lock()
	p.tabs = out
	moved := front != nil && (front.URL != p.url || front.Title != p.title)
	if front != nil && front.ID != p.front {
		p.front = front.ID
		p.requests.clear()
	}
	if moved {
		p.url, p.title = front.URL, front.Title
	}
	onInfo, onTabs := p.onInfo, p.onTabs
	p.mu.Unlock()
	if moved && onInfo != nil {
		onInfo(front.URL, front.Title)
	}
	if onTabs != nil {
		onTabs(out)
	}
}

// tabIcon is the key BrowserIcon reads t's icon by, "" for none or for one
// past the size an icon may have.
func tabIcon(t browserwire.Tab) string {
	if len(t.Icon) == 0 || len(t.Icon) > iconMaxBytes {
		return ""
	}
	return icons.store("lich-browser "+t.URL, t.Icon, time.Now())
}

func (p *webkitPage) consoleError(text string) {
	p.mu.Lock()
	onConsoleError := p.onConsoleError
	p.mu.Unlock()
	if onConsoleError != nil {
		onConsoleError(text)
	}
}

// dialog holds every call that runs the page's script or input while d is
// open, as chromePage does: WebKit runs none of the page's script until it
// is answered.
func (p *webkitPage) dialog(d *browserwire.JSDialog) {
	p.mu.Lock()
	onDialog, url := p.onDialog, p.url
	p.mu.Unlock()
	if d == nil {
		p.gate.close()
		if onDialog != nil {
			onDialog(nil)
		}
		return
	}
	// The service holds the dialog before a call the gate releases asks it
	// which one.
	if onDialog != nil {
		onDialog(&api.BrowserDialog{Type: d.Type, Message: d.Message, Default: d.Default, URL: url})
	}
	p.gate.open()
}

// chooseFiles answers the page's file chooser with what the picker answers,
// nothing cancelling it: lich-browser holds the chooser until it is answered.
func (p *webkitPage) chooseFiles(multiple bool) {
	p.mu.Lock()
	pick := p.onFiles
	p.mu.Unlock()
	paths := []string{}
	if pick != nil {
		mode := page.FileChooserOpenedModeSelectSingle
		if multiple {
			mode = page.FileChooserOpenedModeSelectMultiple
		}
		picked, err := pick(mode)
		if err != nil {
			slog.Warn("browser: open the file chooser", "err", err)
		}
		paths = append(paths, picked...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.do(ctx, browserwire.Files, browserwire.FilesParams{Page: p.id, Paths: paths}, nil); err != nil && p.Alive() {
		slog.Warn("browser: answer a file chooser", "err", err)
	}
}

func (p *webkitPage) Listen(_ func(tab, data string), onInfo func(url, title string), onDownload func(guid string, d api.BrowserDownload), onConsoleError func(text string), onDialog func(*api.BrowserDialog), onTabs func([]api.BrowserTab)) {
	p.mu.Lock()
	p.onInfo, p.onDownload, p.onConsoleError, p.onDialog, p.onTabs = onInfo, onDownload, onConsoleError, onDialog, onTabs
	tabs := p.tabs
	p.mu.Unlock()
	// The tabs open sent came before anyone listened.
	if tabs != nil && onTabs != nil {
		onTabs(tabs)
	}
}

// OnReload is Chrome's: the young browser's reload over a download and the
// window's own page following a reload are none of lich-browser's.
func (p *webkitPage) OnReload(func(reloadStep)) {}

func (p *webkitPage) OnFileChooser(pick func(page.FileChooserOpenedMode) ([]string, error)) {
	p.mu.Lock()
	p.onFiles = pick
	p.mu.Unlock()
}

// gone is the page dying under the backend, its process or its web process.
func (p *webkitPage) gone() {
	p.mu.Lock()
	was := p.dead || p.closed
	p.dead = true
	p.mu.Unlock()
	if !was && p.died != nil {
		go p.died()
	}
}

func (p *webkitPage) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.dead && !p.closed && p.conn.alive()
}

func (p *webkitPage) Close() error {
	p.mu.Lock()
	was := p.closed || p.ended
	p.closed = true
	p.mu.Unlock()
	p.conn.remove(p)
	if was || !p.conn.alive() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), closeWait)
	defer cancel()
	return p.do(ctx, browserwire.Close, browserwire.PageParams{Page: p.id}, nil)
}

func (p *webkitPage) do(ctx context.Context, method string, params, out any) error {
	return p.conn.call(ctx, method, params, out)
}

// gated runs fn, failing at once with errDialogOpen while the page holds a
// dialog and as soon as one opens while it runs, as chromePage.run does.
func (p *webkitPage) gated(ctx context.Context, fn func(context.Context) error) error {
	held := p.gate.held()
	select {
	case <-held:
		return errDialogOpen
	default:
	}
	done := make(chan error, 1)
	go func() { done <- fn(ctx) }()
	return awaitAction(done, held, nil)
}

// evalRaw is the evalFunc of the page's own scripts, which a dialog holds.
func (p *webkitPage) evalRaw(ctx context.Context, expr string, out any) error {
	var r browserwire.EvalResult
	if err := p.do(ctx, browserwire.Eval, browserwire.EvalParams{Page: p.id, Script: expr}, &r); err != nil {
		return err
	}
	if out == nil || r.JSON == "" {
		return nil
	}
	return json.Unmarshal([]byte(r.JSON), out)
}

func (p *webkitPage) evaluate(ctx context.Context, expr string, out any) error {
	return p.gated(ctx, func(ctx context.Context) error { return p.evalRaw(ctx, expr, out) })
}
