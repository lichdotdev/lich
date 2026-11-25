package main

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestChatMarkdownWritesEachMessageUnderItsHeading(t *testing.T) {
	ask := backend.ChatItem{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "Fix the **flake**"}
	answer := backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "The listener comes `late`.", Model: "Opus 5.5"}
	call := func(id, name string) backend.ChatItem {
		return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: name, Title: "go test ./...", State: api.ToolDone, Output: "ok"}}
	}
	edit := backend.ChatItem{ID: "e1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Edit", State: api.ToolDone,
		Diff: "--- a/internal/x.go\n+++ b/internal/x.go\n@@ -1,2 +1,3 @@\n-old\n+new\n+more\n same\n"}}
	cases := map[string]struct {
		items []backend.ChatItem
		live  int
		want  string
	}{
		"a turn": {[]backend.ChatItem{ask, answer},
			-1, "## You\n\nFix the **flake**\n\n## Claude Code (Opus 5.5)\n\nThe listener comes `late`."},
		"no model known": {[]backend.ChatItem{ask, {ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "Done."}},
			-1, "## You\n\nFix the **flake**\n\n## Claude Code\n\nDone."},
		"a run of calls, no output": {[]backend.ChatItem{ask, call("t1", "Bash"), call("t2", "Bash"), call("t3", "Read"), answer},
			-1, "## You\n\nFix the **flake**\n\n## Claude Code (Opus 5.5)\n\nRan 2 commands, read 1 file\n\nThe listener comes `late`."},
		"an edit": {[]backend.ChatItem{ask, edit, answer},
			-1, "## You\n\nFix the **flake**\n\n## Claude Code (Opus 5.5)\n\nEdited internal/x.go (+2 −1)\n\nThe listener comes `late`."},
		"attachments by name": {[]backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "look", Files: []string{"/tmp/a/shot.png", "/tmp/log.txt"}}},
			-1, "## You\n\nlook\n\n- shot.png\n- log.txt\n\nThe attached files were not copied, only their names."},
		"a thought is left out": {[]backend.ChatItem{ask, {ID: "th", Kind: api.ItemThought, Turn: 1, Text: "hmm"}, answer},
			-1, "## You\n\nFix the **flake**\n\n## Claude Code (Opus 5.5)\n\nThe listener comes `late`."},
		"the turn streaming": {[]backend.ChatItem{ask, answer},
			1, "## You\n\nFix the **flake**\n\n## Claude Code (Opus 5.5)\n\nThe listener comes `late`.\n\n(incomplete)"},
		"a compaction, then a new section": {[]backend.ChatItem{ask, answer,
			{ID: "c1", Kind: api.ItemCompaction, Turn: 1, Compaction: &api.ChatCompaction{State: api.CompactionDone}},
			{ID: "a2", Kind: api.ItemAgent, Turn: 1, Text: "Carrying on."}},
			-1, "## You\n\nFix the **flake**\n\n## Claude Code (Opus 5.5)\n\nThe listener comes `late`.\n\nConversation compacted\n\n## Claude Code\n\nCarrying on."},
	}
	for name, c := range cases {
		rows := chatRows(c.items, claudeKind, false, time.Unix(1000, 0))
		if got := chatMarkdown(rows, "Claude Code", "", c.live); got != c.want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", name, got, c.want)
		}
	}
}

// copyMenuFrames is s1 of kind with two turns, the first answered by
// model.
func copyMenuFrames(t *testing.T, kind string) (*window, *frameRig) {
	t.Helper()
	u, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(720, 760), busySession())
	u.model.projects[0].Sessions[0].Kind = kind
	u.model.chat.kinds = []string{kind}
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: selectAsk},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "It holds.", Model: "Opus 5.5"},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: "And after?"},
		{ID: "a2", Kind: api.ItemAgent, Turn: 2, Text: "Later words."},
	})
	r.settle(t)
	return u, r
}

