package main

import (
	"io"
	"path/filepath"
	"slices"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// A message's right-click menu copies it, or the conversation up to it as
// Markdown, and quotes its selection. The Markdown is in the interface
// language, not the prompt language: lich types none of it into a session,
// the person pastes it where they choose, and its tool lines are the
// sentences the fold they copied it from shows.

// chatSpeaker is whose section of the copied conversation a row goes in.
type chatSpeaker uint8

const (
	speakerYou   chatSpeaker = iota // the person's message, under its own heading
	speakerAgent                    // the agent's text, calls and edits, under the agent's heading
	speakerAside                    // a compaction: no heading, and the next row starts a section
	speakerSame                     // the person's shell command: in the section it follows
)

// messageMenu lays out message w under its right-click menu: Copy takes text,
// as the copy button does; the conversation up to row rowKey; and Quote while
// the body drawn under mdKey holds a selection. Both copies show the copy
// button's check.
func (u *window) messageMenu(gtx C, th *ui.Theme, p *chatPane, sv sessionView, c *chatCopy, rowKey, mdKey, text string, w layout.Widget) D {
	// The press on Quote lets the selection go before the menu acts: Quote
	// takes what was selected when the menu opened.
	wasOpen := c.menu.Opened()
	var copying, quoting string
	entries := func() []ui.MenuEntry {
		if !wasOpen {
			c.quoted = p.md.Selection(mdKey)
		}
		e := []ui.MenuEntry{
			{Label: i18n.T("chat.copy.menu.copy"), OnClick: func() { copying = text }},
			{Label: i18n.T("chat.copy.menu.upToHere"), OnClick: func() { copying = u.conversationUpTo(p, sv, rowKey) }},
		}
		if c.quoted != "" {
			quoted := c.quoted
			e = append(e, ui.MenuEntry{Kind: ui.MenuSeparator}, ui.MenuEntry{Label: i18n.T("chat.quote.add"), OnClick: func() { quoting = quoted }})
		}
		return e
	}
	// The menu opens from the message's hover area, which passes presses on:
	// a trigger would clip the message to its bounds, and Quote over a
	// selection stands above them.
	d := w(gtx)
	if c.rightPressed && !wasOpen {
		c.menu.OpenFrom(&u.root, c.rightAt)
	}
	c.rightPressed = false
	if c.menu.Opened() {
		th.ContextMenuLazy(&u.root, &c.menu, entries).Layout(gtx, func(C) D { return D{} })
	}
	if copying != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(copying))})
		c.copiedAt = gtx.Now
		gtx.Execute(op.InvalidateCmd{})
	}
	if quoting != "" {
		p.addQuote(quoting)
		p.md.DropSelection()
		gtx.Execute(key.FocusCmd{Tag: &p.editor})
		gtx.Execute(op.InvalidateCmd{})
	}
	return d
}

// conversationUpTo is sv's conversation as Markdown, from its start to row
// key and nothing after it; "" when the row is no longer in it.
func (u *window) conversationUpTo(p *chatPane, sv sessionView, key string) string {
	i := slices.IndexFunc(p.drawn, func(r chatRow) bool { return chatRowKey(r) == key })
	if i < 0 {
		return ""
	}
	live := -1
	if items := u.model.chat.of(sv.ID); u.model.chat.isRunning(sv.ID) && len(items) > 0 {
		live = items[len(items)-1].Turn
	}
	return chatMarkdown(p.drawn[:i+1], u.providerName(sv), sv.path, live)
}

// chatMarkdown is rows as Markdown: each message under a heading, "You" or
// the provider with the model that wrote it, the texts as they are, a run of
// calls as its fold's sentence and an edit as its path and lines, with no
// output. An agent's section of turn live, the one still streaming, ends
// marked incomplete. checkout makes the edits' paths relative.
func chatMarkdown(rows []chatRow, provider, checkout string, live int) string {
	var blocks []string
	agent, turn := false, 0
	for i, r := range rows {
		block, who := chatRowMarkdown(r, checkout)
		if block == "" {
			continue
		}
		switch who {
		case speakerYou:
			blocks, agent = append(blocks, "## "+i18n.T("chat.copy.you")), false
		case speakerAgent:
			if !agent {
				blocks, agent = append(blocks, "## "+agentHeading(provider, sectionModel(rows[i:]))), true
			}
		case speakerAside:
			agent = false
		}
		blocks, turn = append(blocks, block), chatRowTurn(r)
	}
	if agent && turn == live {
		blocks = append(blocks, i18n.T("chat.copy.incomplete"))
	}
	return strings.Join(blocks, "\n\n")
}

// chatRowMarkdown is row r's Markdown and whose section it goes in; "" for a
// row the copy leaves out: a thought, a notice, a question, a day or a turn's
// live or closing line.
func chatRowMarkdown(r chatRow, checkout string) (string, chatSpeaker) {
	noVeto := func(backend.ChatItem) bool { return false }
	switch r.kind {
	case chatRowWork:
		label, _ := callSummary(r.tools, noVeto)
		return label, speakerAgent
	case chatRowShell:
		label, _ := callSummary([]backend.ChatItem{r.item}, noVeto)
		return label, speakerSame
	case chatRowEdit:
		t := r.item.Tool
		if t == nil || t.State == api.ToolFailed {
			return "", speakerAgent
		}
		_, added, deleted, _ := splitDiff(t.Diff)
		return i18n.T("chat.copy.edited", i18n.Args{"path": editPath(t, checkout), "added": added, "deleted": deleted}), speakerAgent
	case chatRowItem:
		switch it := r.item; it.Kind {
		case api.ItemUser:
			return userMessageMarkdown(it), speakerYou
		case api.ItemAgent:
			return it.Text, speakerAgent
		case api.ItemCompaction:
			if it.Compaction != nil {
				title, _ := chatCompactTitle(it.Compaction)
				return title, speakerAside
			}
		}
	}
	return "", speakerAside
}

// userMessageMarkdown is the person's message: the text, then each file it attached
// by name and a line saying the files themselves were not copied.
func userMessageMarkdown(it backend.ChatItem) string {
	parts := make([]string, 0, 3)
	if it.Text != "" {
		parts = append(parts, it.Text)
	}
	if len(it.Files) > 0 {
		names := make([]string, len(it.Files))
		for i, f := range it.Files {
			names[i] = "- " + filepath.Base(f)
		}
		parts = append(parts, strings.Join(names, "\n"), i18n.T("chat.copy.attachments", i18n.Args{"count": len(it.Files)}))
	}
	return strings.Join(parts, "\n\n")
}

// agentHeading is the provider, and the model in parentheses when known.
func agentHeading(provider, model string) string {
	if model == "" {
		return provider
	}
	return provider + " (" + model + ")"
}

// sectionModel is the model the agent's section opening rows says it was
// written by: the first its rows name before the next message.
func sectionModel(rows []chatRow) string {
	for _, r := range rows {
		if r.kind == chatRowItem && (r.item.Kind == api.ItemUser || r.item.Kind == api.ItemCompaction) {
			return ""
		}
		if r.item.Model != "" {
			return r.item.Model
		}
	}
	return ""
}

// chatRowTurn is the turn row r belongs to.
func chatRowTurn(r chatRow) int {
	if r.kind == chatRowItem {
		return r.item.Turn
	}
	return r.turn
}
