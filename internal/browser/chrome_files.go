package browser

import (
	"context"
	"log/slog"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// interceptFiles turns a file input's chooser into an event: a headless
// Chrome has no dialog to show, and a click on the input did nothing.
var interceptFiles = page.SetInterceptFileChooserDialog(true)

func (p *chromePage) OnFileChooser(pick func(page.FileChooserOpenedMode) ([]string, error)) {
	p.listen.mu.Lock()
	p.listen.onFileChooser = pick
	p.listen.mu.Unlock()
}

// chooseFiles fills the file input of t that asked for files with what the
// picker answers.
func (p *chromePage) chooseFiles(t *tab, e *page.EventFileChooserOpened) {
	p.listen.mu.Lock()
	pick := p.listen.onFileChooser
	p.listen.mu.Unlock()
	if pick == nil {
		return
	}
	paths, err := pick(e.Mode)
	if err != nil {
		slog.Warn("browser: open the file chooser", "err", err)
		return
	}
	if len(paths) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(t.ctx, pageWait)
	defer cancel()
	if err := chromedp.Run(ctx, dom.SetFileInputFiles(paths).WithBackendNodeID(e.BackendNodeID)); err != nil && p.Alive() {
		slog.Warn("browser: fill a file input", "err", err)
	}
}