func (r *frameRig) rightClick(at f32.Point) {
	r.hover(at)
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonSecondary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	r.frame()
}

// clickMenu clicks the menu's row drawn as label on its first glyph: the
// message under the menu is a smaller clickable there than the row.
func (r *frameRig) clickMenu(t *testing.T, label string) {
	t.Helper()
	g := r.glyph(t, label)
	r.click(t, f32.Pt(float32(g.X), float32(g.Y)))
}

// The menu of the person's second message copies the conversation down to
// it, the answer under the provider's name, and nothing after it, for each of
// the chat's providers.
func TestCopyConversationUpToHereStopsAtTheClickedMessage(t *testing.T) {
	for _, kind := range []string{claudeKind, codexKind, opencodeKind} {
		t.Run(kind, func(t *testing.T) {
			u, r := copyMenuFrames(t, kind)
			sv, _ := u.model.sessionViewOf("s1")
			r.rightClick(textStart(t, r, labelBox(t, r, "And after?").Min))
			if r.shows(i18n.T("chat.quote.add")) {
				t.Error("Quote in the menu with nothing selected")
			}
			r.settle(t)
			writeShot(t, r, "shot-chat-copy-menu-"+kind+".png")
			r.clickMenu(t, i18n.T("chat.copy.menu.upToHere"))
			want := "## You\n\n" + selectAsk + "\n\n## " + u.providerName(sv) + " (Opus 5.5)\n\nIt holds.\n\n## You\n\nAnd after?"
			if got := clipboardText(t, r); got != want {
				t.Errorf("copied:\n%s\nwant:\n%s", got, want)
			}
			if !u.chats.pane("s1").copyOf("u2").done(r.now) {
				t.Error("the copy button shows no check after the menu copied")
			}
		})
	}
}

// Copy takes the message as its copy button does, and Quote comes into the
// menu over a selection and takes it into the composer.
func TestTheMessageMenuCopiesAndQuotes(t *testing.T) {
	for _, kind := range []string{claudeKind, codexKind, opencodeKind} {
		t.Run(kind, func(t *testing.T) {
			u, r := copyMenuFrames(t, kind)
			p := u.chats.pane("s1")
			start := textStart(t, r, labelBox(t, r, selectAsk).Min.Add(image.Pt(48, 0)))
			r.rightClick(start)
			r.clickMenu(t, i18n.T("chat.copy.menu.copy"))
			if got := clipboardText(t, r); got != selectAsk {
				t.Errorf("Copy took %q, want %q", got, selectAsk)
			}
			r.dragAcross(start, start.Add(f32.Pt(90, 0)))
			r.frame()
			r.rightClick(start.Add(f32.Pt(20, 0)))
			r.settle(t)
			writeShot(t, r, "shot-chat-copy-menu-quote-"+kind+".png")
			// the floating Quote over the selection is drawn as Quote too: the
			// menu's is the one under its other rows
			below := r.glyph(t, i18n.T("chat.copy.menu.upToHere")).Y
			var quotes []image.Point
			for _, n := range r.nodes() {
				if n.Desc.Label == i18n.T("chat.quote.add") {
					quotes = append(quotes, n.Desc.Bounds.Min)
				}
			}
			i := slices.IndexFunc(quotes, func(at image.Point) bool { return at.Y > below })
			if len(quotes) != 2 || i < 0 {
				t.Fatalf("Quote drawn at %v, want the floating one and the menu's under %d", quotes, below)
			}
			r.click(t, f32.Pt(float32(quotes[i].X+4), float32(quotes[i].Y+4)))
			if len(p.quotes) != 1 || len(p.quotes[0]) < 5 || !strings.HasPrefix("Can you check app.go before lunch?", p.quotes[0]) {
				t.Fatalf("quotes %q, want the selection's text", p.quotes)
			}
			if !r.router.Source().Focused(&p.editor) {
				t.Error("the composer does not have the keys after Quote")
			}
		})
	}
}
