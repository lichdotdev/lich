package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

var markSeq uint64

func clickMark(at time.Time, x, y float64) browserMark {
	markSeq++
	return browserMark{seq: markSeq, at: at, x: &x, y: &y, action: "click"}
}

func scrollMark(at time.Time, x, y float64, dy int) browserMark {
	m := clickMark(at, x, y)
	m.action, m.dy = "scroll", dy
	return m
}

// atMark places a page mark at its own point, and a toolbar one at toolbarAt.
func atMark(m browserMark) f32.Point {
	if m.x == nil {
		return toolbarAt
	}
	return f32.Pt(float32(*m.x), float32(*m.y))
}

var toolbarAt = f32.Pt(12, 40)

// play feeds marks to c at now and lets it start what it can.
func play(c *agentCursor, now time.Time, marks ...browserMark) {
	c.take(marks)
	c.advance(now, atMark)
}

func TestAgentCursorAppearsAtItsFirstClickAndGlidesToTheNext(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, clickMark(t0, 50, 40))
	if !c.shown || c.position(t0) != f32.Pt(50, 40) {
		t.Fatalf("first click at %v, shown %v", c.position(t0), c.shown)
	}
	t1 := t0.Add(time.Second)
	play(&c, t1, clickMark(t1, 150, 40))
	mid := c.position(t1.Add(cursorGlide / 2))
	if mid.X <= 50 || mid.X >= 150 {
		t.Fatalf("halfway through the glide at %v, want between the two clicks", mid)
	}
	if got := c.position(t1.Add(cursorGlide)); got != f32.Pt(150, 40) {
		t.Fatalf("after the glide at %v, want the new click", got)
	}
}

func TestAgentCursorPlaysParallelActionsInTurn(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	// An agent's click and scroll called in parallel reach the pane between
	// two frames; the click still dips before the scroll plays, and its ring
	// fades on where it was.
	play(&c, t0, clickMark(t0, 40, 40), scrollMark(t0, 40, 40, 500))
	if c.scroll != 0 || c.to != f32.Pt(40, 40) {
		t.Fatalf("playing scroll %d to %v first, want the click", c.scroll, c.to)
	}
	dipped := c.played()
	c.advance(dipped.Add(-time.Millisecond), atMark)
	if c.scroll != 0 {
		t.Fatal("the scroll cut the click's dip short")
	}
	c.advance(dipped, atMark)
	if c.scroll != 1 {
		t.Fatalf("after the click dipped, scroll %d, want the scroll down", c.scroll)
	}
	if len(c.rings) != 1 {
		t.Fatalf("rings %d, want the click's ring still fading", len(c.rings))
	}
}

func TestAgentCursorKeepsOnlyTheLatestPendingActions(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	marks := []browserMark{}
	for i := range 6 {
		marks = append(marks, clickMark(t0, float64(i), 0))
	}
	play(&c, t0, marks...)
	if len(c.pending) != cursorPending-1 || c.to != f32.Pt(6-cursorPending, 0) {
		t.Fatalf("pending %d, playing %v; want the latest %d, the oldest of them playing", len(c.pending), c.to, cursorPending)
	}
}

func TestAgentCursorNeverDropsAnAnswerToTheDialog(t *testing.T) {
	t0 := time.Unix(100, 0)
	accept := clickMark(t0, 0, 0)
	accept.action, accept.x, accept.y = "accept", nil, nil
	pending := []browserMark{accept}
	for i := range 5 {
		pending = append(pending, clickMark(t0, float64(i+1), 0))
	}
	got := trimPending(pending)
	if len(got) != cursorPending+1 || got[0].action != "accept" || *got[1].x != 6-cursorPending {
		t.Fatalf("kept %+v, want the accept and the latest %d clicks", got, cursorPending)
	}
}

func TestAgentCursorStaysUpWhileTheAgentActsAndThenFades(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, clickMark(t0, 10, 10))
	markSeq++
	play(&c, t0.Add(time.Second), browserMark{seq: markSeq, at: t0.Add(time.Second)})
	if a := c.alpha(t0.Add(time.Second + agentHold - time.Millisecond)); a != 1 {
		t.Fatalf("alpha %v before the hold ends, want 1: an action without a point keeps it up", a)
	}
	if a := c.alpha(t0.Add(time.Second + agentHold + cursorFade)); a != 0 {
		t.Fatalf("alpha %v after the fade, want 0", a)
	}
	if c.position(t0.Add(2*time.Second)) != f32.Pt(10, 10) {
		t.Fatal("an action without a point moved the cursor")
	}
}

func TestAgentCursorTakesEachMarkOnce(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	m := clickMark(t0, 10, 10)
	play(&c, t0, m)
	later := t0.Add(time.Second)
	play(&c, later, clickMark(later, 90, 90))
	play(&c, later.Add(time.Second), m)
	if c.to != f32.Pt(90, 90) || len(c.pending) != 0 {
		t.Fatalf("an old mark taken again: playing %v, pending %d", c.to, len(c.pending))
	}
}

