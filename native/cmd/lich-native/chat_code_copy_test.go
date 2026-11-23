package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func TestACodeBlockInAnAnswerCopiesItsCode(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 14, 32), Text: "show me"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 14, 35), Text: "Here:\n\n```go\nfunc main() {\n\tprintln(\"hi\")\n}\n```\n\nDone."},
	})
	r.settle(t)
	// The answer's text has no labels to find it by: walk the pointer down
	// from its header until the block's button, at the column's right, shows.
	top := labelTop(t, r, "claude")
	right := image.Rect(r.size.X/2, top, r.size.X, composerBox(t, r).Min.Y)
	for y := top; clickablesIn(r, right) == 0; y += 8 {
		if y > right.Max.Y {
			t.Fatalf("no copy button over the code block:\n%s", r.drawn())
		}
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(r.size.X/2), float32(y))})
		r.frame()
		r.frame()
	}
	r.queueClick(r.smallestClickIn(t, right))
	r.frame()
	if got, want := clipboardText(t, r), "func main() {\n\tprintln(\"hi\")\n}"; got != want {
		t.Errorf("the code block copied %q, want %q", got, want)
	}
}
