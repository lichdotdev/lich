package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log/slog"
	"math"
	"os"
	"slices"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp/kb"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
	xdraw "golang.org/x/image/draw"
)

// errNoFrames: the window embeds a lich-browser page, which sends no frames.
var errNoFrames = errors.New("lich-browser's page is drawn by the window, not by frames")

func (p *webkitPage) Navigate(ctx context.Context, url string) error {
	return p.load(ctx, browserwire.Navigate, browserwire.NavigateParams{Page: p.id, URL: url})
}

func (p *webkitPage) Reload(ctx context.Context) error { return p.history(ctx, browserwire.Reload) }

func (p *webkitPage) Back(ctx context.Context) error { return p.history(ctx, browserwire.Back) }

func (p *webkitPage) Forward(ctx context.Context) error { return p.history(ctx, browserwire.Forward) }

func (p *webkitPage) history(ctx context.Context, method string) error {
	err := p.load(ctx, method, browserwire.PageParams{Page: p.id})
	if errors.As(err, new(failedLoad)) {
		return nil
	}
	return err
}

func (p *webkitPage) Info(ctx context.Context, within string) (PageInfo, error) {
	return readInfo(ctx, p.evaluate, &p.indexCounter, within)
}

func (p *webkitPage) HasText(ctx context.Context, text string) (bool, error) {
	return hasText(ctx, p.evaluate, text)
}

func (p *webkitPage) Selection(ctx context.Context) (string, error) {
	var text string
	return text, p.evaluate(ctx, selectionScript, &text)
}

func (p *webkitPage) ElementAt(ctx context.Context, x, y float64) (api.BrowserElement, error) {
	return elementAt(ctx, p.evaluate, x, y)
}

func (p *webkitPage) Locate(ctx context.Context, selectors []string, reveal string) ([]api.BrowserRect, error) {
	return locate(ctx, p.evaluate, selectors, reveal)
}

