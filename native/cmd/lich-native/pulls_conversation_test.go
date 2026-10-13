package main

import (
	"context"
	"encoding/json"
	"image"
	"net/http"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

func talkFixture() *lichclient.PRConversation {
	return &lichclient.PRConversation{
		Reviews:  []lichclient.PRReview{{Author: "rev", State: "CHANGES_REQUESTED", Body: "A couple of things.", Date: "2026-01-02T00:00:00Z"}},
		Comments: []lichclient.PRComment{{Author: "octo", Body: "Thanks!", Date: "2026-01-01T00:00:00Z"}},
		Threads: []lichclient.PRReviewThread{
			{ID: "T1", Path: "internal/project/pr.go", Line: 61, StartLine: 58, IsOutdated: true, Comments: []lichclient.PRThreadComment{
				{ID: 1, Author: "rev", Body: "Why not cap this?", Date: "2026-01-02T00:00:00Z", DiffHunk: "@@ -1,3 +1,4 @@\n ctx\n-old\n+new"},
			}},
			{ID: "T2", Path: "a.go", Line: 3, IsResolved: true, Comments: []lichclient.PRThreadComment{{ID: 2, Author: "rev", Body: "ok", Date: "2026-01-01T00:00:00Z"}}},
		},
	}
}

// layoutTalk lays the Conversation tab out once and answers the frame.
func layoutTalk(t *testing.T, conv *lichclient.PRConversation) (*op.Ops, image.Point) {
	t.Helper()
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return conv, http.StatusOK }}
	p := newPullsScreen(newPullsStore(context.Background(), stub.client(t), func() {}), new(ui.Root))
	p.project, p.selected = lichclient.Project{Path: "/repo"}, 7
	d := lichclient.PRDetail{Number: 7, URL: "https://github.com/o/r/pull/7"}
	p.view.pendingOf(d.URL).add("a.go", "3-5", "Rename this.")
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
	layoutTalk(t, talkFixture())
}