func TestAgentCursorScrollShowsItsDirectionInsteadOfAClick(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, scrollMark(t0, 200, 150, -300))
	if c.scroll != -1 || c.to != f32.Pt(200, 150) {
		t.Fatalf("scroll up: scroll %d to %v", c.scroll, c.to)
	}
	later := t0.Add(time.Second)
	play(&c, later, clickMark(later, 10, 10))
	if c.scroll != 0 {
		t.Fatalf("a click after a scroll kept scroll %d", c.scroll)
	}
}

func TestInkOnKeepsTheThemesPairAndReadsOnCardColors(t *testing.T) {
	white, black := color.NRGBA{R: 255, G: 255, B: 255, A: 255}, color.NRGBA{A: 255}
	if got := inkOn(ui.Dark.Primary, ui.Dark.PrimaryForeground); got != ui.Dark.PrimaryForeground {
		t.Fatalf("on the dark theme's primary, ink %v, want its primary-foreground", got)
	}
	for name, c := range cardColors {
		ink := inkOn(c, ui.Dark.PrimaryForeground)
		if ink != white && ink != black && ink != ui.Dark.PrimaryForeground {
			t.Fatalf("%s: ink %v", name, ink)
		}
		if contrast(c, ink) < 3 {
			t.Errorf("%s: badge text contrast %.2f, want at least 3", name, contrast(c, ink))
		}
	}
}

func TestDrawAgentCursorLaysOutWithoutAPage(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Canvas}
	u := &window{theme: th}
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, clickMark(t0, 40, 30))
	for _, d := range []time.Duration{0, cursorGlide, cursorGlide + cursorDip/2, agentHold + cursorFade/2, agentHold + cursorFade} {
		gtx := layout.Context{Ops: new(op.Ops), Now: t0.Add(d), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(400, 300))}
		u.drawAgentCursor(gtx, &c, th.Primary, th.PrimaryForeground, "Claude Code", providerIcon("claude"))
	}
	if c.shown {
		t.Fatal("cursor still shown after it faded out")
	}
}

func TestAgentCursorStaysWhileTheTurnRunsAndHoldsAfterItEnds(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, clickMark(t0, 10, 10))
	thinking := t0.Add(time.Minute)
	c.observe(thinking, true)
	if a := c.alpha(thinking); a != 1 {
		t.Fatalf("alpha %v a minute into the turn, want 1: the agent is thinking between tools", a)
	}
	ended := thinking.Add(time.Second)
	c.observe(ended, false)
	if a := c.alpha(ended.Add(agentHold - time.Millisecond)); a != 1 {
		t.Fatalf("alpha %v within the hold after the turn ended, want 1", a)
	}
	if a := c.alpha(ended.Add(agentHold + cursorFade)); a != 0 {
		t.Fatalf("alpha %v after the hold and the fade, want 0", a)
	}
}

func TestAgentCursorPlaysNavigationOnTheToolbar(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	markSeq++
	back := browserMark{seq: markSeq, at: t0, action: "back"}
	markSeq++
	shot := browserMark{seq: markSeq, at: t0, action: "screenshot"}
	c.take([]browserMark{back, shot})
	m, started := c.advance(t0, atMark)
	if !started || m.action != "back" || c.to != toolbarAt {
		t.Fatalf("started %v %q at %v, want back on the toolbar", started, m.action, c.to)
	}
	if len(c.pending) != 0 {
		t.Fatalf("pending %d: an action that changes nothing on screen was queued", len(c.pending))
	}
}

func TestAgentCursorCatchesUpWithAFastAgent(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	// four fields typed into within a second and a half, as a fast agent fills
	// a form
	play(&c, t0, clickMark(t0, 10, 10), clickMark(t0, 10, 50), clickMark(t0, 10, 90))
	if c.pace != cursorCatchUp {
		t.Fatalf("pace %v with two actions behind, want the catch-up pace", c.pace)
	}
	now := t0
	for len(c.pending) > 0 {
		now = c.played()
		c.advance(now, atMark)
	}
	if c.pace != 1 {
		t.Fatalf("pace %v for the last action, want it played whole", c.pace)
	}
	whole := cursorGlide + cursorRing
	if behind := now.Sub(t0); behind >= 2*whole {
		t.Fatalf("the last action started %v after the first, want well under two whole plays (%v)", behind, 2*whole)
	}
}

func TestAgentCursorGoesOnOnceAClickDippedAndLeavesItsRing(t *testing.T) {
	var c agentCursor
	t0 := time.Unix(100, 0)
	play(&c, t0, clickMark(t0, 10, 10))
	t1 := t0.Add(cursorGlide + cursorDip)
	play(&c, t1, clickMark(t1, 200, 10))
	if c.to != f32.Pt(200, 10) {
		t.Fatalf("playing %v after the first click dipped, want the next click", c.to)
	}
	if len(c.rings) != 1 || c.rings[0].at != f32.Pt(10, 10) {
		t.Fatalf("rings %+v, want the first click's ring left fading at its point", c.rings)
	}
	if !c.settled().After(c.played()) {
		t.Fatal("settled before the rings faded")
	}
}