// Click is a native press and release at the target's point, which the page
// sees as trusted, as a person's.
func (p *webkitPage) Click(ctx context.Context, t Target) (x, y float64, err error) {
	pt, err := resolveTarget(ctx, p.evaluate, t)
	if err != nil {
		return 0, 0, err
	}
	return pt.X, pt.Y, p.gated(ctx, func(ctx context.Context) error {
		for _, kind := range []string{browserwire.MouseMove, browserwire.MouseDown, browserwire.MouseUp} {
			m := browserwire.MouseParams{Page: p.id, Kind: kind, X: pt.X, Y: pt.Y}
			if kind != browserwire.MouseMove {
				m.Clicks = 1
			}
			if err := p.do(ctx, browserwire.Mouse, m, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *webkitPage) Type(ctx context.Context, text string, clear bool, t Target) error {
	if !t.empty() {
		if _, _, err := p.Click(ctx, t); err != nil {
			return err
		}
	}
	if clear {
		if err := p.evaluate(ctx, clearFieldJS, nil); err != nil {
			return err
		}
	}
	return p.gated(ctx, func(ctx context.Context) error {
		return p.do(ctx, browserwire.Text, browserwire.TextParams{Page: p.id, Text: text}, nil)
	})
}

// Press sends the named key down and up, by the DOM key and code chromedp's
// table gives the rune keySequence names it by.
func (p *webkitPage) Press(ctx context.Context, key string, t Target) error {
	seq, err := keySequence(key)
	if err != nil {
		return err
	}
	k := kb.Keys[[]rune(seq)[0]]
	if !t.empty() {
		if _, _, err := p.Click(ctx, t); err != nil {
			return err
		}
	}
	return p.gated(ctx, func(ctx context.Context) error {
		for _, kind := range []string{browserwire.KeyDown, browserwire.KeyUp} {
			if err := p.do(ctx, browserwire.Key, browserwire.KeyParams{Page: p.id, Kind: kind, Key: k.Key, Code: k.Code}, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// Scroll is a native wheel turn, so it scrolls what is under the point.
func (p *webkitPage) Scroll(ctx context.Context, at *point, dy int) (x, y float64, err error) {
	if at == nil {
		at = new(point)
		if err := p.evaluate(ctx, viewMiddleJS, at); err != nil {
			return 0, 0, err
		}
	}
	return at.X, at.Y, p.gated(ctx, func(ctx context.Context) error {
		m := browserwire.MouseParams{Page: p.id, Kind: browserwire.MouseWheel, X: at.X, Y: at.Y, DY: float64(dy)}
		return p.do(ctx, browserwire.Mouse, m, nil)
	})
}

// Eval answers as chromePage.Eval does: a string as it is, anything else as
// its JSON, and a throw as the script's error.
func (p *webkitPage) Eval(ctx context.Context, script string) (string, error) {
	var r browserwire.EvalResult
	err := p.gated(ctx, func(ctx context.Context) error {
		return p.do(ctx, browserwire.Eval, browserwire.EvalParams{Page: p.id, Script: script}, &r)
	})
	switch {
	case errors.Is(err, errDialogOpen), errors.Is(err, errBrowserGone), errors.Is(err, context.DeadlineExceeded):
		return "", err
	case err != nil:
		return "", fmt.Errorf("the script threw: %s", err)
	}
	return evalText(r.JSON), nil
}

func evalText(raw string) string {
	var s string
	switch {
	case raw == "":
		return "undefined"
	case json.Unmarshal([]byte(raw), &s) == nil:
		return s
	}
	return raw
}

// Screenshot is the front tab's snapshot scaled to shotScale image pixels
// per CSS pixel, as Chrome's clip is, without the window's layer: the notes
// and the agent's own cursor and chip are the window's, not the page's.
func (p *webkitPage) Screenshot(ctx context.Context, dest string) (w, h int, err error) {
	var snap browserwire.SnapshotResult
	err = p.gated(ctx, func(ctx context.Context) error {
		return p.do(ctx, browserwire.Snapshot, browserwire.SnapshotParams{Page: p.id, Bare: true}, &snap)
	})
	if err != nil {
		return 0, 0, err
	}
	data, err := shotImage(snap)
	if err != nil {
		return 0, 0, err
	}
	return snap.W, snap.H, os.WriteFile(dest, data, 0o600)
}

func shotImage(snap browserwire.SnapshotResult) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(snap.PNG))
	if err != nil {
		return nil, fmt.Errorf("lich-browser's snapshot: %w", err)
	}
	k := shotScale(snap.W, snap.H)
	w, h := int(math.Round(float64(snap.W)*k)), int(math.Round(float64(snap.H)*k))
	if b := src.Bounds(); b.Dx() == w && b.Dy() == h {
		return snap.PNG, nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// AnnotatedScreenshot draws the labels into the page for the snapshot, as
// chromePage's does.
func (p *webkitPage) AnnotatedScreenshot(ctx context.Context, dest string) (w, h int, err error) {
	info, err := p.Info(ctx, "")
	if err != nil {
		return 0, 0, err
	}
	script, err := annotation(info)
	if err != nil {
		return 0, 0, err
	}
	defer p.unannotate()
	if err := p.evaluate(ctx, script, nil); err != nil {
		return 0, 0, err
	}
	return p.Screenshot(ctx, dest)
}

// unannotate skips the gate: the page's own timer removes what a held
// dialog keeps this from reaching.
func (p *webkitPage) unannotate() {
	ctx, cancel := context.WithTimeout(context.Background(), unannotateWait)
	defer cancel()
	if err := p.evalRaw(ctx, unannotateJS, nil); err != nil {
		slog.Warn("browser: annotation overlay left for the page to remove", "err", err)
	}
}

func (p *webkitPage) NewTab(ctx context.Context) error {
	return p.do(ctx, browserwire.NewTab, browserwire.PageParams{Page: p.id}, nil)
}

func (p *webkitPage) SelectTab(ctx context.Context, id string) error {
	if _, err := p.tabCount(id); err != nil {
		return err
	}
	return p.do(ctx, browserwire.SelectTab, browserwire.TabParams{Page: p.id, Tab: id}, nil)
}

func (p *webkitPage) CloseTab(ctx context.Context, id string) error {
	n, err := p.tabCount(id)
	if err != nil {
		return err
	}
	if n == 1 {
		return errLastTab
	}
	return p.do(ctx, browserwire.CloseTab, browserwire.TabParams{Page: p.id, Tab: id}, nil)
}

// tabCount is how many tabs the page has, errNoTab when id is none of them.
func (p *webkitPage) tabCount(id string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !slices.ContainsFunc(p.tabs, func(t api.BrowserTab) bool { return t.ID == id }) {
		return 0, errNoTab
	}
	return len(p.tabs), nil
}

// DevTools is WebKit's inspector on the front tab, in a window of its own.
func (p *webkitPage) DevTools(ctx context.Context) error {
	return p.do(ctx, browserwire.Inspect, browserwire.PageParams{Page: p.id}, nil)
}

func (p *webkitPage) AnswerDialog(ctx context.Context, accept bool, text string) error {
	return p.do(ctx, browserwire.Dialog, browserwire.DialogParams{Page: p.id, Accept: accept, Text: text}, nil)
}

// Resize lays the page out at v while no window shows it, and draws it at
// v's scale: the page zoom and a monitor of another density reach it here,
// where Chrome had to relaunch.
func (p *webkitPage) Resize(ctx context.Context, v Viewport) error {
	if err := p.do(ctx, browserwire.Size, browserwire.SizeParams{Page: p.id, W: v.W, H: v.H}, nil); err != nil {
		return err
	}
	p.mu.Lock()
	same := v.Scale <= 0 || v.Scale == p.zoom
	p.mu.Unlock()
	if same {
		return nil
	}
	if err := p.do(ctx, browserwire.Zoom, browserwire.ZoomParams{Page: p.id, Scale: v.Scale}, nil); err != nil {
		return err
	}
	p.mu.Lock()
	p.zoom = v.Scale
	p.mu.Unlock()
	return nil
}

// The pane's screencast path: the window embeds the page's X window, so a
// pane gets no frames, and the person's input reaches the page from the X
// server, never through the backend.

func (p *webkitPage) Screencast(ctx context.Context, v Viewport) error { return p.Resize(ctx, v) }

func (p *webkitPage) StopScreencast(context.Context) error { return nil }

func (p *webkitPage) Still(context.Context) (tab, data string, err error) {
	return "", "", errNoFrames
}

// Mouse passes on the wheel alone: the window sends it while Select is on,
// when the page lets the pointer through to the window and X gives the page
// no wheel. Everything else reaches the page from the X server.
func (p *webkitPage) Mouse(ctx context.Context, m api.BrowserMouseOptions) error {
	if m.Type != "wheel" {
		return nil
	}
	return p.do(ctx, browserwire.Mouse, browserwire.MouseParams{Page: p.id, Kind: browserwire.MouseWheel, X: m.X, Y: m.Y, DX: m.DX, DY: m.DY}, nil)
}

func (p *webkitPage) Key(context.Context, api.BrowserKeyOptions) error { return nil }

func (p *webkitPage) InsertText(context.Context, string) error { return nil }

// Focus raises the page's own window; the service asks only once it has one.
func (p *webkitPage) Focus(ctx context.Context) error {
	return p.do(ctx, browserwire.Raise, browserwire.PageParams{Page: p.id}, nil)
}

// PopOut moves the page to a window of its own, as it is: nothing reloads.
func (p *webkitPage) PopOut(ctx context.Context) error {
	return p.do(ctx, browserwire.PopOut, browserwire.PageParams{Page: p.id}, nil)
}

// Cookies and SetCookies carry cookies across a Chrome relaunch, which a
// lich-browser page never has: a zoom replaces the rescale's, and PopOut the
// promotion's.
func (p *webkitPage) Cookies(context.Context) ([]*network.CookieParam, error) { return nil, nil }

func (p *webkitPage) SetCookies(context.Context, []*network.CookieParam) error { return nil }
