package main

import (
	"context"
	"encoding/json"
	"image"
	"net/http"
	"slices"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func talkFixture() *backend.PRConversation {
	return &backend.PRConversation{
		Reviews:  []backend.PRReview{{DatabaseID: 30, Author: "rev", State: "CHANGES_REQUESTED", Body: "A couple of things.", Date: "2026-01-02T00:00:00Z", CanEdit: true}},
		Comments: []backend.PRComment{{ID: 20, Author: "octo", Body: "Thanks!", Date: "2026-01-01T00:00:00Z", CanEdit: true, CanDelete: true}},
		Threads: []backend.PRReviewThread{
			{ID: "T1", Path: "internal/project/pr.go", Line: 61, StartLine: 58, IsOutdated: true, Comments: []backend.PRThreadComment{
				{ID: 1, Author: "rev", Body: "Why not cap this?", Date: "2026-01-02T00:00:00Z", DiffHunk: "@@ -1,3 +1,4 @@\n ctx\n-old\n+new", CanEdit: true, CanDelete: true},
			}},
			{ID: "T2", Path: "a.go", Line: 3, IsResolved: true, Comments: []backend.PRThreadComment{{ID: 2, Author: "rev", Body: "ok", Date: "2026-01-01T00:00:00Z"}}},
		},
	}
}

// layoutTalk lays the Conversation tab out once and answers the frame;
// prepare, when set, puts the tab's widgets in a state first.
func layoutTalk(t *testing.T, conv *backend.PRConversation, prepare func(*talkState)) (*op.Ops, image.Point) {
	t.Helper()
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return conv, http.StatusOK }}
	p := newPullsScreen(newPullsStore(context.Background(), stub.client(t), func() {}), new(ui.Root))
	p.project, p.selected = backend.Project{Path: "/repo"}, 7
	d := backend.PRDetail{Number: 7, URL: "https://github.com/o/r/pull/7"}
	p.view.pendingOf(d.URL).add("a.go", "3-5", "Rename this.")
	if prepare != nil {
		prepare(&p.view.talk)
	}
	p.store.conversation("/repo", 7)
	waitFor(t, "the conversation", func() bool { return p.store.conversation("/repo", 7).val != nil })
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	size := image.Pt(900, 1400)
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Now: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Constraints: layout.Exact(size)}
	var act pullsAction
	rows := p.view.conversationRows(gtx, th, p, d, &act)
	y := 0
	for _, row := range rows {
		stack := op.Offset(image.Pt(0, y)).Push(ops)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(size.X, size.Y-y)}
		y += row(g).Size.Y
		stack.Pop()
	}
	if y == 0 {
		t.Fatal("the tab drew nothing")
	}
	return ops, size
}

func TestConversationTabLaysOutEveryKindOfEntry(t *testing.T) {
	layoutTalk(t, talkFixture(), nil)
}

func TestCommentMenuOffersWhatGitHubAllows(t *testing.T) {
	var s talkState
	s.init()
	labels := func(entries []ui.MenuEntry) []string {
		var out []string
		for _, e := range entries {
			out = append(out, e.Label)
		}
		return out
	}
	edit, del := i18n.T("pulls.reviewThread.edit"), i18n.T("pulls.reviewThread.delete")
	if got := labels(s.commentEntries(commentTarget{kind: reviewSummary, canEdit: true})); !slices.Equal(got, []string{edit}) {
		t.Errorf("review summary menu = %v, want only Edit", got)
	}
	both := s.commentEntries(commentTarget{kind: prComment, canEdit: true, canDelete: true})
	if got := labels(both); !slices.Equal(got, []string{edit, del}) {
		t.Errorf("comment menu = %v, want Edit and Delete", got)
	}
	if both[1].Variant != ui.VariantDestructive {
		t.Error("Delete is not drawn as destructive")
	}
	if got := s.commentEntries(commentTarget{kind: threadComment}); got != nil {
		t.Errorf("menu for a comment the viewer may not touch = %v, want none", labels(got))
	}
}

func TestEditingAConversationEntryLaysOutEveryKind(t *testing.T) {
	conv := talkFixture()
	layoutTalk(t, conv, func(s *talkState) {
		s.startEdit(threadTarget(conv.Threads[0], conv.Threads[0].Comments[0]))
		s.startEdit(commentTarget{kind: prComment, id: 20, body: "Thanks!", canEdit: true})
		s.startEdit(commentTarget{kind: reviewSummary, id: 30, body: "A couple of things.", canEdit: true})
	})
}
