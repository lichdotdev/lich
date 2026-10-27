package main

import (
	"context"
	"image"
	"reflect"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

func TestBrowserQueueMergesWhatPilesUpBehindACall(t *testing.T) {
	q := newBrowserQueue()
	var sent []string
	call := func(m api.BrowserMouseOptions) browserCall {
		return browserCall{mouse: &m, send: func(context.Context) error {
			sent = append(sent, m.Type)
			return nil
		}}
	}
	q.push(call(api.BrowserMouseOptions{Type: "move", X: 1}))
	q.push(call(api.BrowserMouseOptions{Type: "move", X: 2}))
	q.push(call(api.BrowserMouseOptions{Type: "press"}))
	q.push(call(api.BrowserMouseOptions{Type: "wheel", DY: 10}))
	q.push(call(api.BrowserMouseOptions{Type: "wheel", DY: 15}))
	q.push(browserCall{send: func(context.Context) error { sent = append(sent, "key"); return nil }})
	q.push(call(api.BrowserMouseOptions{Type: "move", X: 3}))

	if len(q.calls) != 5 || q.calls[0].mouse.X != 2 || q.calls[2].mouse.DY != 25 {
		t.Fatalf("queued %d calls: first move x=%v wheel dy=%v", len(q.calls), q.calls[0].mouse.X, q.calls[2].mouse.DY)
	}
	for c, ok := q.next(); ok; c, ok = q.next() {
		if err := c.send(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"move", "press", "wheel", "key", "move"}; !reflect.DeepEqual(sent, want) {
		t.Fatalf("sent %v, want %v", sent, want)
	}
}

func TestBrowserQueueKeepsOnlyTheLastPendingResize(t *testing.T) {
	q := newBrowserQueue()
	var sizes []int
	for w := 1; w <= 3; w++ {
		q.push(browserCall{resize: true, send: func(context.Context) error { sizes = append(sizes, w); return nil }})
	}
	for c, ok := q.next(); ok; c, ok = q.next() {
		_ = c.send(context.Background())
	}
	if !reflect.DeepEqual(sizes, []int{3}) {
		t.Fatalf("sent sizes %v, want the last one", sizes)
	}
}

func TestToolbarPointsFollowTheToolbarsLayout(t *testing.T) {
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	back, fwd, reload, addr := toolbarPoint(gtx, "back", 30, 36), toolbarPoint(gtx, "forward", 30, 36), toolbarPoint(gtx, "reload", 30, 36), toolbarPoint(gtx, "navigate", 30, 36)
	side := float32(gtx.Dp(ui.Space(6)))
	if fwd.X-back.X != side || reload.X-fwd.X != side || addr.X <= reload.X+side/2 {
		t.Fatalf("back %v forward %v reload %v address %v: buttons one side apart, the address after them", back, fwd, reload, addr)
	}
	if back.Y != 48 || addr.Y != 48 {
		t.Fatalf("y %v, want the toolbar's middle under the header", back.Y)
	}
}

func TestTypedAddressGrowsAfterTheCursorLands(t *testing.T) {
	t0 := time.Unix(100, 0)
	p := &browserPaneUI{typing: agentPress{what: "example.com", at: t0}}
	gtx := layout.Context{Ops: new(op.Ops), Now: t0.Add(-time.Millisecond)}
	if text, typing := p.typed(gtx); text != "" || !typing {
		t.Fatalf("before landing: %q %v", text, typing)
	}
	gtx.Now = t0.Add(3 * typingStep)
	if text, _ := p.typed(gtx); text != "exam" {
		t.Fatalf("three steps in: %q", text)
	}
	gtx.Now = t0.Add(typingMost)
	if text, typing := p.typed(gtx); text != "example.com" || typing {
		t.Fatalf("done: %q %v", text, typing)
	}
}

func TestToolbarButtonFlashesOnceTheCursorLands(t *testing.T) {
	t0 := time.Unix(100, 0)
	p := &browserPaneUI{flash: agentPress{what: "back", at: t0}}
	gtx := layout.Context{Ops: new(op.Ops), Now: t0.Add(toolbarFlash / 2)}
	if !p.flashing(gtx, "back") || p.flashing(gtx, "forward") {
		t.Fatal("back should be lit, forward not")
	}
	gtx.Now = t0.Add(toolbarFlash)
	if p.flashing(gtx, "back") {
		t.Fatal("back still lit after the flash")
	}
}

func TestPagePointStaysInsideThePage(t *testing.T) {
	page := image.Rect(0, 66, 800, 666)
	if got := pagePoint(100, 50, 1, page); got != f32.Pt(100, 116) {
		t.Fatalf("inside: %v", got)
	}
	if got := pagePoint(475, 715, 1, page); got.Y != 665 {
		t.Fatalf("below the page: %v, want kept at its bottom edge", got)
	}
	if got := pagePoint(-5, 10, 2, page); got.X != 0 || got.Y != 86 {
		t.Fatalf("left of the page at scale 2: %v", got)
	}
}
