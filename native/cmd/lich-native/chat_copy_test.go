package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func copyFrames(t *testing.T) (*window, *frameRig, string, string) {
	t.Helper()
	u, r := stampFrames(t)
	userAt, agentAt := novDay(18, 14, 32), novDay(18, 14, 35)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: userAt, Text: "fix **it**"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: agentAt, Text: "Done: `x` and **y**"},
	})
	r.settle(t)
	return u, r, clockFor(r, userAt), clockFor(r, agentAt)
}

// copyArea is where the copy button of the message whose time reads stamp
// stands: left of the time for the person's message, right of it for an answer.
func copyArea(t *testing.T, r *frameRig, stamp string, left bool) image.Rectangle {
	t.Helper()
	x, y := labelLeft(t, r, stamp), labelTop(t, r, stamp)
	if left {
		return image.Rect(x-34, y-8, x, y+24)
	}
	return image.Rect(x+32, y-8, x+90, y+24)
}

func clickablesIn(r *frameRig, within image.Rectangle) int {
	n := 0
	for _, node := range r.nodes() {
		if b := node.Desc.Bounds; node.Desc.Gestures != 0 && b.In(within) {
			n++
		}
	}
	return n
}

func overMessage(r *frameRig, stamp string, t *testing.T) {
	t.Helper()
	x, y := labelLeft(t, r, stamp), labelTop(t, r, stamp)
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(x+4), float32(y+4))})
	r.frame()
	r.frame()
}

// overAnswer brings the pointer onto the answer, by its header, which an
// answer's foot is shown under.
func overAnswer(r *frameRig, t *testing.T) {
	t.Helper()
	x, y := labelLeft(t, r, "claude"), labelTop(t, r, "claude")
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(x+4), float32(y+4))})
	r.frame()
	r.frame()
}

func TestTheCopyButtonShowsOnlyWhileThePointerIsOverTheMessage(t *testing.T) {
	_, r, user, agent := copyFrames(t)
	userTop, box := labelTop(t, r, user), composerBox(t, r)
	if clickablesIn(r, copyArea(t, r, user, true)) != 0 || r.shows(agent) {
		t.Fatal("a copy button, or an answer's time, at rest")
	}
	overMessage(r, user, t)
	if clickablesIn(r, copyArea(t, r, user, true)) == 0 {
		t.Errorf("no copy button left of the person's time while the pointer is over it:\n%s", r.drawn())
	}
	overAnswer(r, t)
	if !r.shows(agent) || clickablesIn(r, copyArea(t, r, agent, false)) == 0 {
		t.Error("no time and no copy button under an answer while the pointer is over it")
	}
	if labelTop(t, r, user) != userTop || composerBox(t, r) != box {
		t.Error("the answer's foot pushed the layout")
	}
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(5, 5)})
	r.frame()
	r.frame()
	if r.shows(agent) {
		t.Error("the answer's foot stayed after the pointer left")
	}
}

func clipboardText(t *testing.T, r *frameRig) string {
	t.Helper()
	_, data, ok := r.router.WriteClipboard()
	if !ok {
		t.Fatal("nothing was put on the clipboard")
	}
	return string(data)
}

func TestTheCopyButtonCopiesTheWholeTextOfItsMessage(t *testing.T) {
	_, r, user, agent := copyFrames(t)
	overMessage(r, user, t)
	r.queueClick(r.smallestClickIn(t, copyArea(t, r, user, true)))
	r.frame()
	if got := clipboardText(t, r); got != "fix **it**" {
		t.Errorf("the person's message copied %q", got)
	}
	overAnswer(r, t)
	r.queueClick(r.smallestClickIn(t, copyArea(t, r, agent, false)))
	r.frame()
	if got := clipboardText(t, r); got != "Done: `x` and **y**" {
		t.Errorf("an answer copied %q, want its Markdown source", got)
	}
}

func TestTheCopyButtonHasNoTooltipAndShowsACheckForASecondAndAHalf(t *testing.T) {
	u, r, user, _ := copyFrames(t)
	overMessage(r, user, t)
	button := r.smallestClickIn(t, copyArea(t, r, user, true))
	r.queueClick(button)
	r.frame()
	c := u.chats.pane("s1").copyOf("u1")
	if !c.done(r.now) {
		t.Fatal("no check right after copying")
	}
	// Left alone on the button, no tooltip comes up.
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(5, 5)})
	r.frame()
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: button})
	r.frame()
	r.now = r.now.Add(800 * time.Millisecond)
	r.frame()
	if r.shows("Copy") || r.shows("Copied") {
		t.Errorf("the copy button grew a tooltip:\n%s", r.drawn())
	}
	if !c.done(r.now) {
		t.Error("the check is gone before a second and a half")
	}
	r.now = r.now.Add(2 * time.Second)
	r.frame()
	if c.done(r.now) {
		t.Error("the check stayed past a second and a half")
	}
}
