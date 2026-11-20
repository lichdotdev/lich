package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// peakInk is how far any pixel inside box stands from what the frame's top
// left corner, an empty spot, is drawn in: a row drawn at part of its
// opacity stands closer to what is under it.
func peakInk(img *image.RGBA, box image.Rectangle) uint8 {
	ground := image.NewRGBA(box)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			ground.SetRGBA(x, y, img.RGBAAt(0, 0))
		}
	}
	return inkShift(img, ground, box)
}

// textBox is the first line of label's bounds, w wide: a label's own bounds
// run on to the edge of its constraints.
func textBox(t *testing.T, r *frameRig, label string, w int) image.Rectangle {
	t.Helper()
	const line = 16
	b := labelBox(t, r, label)
	b.Max = image.Pt(min(b.Max.X, b.Min.X+w), min(b.Max.Y, b.Min.Y+line))
	return b
}

// play frames the screen n times, frameStep apart.
func (r *frameRig) play(n int) {
	for range n {
		r.now = r.now.Add(frameStep)
		r.frame()
	}
}

// A message that arrives grows in by its height and opacity from where it
// will stand, never from a smaller size: half way in it stands at its final
// left edge, its row shorter and its text dimmer than at rest.
func TestAChatRowEntersByHeightAndOpacityWithoutScale(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", userTurns(2))
	r.settle(t)
	before := labelTop(t, r, "q0")
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "new", Kind: api.ItemUser, Turn: 3, Text: "fresh"}})
	r.frame()
	r.play(5) // 80ms of the entry's 160
	mid, midTop := labelBox(t, r, "fresh"), labelTop(t, r, "q0")
	midInk := peakInk(renderRig(t, r), textBox(t, r, "fresh", 30))
	r.settle(t)
	rest, after := labelBox(t, r, "fresh"), labelTop(t, r, "q0")
	restInk := peakInk(renderRig(t, r), textBox(t, r, "fresh", 30))
	if mid.Min.X != rest.Min.X {
		t.Errorf("half way in the message starts at x=%d, at rest at x=%d: it grew from a smaller size", mid.Min.X, rest.Min.X)
	}
	if midTop >= before || midTop <= after {
		t.Errorf("the message before it at y=%d, %d half way and %d at rest: want the new row's height growing", before, midTop, after)
	}
	if midInk >= restInk {
		t.Errorf("half way in the text peaks at %d, at rest %d: want it dimmer while it fades in", midInk, restInk)
	}
}

// The live line leaving for a permission plays the entry backwards: it keeps
// its place and size, its row shrinking and its verb dimming. The live line
// keeps its slot over the permission while it goes, so the permission's
// distance from the message above is the live row's height.
func TestAChatRowLeavesByHeightAndOpacityWithoutScale(t *testing.T) {
	items := []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"}}
	u, r := lifeFrames(t, items, true)
	r.frames(3)
	verb := chatVerb("s1", 1)
	r.frame()
	rest := labelBox(t, r, verb)
	restInk := peakInk(renderRig(t, r), textBox(t, r, verb, 40))
	// The flame and the shimmer loop every 1.6s: the frame a lap after this
	// one draws them alike.
	const into = 60 * time.Millisecond
	r.now = r.now.Add(ui.ShimmerLap - into)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "p1", Kind: api.ItemPermission, Turn: 1, Permission: &backend.ChatPermission{
		Tool:    backend.ChatToolCall{Name: "Edit", Title: "a.go", State: api.ToolPending},
		Options: []api.ChatPermitOption{{ID: "yes", Label: "Allow", Kind: api.PermitAllowOnce}},
	}}})
	r.frame()
	gap := func() int { return labelTop(t, r, "Edit") - labelTop(t, r, "go") }
	r.play(1)
	start := gap()
	r.now = r.now.Add(into - frameStep)
	r.frame()
	if !r.shows(verb) {
		t.Fatalf("the live line left at once:\n%s", r.drawn())
	}
	mid, midGap := labelBox(t, r, verb), gap()
	midInk := peakInk(renderRig(t, r), textBox(t, r, verb, 40))
	r.settle(t)
	if r.shows(verb) {
		t.Error("the live line never left")
	}
	if mid.Dx() != rest.Dx() || mid.Dy() != rest.Dy() {
		t.Errorf("%v into the exit the verb is %v, at rest %v: it shrank", into, mid.Size(), rest.Size())
	}
	if end := gap(); midGap >= start || midGap <= end {
		t.Errorf("the permission %dpx under the message, %d at %v, %d once the live line left: want its row shrinking", start, midGap, into, end)
	}
	if midInk >= restInk {
		t.Errorf("%v into the exit the verb peaks at %d, at rest %d: want it dimmer while it fades out", into, midInk, restInk)
	}
}

