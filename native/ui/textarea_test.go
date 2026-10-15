package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"
)

// textareaRig is a resizable textarea 300 wide with a 200px max-h.
type textareaRig struct {
	motionRig
	editor  widget.Editor
	resize  TextareaResize
	heights []int
}

func (r *textareaRig) frame() int {
	d := r.motionRig.frame(func(gtx C) D {
		gtx.Constraints = layout.Constraints{Max: image.Pt(300, 200)}
		ta := testTheme().Textarea(&r.editor, "")
		ta.Resize = &r.resize
		return ta.Layout(gtx)
	})
	r.heights = append(r.heights, d.Size.Y)
	return d.Size.Y
}

func (r *textareaRig) drag(from image.Point, ys ...int) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(from)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(from)},
	)
	r.frame()
	for _, y := range ys {
		r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(from.X, y))})
		r.frame()
	}
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(image.Pt(from.X, ys[len(ys)-1]))})
	r.frame()
}

// The grip in the bottom-right corner drags the height, which keeps
// following the pointer as the grip moves with the edge it drags.
func TestTextareaGripDragsTheHeight(t *testing.T) {
	var r textareaRig
	if got := r.frame(); got != 64 {
		t.Fatalf("before a drag: height %d, want min-h-16 (64)", got)
	}
	r.drag(image.Pt(296, 60), 90, 120)
	if got := r.frame(); got != 124 {
		t.Errorf("dragged 60px down: height %d, want 124 (heights %v)", got, r.heights)
	}
}

// A dragged height holds against the text: more lines scroll instead of
// growing the box, as an inline height does over field-sizing-content.
func TestTextareaDraggedHeightHoldsAgainstTheText(t *testing.T) {
	var r textareaRig
	r.frame()
	r.drag(image.Pt(296, 60), 100)
	before := r.frame()
	r.editor.SetText(strings.Repeat("line\n", 20))
	if got := r.frame(); got != before {
		t.Errorf("twenty lines changed a dragged height from %d to %d", before, got)
	}
}

// The drag stops at min-h-16 and at the max-h the caller set.
func TestTextareaGripClampsTheHeight(t *testing.T) {
	var r textareaRig
	r.frame()
	r.drag(image.Pt(296, 60), 0)
	if got := r.frame(); got != 64 {
		t.Errorf("dragged up past the floor: height %d, want 64", got)
	}
	r.drag(image.Pt(296, 60), 900)
	if got := r.frame(); got != 200 {
		t.Errorf("dragged past max-h: height %d, want 200", got)
	}
}

// Without a TextareaResize the corner is the field: a press there focuses
// the editor and drags nothing.
func TestTextareaWithoutResizeHasNoGrip(t *testing.T) {
	var r motionRig
	var ed widget.Editor
	layoutIt := func() int {
		return r.frame(func(gtx C) D {
			gtx.Constraints = layout.Constraints{Max: image.Pt(300, 200)}
			return testTheme().Textarea(&ed, "").Layout(gtx)
		}).Size.Y
	}
	layoutIt()
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(296, 60))},
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(296, 150))},
	)
	if got := layoutIt(); got != 64 {
		t.Errorf("a drag on the corner resized a textarea without a grip to %d", got)
	}
}
