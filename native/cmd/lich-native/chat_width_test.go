package main

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// composerBox is the composer's box: the widest clickable in the foot of the
// pane.
func composerBox(t *testing.T, r *frameRig) image.Rectangle {
	t.Helper()
	var best image.Rectangle
	for _, n := range r.nodes() {
		if b := n.Desc.Bounds; n.Desc.Gestures != 0 && b.Min.Y > 300 && b.Dx() > best.Dx() {
			best = b
		}
	}
	if best.Empty() {
		t.Fatalf("no composer box:\n%s", r.drawn())
	}
	return best
}

func TestTheComposerFillsThePaneLessItsEdgeUpToTheCeiling(t *testing.T) {
	for _, tc := range []struct{ pane, want int }{{1200, 960}, {1000, 952}, {700, 652}} {
		_, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(tc.pane, 800), busySession())
		r.settle(t)
		box := composerBox(t, r)
		if box.Dx() != tc.want {
			t.Errorf("a pane %dpx wide: the box is %dpx, want %d (the pane less 24px each side, to 960)", tc.pane, box.Dx(), tc.want)
		}
		if axis := (box.Min.X + box.Max.X) / 2; axis != tc.pane/2 {
			t.Errorf("a pane %dpx wide: the box is centred on x=%d, want %d", tc.pane, axis, tc.pane/2)
		}
	}
}

func TestTheConversationSitsInsideTheBoxByTheTextInset(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 14, 32), Text: "hi"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 14, 35), Text: "hello"},
	})
	r.settle(t)
	box := composerBox(t, r)
	if left, want := labelLeft(t, r, clockFor(r, novDay(18, 14, 35))), box.Min.X+6; left != want {
		t.Errorf("an answer's text starts at x=%d, want %d: 6px inside the box's left edge", left, want)
	}
	// The person's message is against the column's right edge, so its time
	// ends 6px inside the box's right edge: it starts well right of the middle.
	if x := labelLeft(t, r, clockFor(r, novDay(18, 14, 32))); x < (box.Min.X+box.Max.X)/2 {
		t.Errorf("the message's time starts at x=%d, left of the box's middle", x)
	}
}

func TestTheComposerAsksInTheInterfaceFontAtTheBodySize(t *testing.T) {
	_, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	want := i18n.T("chat.composer.placeholder", i18n.Args{"provider": "claude"})
	if want != "Ask claude about this project" || !r.shows(want) {
		t.Fatalf("the composer asks %q, drawn %v:\n%s", want, r.shows(want), r.drawn())
	}
	// Its text starts a Space(4) in from the box's left and under the top by a
	// Space(3.5), like the body text it will become.
	box := composerBox(t, r)
	if in := labelLeft(t, r, want) - box.Min.X; in < 14 || in > 17 {
		t.Errorf("the placeholder starts %dpx in from the box's edge, want 16", in)
	}
}

// The conversation ends 16px above the composer's box, resting on it or pinned
// to its end.
func TestTheConversationEndsSixteenPixelsAboveTheBox(t *testing.T) {
	u, r := stampFrames(t)
	at := novDay(18, 14, 35)
	gap := func() int {
		stamp := clockFor(r, at)
		for _, n := range r.nodes() {
			if n.Desc.Label == stamp {
				return composerBox(t, r).Min.Y - (n.Desc.Bounds.Min.Y + 16)
			}
		}
		t.Fatalf("no time %q:\n%s", stamp, r.drawn())
		return 0
	}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: at, Text: "short"}})
	r.settle(t)
	if g := gap(); g != 16 {
		t.Errorf("a short conversation ends %dpx above the box, want 16", g)
	}
	items := userTurns(60)
	items = append(items, backend.ChatItem{ID: "a2", Kind: api.ItemAgent, Turn: 61, At: at, Text: "last"})
	u.model.chat.history("s1", items)
	r.settle(t)
	if g := gap(); g != 16 {
		t.Errorf("a long conversation, pinned, ends %dpx above the box, want 16", g)
	}
}
