package main

// Two providers fold a long paste into a placeholder the user cannot edit, and
// each unfolds it on a key of its own: Claude Code ("[Pasted text #1 +32
// lines]") when the identical text is pasted again while the chip is in the
// prompt, Kiro CLI ("32 lines ▸") on Tab. lich sends that key right after the
// paste wherever the provider would fold, so the text lands editable. Mirroring
// each provider's own rule is the point and the risk (docs/ceilings.md).

import (
	"context"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// Claude Code 2.1.296, read off its prompt's paste handler: fold above 800
// characters or above min(rows - 10, 2) newlines, counted after it turns
// CRLF/CR into LF and a tab into four spaces; unfold only up to 100 000
// characters. Writing both pastes back to back was measured to unfold.
const (
	claudeFoldChars        = 800
	claudeFoldNewlines     = 2
	claudeFoldRowsReserved = 10
	claudeUnfoldMaxChars   = 100_000
)

// Kiro CLI 2.21.0, read off its chat bundle (`shouldCollapse:t>10||a>500`) and
// measured at each edge: fold above 10 lines or above 500 UTF-16 units, counted
// on the text as the terminal delivers it, where every newline is one CR. A
// trailing newline opens an eleventh line. Tab expands the newest chip.
const (
	kiroFoldLines = 10
	kiroFoldChars = 500
	kiroUnfoldKey = "\t"
)

// The JS rules count UTF-16 units, which is what String.length is.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func claudeFolds(text string, rows int) bool {
	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n", "\t", "    ").Replace(text)
	length := utf16Len(normalized)
	if length > claudeUnfoldMaxChars {
		return false
	}
	newlineLimit := max(0, min(rows-claudeFoldRowsReserved, claudeFoldNewlines))
	return length > claudeFoldChars || strings.Count(normalized, "\n") > newlineLimit
}

func kiroFolds(text string) bool {
	delivered := strings.NewReplacer("\r\n", "\r", "\n", "\r").Replace(text)
	return strings.Count(delivered, "\r")+1 > kiroFoldLines || utf16Len(delivered) > kiroFoldChars
}

// unfoldPaste is what goes right behind paste, the bytes that put text at the
// prompt of a session of kind on a terminal rows tall, for it to land as
// editable text: the paste again, a key, or "" where the agent keeps it as typed.
func unfoldPaste(kind, text, paste string, rows int) string {
	switch {
	case kind == "claude" && claudeFolds(text, rows):
		return paste
	case kind == "kiro" && kiroFolds(text):
		return kiroUnfoldKey
	}
	return ""
}

// unfolded is paste with unfoldPaste behind it when session id's provider
// would fold text and Unfold long pastes is on. The setting is read now, so a
// switch flipped in Settings answers the next paste, and so this runs off the
// UI goroutine. A session whose terminal this window never opened has no rows
// to measure and gets the paste alone.
func (ts *terminals) unfolded(ctx context.Context, id, text, paste string) string {
	ts.mu.Lock()
	t := ts.byID[id]
	ts.mu.Unlock()
	if t == nil {
		return paste
	}
	_, rows := t.vt.Size()
	more := unfoldPaste(t.kind, text, paste, rows)
	if more == "" {
		return paste
	}
	on, err := ts.client.GetSetting(ctx, providerPasteKey(t.kind), "")
	if err != nil {
		ts.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.view.pasteSettingFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
		return paste
	}
	if on != "true" {
		return paste
	}
	return paste + more
}

// writePaste writes a block bracketedPaste built into session id's PTY,
// unfolded where its provider would fold it.
func (ts *terminals) writePaste(ctx context.Context, id, block string) error {
	text := strings.TrimSuffix(strings.TrimPrefix(block, pasteStart), pasteEnd)
	return ts.client.Write(ctx, id, ts.unfolded(ctx, id, text, block))
}

// pasteClipboard is the clipboard paste into t, off the UI goroutine.
func (ts *terminals) pasteClipboard(t *termView, text string, goos string) {
	paste := string(pastedClipboard(t.vt, text, goos))
	if text != "" {
		paste = ts.unfolded(context.Background(), t.id, text, paste)
	}
	ts.send(t.id, []byte(paste))
}
