package browser

import (
	"context"
	"log/slog"
	"sync"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// dialogGate is held while the page has a JavaScript dialog open. Until it is
// answered, every script call and input event waits on it: an alert held the
// agent's click, and every read after it, for the whole pageWait (Chromium
// 155, measured 2026-10-27).
type dialogGate struct {
	mu sync.Mutex
	// ch is closed while a dialog is open.
	ch     chan struct{}
	isOpen bool
	// view is the size a pane asked for while a dialog was open.
	view *Viewport
}

// held is closed while a dialog is open, or as soon as one opens.
func (g *dialogGate) held() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ch == nil {
		g.ch = make(chan struct{})
	}
	return g.ch
}

func (g *dialogGate) open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ch == nil {
		g.ch = make(chan struct{})
	}
	if !g.isOpen {
		close(g.ch)
		g.isOpen = true
	}
}

// close lets calls through again and hands back the size a pane asked for
// meanwhile, nil for none.
func (g *dialogGate) close() *Viewport {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.isOpen {
		g.ch = make(chan struct{})
		g.isOpen = false
	}
	v := g.view
	g.view = nil
	return v
}

// deferView keeps v for when the dialog closes; false when none is open
// anymore, and v is the caller's to lay out.
func (g *dialogGate) deferView(v Viewport) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.isOpen {
		return false
	}
	g.view = &v
	return true
}

// layOut applies a size a pane asked for while a dialog held the page.
func (p *chromePage) layOut(v Viewport) {
	if err := p.Resize(nil, v); err != nil && p.Alive() {
		slog.Warn("browser: lay the page out after its dialog", "err", err)
	}
}

// AnswerDialog answers the tab holding the dialog, which may be behind the
// front one.
func (p *chromePage) AnswerDialog(ctx context.Context, accept bool, text string) error {
	tabCtx, cancel := withPageDeadline(p.tabs.dialogHolder().ctx, ctx)
	defer cancel()
	return chromedp.Run(tabCtx, page.HandleJavaScriptDialog(accept).WithPromptText(text))
}