// While chunks of the answer arrive the caret stays lit, even on the half
// seconds it would blink off; once they stop it blinks again. The caret is
// read off the pixels: the answer drawn live against the same text drawn
// finished, which never has one.
func TestTheCaretStaysLitWhileChunksArrive(t *testing.T) {
	u, _ := stampFrames(t)
	sv, _ := u.model.sessionViewOf("s1")
	text := "Hel"
	answer := func(p *chatPane, live bool) *frameRig {
		row := chatRow{kind: chatRowItem, live: live}
		return newFrameRig(image.Pt(400, 120), func(gtx C, th *ui.Theme) {
			u.theme = th
			row.item = backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: text}
			u.chatAgent(gtx, th, p, sv, row)
		})
	}
	live, done := answer(newChatPane(), true), answer(newChatPane(), false)
	lit := func(now time.Time) bool {
		live.now, done.now = now, now
		live.frame()
		done.frame()
		return inkShift(renderRig(t, live), renderRig(t, done), image.Rectangle{Max: live.size}) > 0
	}
	// 18:00:00.750, a half second the caret would blink off on.
	now := time.Date(2026, 11, 18, 18, 0, 0, 0, time.Local).Add(750 * time.Millisecond)
	for i := range 8 {
		text += "lo"
		now = now.Add(250 * time.Millisecond)
		if !lit(now) {
			t.Fatalf("chunk %d at %v: no caret while the answer is being written", i, now)
		}
	}
	var off bool
	for range 20 {
		now = now.Add(100 * time.Millisecond)
		off = off || !lit(now)
	}
	if !off {
		t.Error("two seconds after the last chunk the caret never blinked off")
	}
}

// The turn ending turns the live line into the closing line in its slot:
// Done stands where the live line's verb was in every frame of the change,
// never under it, and the slot only eases from one line's height to the
// other's (at 1x the flame's 11 rows of 2px are 22px, the avatar 20px).
func TestTheLiveLineTurnsIntoDoneInItsSlot(t *testing.T) {
	items := []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"}}
	u, r := lifeFrames(t, items, true)
	r.frames(3)
	verb, done := chatVerb("s1", 1), i18n.T("chat.done.done")
	centre := func(label string) int { b := labelBox(t, r, label); return (b.Min.Y + b.Max.Y) / 2 }
	top, verbAt := labelTop(t, r, "go"), centre(verb)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1"})
	last := top
	var tops []int
	for i := range 30 {
		r.frame()
		got := labelTop(t, r, "go")
		tops = append(tops, got)
		if (got-last)*(got-top) < 0 || i > 0 && chatSwapHeight <= time.Duration(i-1)*frameStep && got != last {
			t.Errorf("the message above went %v: want it easing one way, within %v", tops, chatSwapHeight)
		}
		if at := centre(done); abs(at-verbAt-(got-top)) > 2 {
			t.Errorf("frame %d: Done centred at y=%d, the verb was at %d and the slot moved by %d: it is not in the verb's place", i, at, verbAt, got-top)
		}
		if i == 3 && !r.shows(verb) {
			t.Errorf("48ms into the change the verb is gone: it does not fade")
		}
		last = got
		r.now = r.now.Add(frameStep)
	}
	if r.shows(verb) {
		t.Error("the verb is still drawn after the change")
	}
}

func abs(n int) int { return max(n, -n) }

