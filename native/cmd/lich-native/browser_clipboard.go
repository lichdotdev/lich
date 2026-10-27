package main

import (
	"context"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"
)

// The page pane's clipboard. The headless browser has a clipboard of its own,
// so the page's copy and paste reach the person's only through the pane: a
// paste types the system clipboard's text into the page, a copy or a cut
// reads the page's selection and writes it to the system clipboard.

// browserClipboardChord is the clipboard action a key asks of the page:
// paste, copy or cut, "" for none. They are a browser's chords (Ctrl+V, C and
// X, Cmd on macOS), not the terminal's Ctrl+Shift+V, which pastes too.
func browserClipboardChord(e key.Event, goos string) string {
	if e.State != key.Press {
		return ""
	}
	mod := key.ModCtrl
	if goos == "darwin" {
		mod = key.ModCommand
	}
	if !e.Modifiers.Contain(mod) {
		return ""
	}
	switch e.Name {
	case "V":
		return "paste"
	case "C":
		if !e.Modifiers.Contain(key.ModShift) {
			return "copy"
		}
	case "X":
		return "cut"
	}
	return ""
}

// copyPageSelection reads the page's selection and hands it to the next
// frame to write: a clipboard write runs on the frame's goroutine.
func (u *window) copyPageSelection(p *browserPaneUI, session string) {
	u.sendBrowser(session, "copy", func(ctx context.Context) error {
		text, err := u.client.BrowserSelection(ctx, session)
		if err != nil || text == "" {
			return err
		}
		select {
		case p.copied <- text:
		default:
			// A copy not written yet is replaced by the newer one.
			select {
			case <-p.copied:
			default:
			}
			p.copied <- text
		}
		u.model.invalidate()
		return nil
	})
}

// writeCopied writes a copied selection waiting for this frame.
func writeCopied(gtx C, p *browserPaneUI) {
	select {
	case text := <-p.copied:
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
	default:
	}
}
