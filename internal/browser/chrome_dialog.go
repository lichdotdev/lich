package browser

import (
	"context"
	"sync"

	"github.com/chromedp/cdproto/page"
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

func (g *dialogGate) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.isOpen {
		g.ch = make(chan struct{})
		g.isOpen = false
	}
}

func (p *chromePage) AnswerDialog(ctx context.Context, accept bool, text string) error {
	return p.runFree(ctx, page.HandleJavaScriptDialog(accept).WithPromptText(text))
}