// A question answered turns from its card into its answer lines in its
// slot, the row's height running down from the card's to the lines'.
func TestAnAnsweredQuestionTurnsIntoItsLines(t *testing.T) {
	asks := []api.ChatAsk{{Header: "Scope", Text: "Which files?", Options: []api.ChatOffer{{Label: "All"}, {Label: "Some"}}}}
	it := backend.ChatItem{ID: "q1", Kind: api.ItemQuestion, Question: &backend.ChatQuestion{Questions: asks}}
	p := newChatPane()
	var height int
	r := newFrameRig(image.Pt(600, 400), func(gtx C, th *ui.Theme) {
		gtx.Constraints.Min = image.Point{}
		height = chatQuestion(gtx, th, p, it).Size.Y
	})
	r.settle(t)
	card := height
	it.Question = &backend.ChatQuestion{Questions: asks, Reply: &api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"All"}}}}}
	r.frame()
	if height != card || !r.shows("Which files?") {
		t.Errorf("the frame the reply came in: height %d (the card's %d), card drawn %v", height, card, r.shows("Which files?"))
	}
	r.now = r.now.Add(100 * time.Millisecond)
	r.frame()
	mid := height
	if !r.shows("Which files?") || !r.shows("All") {
		t.Errorf("100ms in: want the card fading over the lines:\n%s", r.drawn())
	}
	r.settle(t)
	if mid >= card || mid <= height {
		t.Errorf("height %d, %d at 100ms, %d at rest: want it running down from the card's to the lines'", card, mid, height)
	}
	if r.shows("Which files?") {
		t.Error("the card is still drawn once answered")
	}
}

// A message sent while the list is scrolled up takes the list back to its
// end over a run, not a jump, and pins it there.
func TestSendingFromAboveTheEndRunsTheListBackToIt(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	p := u.chats.pane("s1")
	p.list.Position.First, p.list.Position.Offset, p.list.Position.BeforeEnd = 10, 0, true
	r.settle(t)
	from := p.list.Position.First
	p.editor.SetText("and now?")
	r.press(key.NameReturn)
	r.play(7) // half of the run's 220ms
	mid := p.list.Position.First
	r.play(10)
	end := p.list.Position.First
	if mid <= from || mid >= end {
		t.Errorf("the list's first row %d, %d half way, %d after the run: want it running to the end", from, mid, end)
	}
	if p.list.Position.BeforeEnd || !r.shows("q59") {
		t.Errorf("after the run the list is not at its end: %+v", p.list.Position)
	}
}

// A session switched to the chat fades it in from the background; a chat
// drawn again plays no entries for what came while it was a terminal.
func TestTheChatFadesInWhenTheViewSwitchesToIt(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", userTurns(2))
	r.settle(t)
	placeholder := "Ask claude about this project"
	restInk := peakInk(renderRig(t, r), textBox(t, r, placeholder, 60))
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	u.model.projects[0].Sessions[0].View = api.ViewTerminal
	r.settle(t)
	u.model.chat.history("s1", userTurns(3))
	u.model.projects[0].Sessions[0].View = api.ViewChat
	r.frame()
	if p := u.chats.pane("s1"); p.rows.Moving() || !r.shows("q2") {
		t.Errorf("the chat drawn again plays an entry for what came while it was a terminal")
	}
	r.now = r.now.Add(chatViewFade / 2)
	r.frame()
	midInk := peakInk(renderRig(t, r), textBox(t, r, placeholder, 60))
	if midInk >= restInk {
		t.Errorf("half way into the fade the composer peaks at %d, at rest %d: want it coming up from the background", midInk, restInk)
	}
	r.settle(t)
	if got := peakInk(renderRig(t, r), textBox(t, r, placeholder, 60)); got != restInk {
		t.Errorf("after the fade the composer peaks at %d, want %d", got, restInk)
	}
}

// A fold opens and shuts by ui.Collapse: half way, its text is drawn in a
// row shorter than the open one.
func TestAThoughtFoldOpensOverItsCollapse(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "t1", Kind: api.ItemThought, Turn: 1, At: novDay(18, 17, 58), Text: "pondering"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "answer"},
	}, false)
	r.settle(t)
	shut := labelTop(t, r, "go")
	r.clickLabel(t, i18n.T("chat.view.thought"))
	r.now = r.now.Add(100 * time.Millisecond)
	r.frame()
	mid := labelTop(t, r, "go")
	if !r.shows("pondering") {
		t.Errorf("half way open the thought is not drawn:\n%s", r.drawn())
	}
	r.settle(t)
	open := labelTop(t, r, "go")
	if mid >= shut || mid <= open {
		t.Errorf("the message above at y=%d shut, %d half way, %d open: want the fold growing", shut, mid, open)
	}
	_ = u
}
