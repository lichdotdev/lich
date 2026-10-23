package main

// The inject half of the Code tab's preview menu: the
// right-click menu that writes a reference to the file, or to the lines under
// the selection, into the session's terminal, plus the comment for the
// session that opens the review agent's composer under the selection.

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// formatLineRef formats 1-based file lines: a single line reads "19", not "19-19".
func formatLineRef(from, to int) string {
	if from == to {
		return strconv.Itoa(from)
	}
	return strconv.Itoa(from) + "-" + strconv.Itoa(to)
}

// fileReference is what Inject file writes: the agent's @-mention of rel,
// ended by a space so the next word typed is not glued to it.
func fileReference(rel string) string { return "@" + rel + " " }

// linesReference is what Inject lines writes: rel:12-30, git's spelling.
func linesReference(rel, lines string) string { return rel + ":" + lines + " " }

const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// safeToWrite makes text safe to type into the terminal. Nothing wraps an
// inject, so every control character goes, newline and tab too: a file committed with a
// newline or an ESC in its name would otherwise send the prompt, or type keys,
// on the agent's behalf.
func safeToWrite(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
}

// injector is what writes into session id's PTY, false with
// no session to take it. The write runs off the UI goroutine.
func (ts *terminals) injector(id string) func(string) bool {
	return func(text string) bool {
		if id == "" {
			return false
		}
		go func() {
			if err := ts.client.Write(context.Background(), id, safeToWrite(text)); err != nil {
				log.Printf("inject: %v", err)
			}
		}()
		return true
	}
}

// submitter is what sends text as session id's prompt, false with no agent
// session to take it: a shell's Enter would run the text as a command. The
// send runs off the UI goroutine; done gets its outcome, and what the backend
// refuses (a session that stopped) is also told in a toast.
func (ts *terminals) submitter(id string) func(string, func(error)) bool {
	return func(text string, done func(error)) bool {
		if id == "" || ts.kindOf(id) == "shell" {
			return false
		}
		go func() {
			err := ts.client.Submit(context.Background(), id, text)
			if err != nil {
				ts.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.commentBatch.sendFailed"), Description: backend.ErrorText(err)})
			}
			done(err)
			ts.invalidate()
		}()
		return true
	}
}

func (ts *terminals) kindOf(id string) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if t, ok := ts.byID[id]; ok {
		return t.kind
	}
	return ""
}

// reviewCommentEntry is InjectMenu's comment on the pull request, offered on
// the Pulls screen: open starts the box that files into the pending review.
func reviewCommentEntry(lines string, open func()) ui.MenuEntry {
	label := i18n.T("diff.injectMenu.reviewComment")
	if lines != "" {
		label = i18n.T("diff.injectMenu.reviewCommentRange", i18n.Args{"lines": lines})
	}
	return ui.MenuEntry{Label: label, Disabled: lines == "", OnClick: open}
}

// injectEntries is the menu over the preview. lines is the selection as
// formatLineRef spells it, "" for none, which disables what needs one.
func injectEntries(rel, lines string, inject func(string) bool, comment func()) []ui.MenuEntry {
	linesLabel, commentLabel := i18n.T("diff.injectMenu.injectLines"), i18n.T("diff.injectMenu.sessionComment")
	if lines != "" {
		linesLabel = i18n.T("diff.injectMenu.injectLinesRange", i18n.Args{"lines": lines})
		commentLabel = i18n.T("diff.injectMenu.sessionCommentRange", i18n.Args{"lines": lines})
	}
	return []ui.MenuEntry{
		{Label: i18n.T("diff.injectMenu.injectFile"), OnClick: func() { inject(fileReference(rel)) }},
		{Label: linesLabel, Disabled: lines == "", OnClick: func() { inject(linesReference(rel, lines)) }},
		{Label: commentLabel, Disabled: lines == "", OnClick: comment},
	}
}
