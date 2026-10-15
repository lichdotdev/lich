package main

// The inject half of InjectMenu.tsx over the Code tab's preview: the
// right-click menu that writes a reference to the file, or to the lines under
// the selection, into the session's terminal, plus the comment for the
// session that opens the review agent's composer under the selection.

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n; the web's en catalog
// (diff.injectMenu).
const (
	injectFile          = "Inject file"
	injectLines         = "Inject lines"
	injectLinesRange    = "Inject lines %s"
	sessionComment      = "Comment for the session…"
	sessionCommentRange = "Comment for the session %s…"
	reviewCommentLabel  = "Comment on the pull request…"
	reviewCommentRange  = "Comment on the pull request %s…"
)

// formatLineRef is diff.ts formatLineRef over 1-based file lines: a single
// line reads "19", not "19-19".
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

// safeToWrite is bracketed-paste.ts safeToWrite. Nothing wraps an inject, so
// every control character goes, newline and tab too: a file committed with a
// newline or an ESC in its name would otherwise send the prompt, or type keys,
// on the agent's behalf. A block bracketedPaste built (a batch of comments) is
// unwrapped and wrapped again, which runs the paste's rule over its inside
// rather than eating the wrapper's own escapes.
func safeToWrite(text string) string {
	if strings.HasPrefix(text, pasteStart) && strings.HasSuffix(text, pasteEnd) && len(text) >= len(pasteStart)+len(pasteEnd) {
		return bracketedPaste(text[len(pasteStart) : len(text)-len(pasteEnd)])
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
}

// dockInjector is use-inject.ts: what writes into session id's PTY, false
// with no session to take it. The write runs off the UI goroutine.
func dockInjector(client *lichclient.Client, id string) func(string) bool {
	return func(text string) bool {
		if id == "" {
			return false
		}
		go func() {
			if err := client.Write(context.Background(), id, safeToWrite(text)); err != nil {
				log.Printf("inject: %v", err)
			}
		}()
		return true
	}
}

// reviewCommentEntry is InjectMenu's comment on the pull request, offered on
// the Pulls screen: open starts the box that files into the pending review.
func reviewCommentEntry(lines string, open func()) ui.MenuEntry {
	label := reviewCommentLabel
	if lines != "" {
		label = fmt.Sprintf(reviewCommentRange, lines)
	}
	return ui.MenuEntry{Label: label, Disabled: lines == "", OnClick: open}
}

// injectEntries is the menu over the preview. lines is the selection as
// formatLineRef spells it, "" for none, which disables what needs one.
func injectEntries(rel, lines string, inject func(string) bool, comment func()) []ui.MenuEntry {
	linesLabel, commentLabel := injectLines, sessionComment
	if lines != "" {
		linesLabel = fmt.Sprintf(injectLinesRange, lines)
		commentLabel = fmt.Sprintf(sessionCommentRange, lines)
	}
	return []ui.MenuEntry{
		{Label: injectFile, OnClick: func() { inject(fileReference(rel)) }},
		{Label: linesLabel, Disabled: lines == "", OnClick: func() { inject(linesReference(rel, lines)) }},
		{Label: commentLabel, Disabled: lines == "", OnClick: comment},
	}
}
