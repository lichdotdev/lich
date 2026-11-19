package ui

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"
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

// scrollRig is a textarea with a 120px max-h and a scrollbar, in a column
// that sets no height of its own, as a scrolling list does.
type scrollRig struct {
	motionRig
	editor widget.Editor
	bar    widget.Scrollbar
}

func (r *scrollRig) frame() int {
	d := r.motionRig.frame(func(gtx C) D {
		gtx.Constraints = layout.Constraints{Max: image.Pt(300, 1<<24)}
		ta := testTheme().Textarea(&r.editor, "")
		ta.MaxHeight, ta.Scrollbar = 120, &r.bar
		return ta.Layout(gtx)
	})
	return d.Size.Y
}

// Past MaxHeight the box stops growing and the text scrolls inside it.
func TestTextareaMaxHeightCapsTheBox(t *testing.T) {
	var r scrollRig
	r.editor.SetText(strings.Repeat("line\n", 40))
	r.frame()
	if got := r.frame(); got != 120 {
		t.Fatalf("40 lines under max-h 120: height %d, want 120", got)
	}
	if r.editor.ScrollBounds().Max.Y <= 0 {
		t.Error("the text that does not fit has nowhere to scroll to")
	}
}

// Dragging the bar's thumb down scrolls the text, as a list's does.
func TestTextareaScrollbarDragScrollsTheText(t *testing.T) {
	var r scrollRig
	r.editor.SetText(strings.Repeat("line\n", 40))
	r.frame()
	r.frame()
	r.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(image.Pt(295, 8))},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(295, 8))},
	)
	r.frame()
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(image.Pt(295, 60))})
	r.frame()
	r.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(image.Pt(295, 60))})
	r.frame()
	r.frame()
	if got := r.editor.ScrollOff().Y; got <= 0 {
		t.Errorf("after dragging the thumb 52px down: scroll %d, want the text scrolled", got)
	}
}

// Text that fits shows no bar and never scrolls.
func TestTextareaScrollbarHiddenWhileTheTextFits(t *testing.T) {
	var r scrollRig
	r.editor.SetText("one line")
	r.frame()
	if got := r.frame(); got >= 120 {
		t.Fatalf("one line: height %d, want it under max-h", got)
	}
	if got := r.editor.ScrollBounds().Max.Y; got > 0 {
		t.Errorf("one line: scroll room %d, want none", got)
	}
}

// A foot stands inside the frame under the text, and its height comes out of
// the ceiling rather than past it.
func TestTextareaFootSitsUnderTheText(t *testing.T) {
	const foot = 30
	var editor widget.Editor
	var rig motionRig
	frame := func() int {
		return rig.frame(func(gtx C) D {
			gtx.Constraints = layout.Constraints{Max: image.Pt(300, 200)}
			ta := testTheme().Textarea(&editor, "")
			ta.Foot = func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, foot)} }
			return ta.Layout(gtx)
		}).Size.Y
	}
	if got := frame(); got != 64+foot {
		t.Errorf("empty: height %d, want min-h-16 plus the foot (%d)", got, 64+foot)
	}
	editor.SetText(strings.Repeat("line\n", 40))
	if got := frame(); got != 200 {
		t.Errorf("long text: height %d, want the 200 ceiling with the foot in it", got)
	}
}

// A textarea takes its own type size, line height and room around the text.
func TestTextareaTakesItsOwnTypeAndRoom(t *testing.T) {
	var editor widget.Editor
	editor.SetText("a\nb\nc")
	height := func(set func(*TextareaStyle)) int {
		var r motionRig
		return r.frame(func(gtx C) D {
			gtx.Constraints = layout.Constraints{Max: image.Pt(300, 400)}
			ta := testTheme().Textarea(&editor, "")
			set(&ta)
			return ta.Layout(gtx)
		}).Size.Y
	}
	plain := height(func(*TextareaStyle) {})
	roomy := height(func(ta *TextareaStyle) { ta.Size, ta.Leading, ta.PadY = 15, 22.5, 14 })
	// Three lines of 22.5 with 14 above and under, against 20 and 8: the
	// first 15 or so taller, within a pixel or two of rounding.
	if plain != 3*20+16 || roomy < 3*22+2*14 || roomy > 3*23+2*14+2 {
		t.Errorf("heights %d and %d, want %d and about %d", plain, roomy, 3*20+16, 3*22+2*14+2)
	}
}

// edgesDiffer is whether a and b differ anywhere outside the box's inside, 8px
// in from its edge: the frame, its border and the ring band round it, not the
// text and its caret.
func edgesDiffer(a, b *image.RGBA, box image.Rectangle) bool {
	inner := box.Inset(8)
	for y := box.Min.Y - 6; y < box.Max.Y+6; y++ {
		for x := box.Min.X - 6; x < box.Max.X+6; x++ {
			if image.Pt(x, y).In(inner) {
				continue
			}
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return true
			}
		}
	}
	return false
}

// A steady textarea draws its frame the same with the keyboard and without:
// the border of rest and no ring, the caret alone saying where the keyboard
// is. A plain one wears the focus ring.
func TestASteadyTextareaFramesTheSameFocusedOrNot(t *testing.T) {
	for _, steady := range []bool{true, false} {
		th, ed := testTheme(), new(widget.Editor)
		ta := th.Textarea(ed, "Ask")
		ta.Steady = steady
		rig := newInputRig(t, 200, 80, ta.Layout)
		box := image.Rectangle{Min: image.Pt(rig.margin, rig.margin), Max: image.Pt(rig.margin+200, rig.margin+80)}
		rest := rig.frame()
		rig.router.Source().Execute(key.FocusCmd{Tag: ed})
		rig.frame()
		focused := rig.frame()
		if differ := edgesDiffer(rest, focused, box); differ == steady {
			t.Errorf("steady %v: the frame differs with the focus: %v", steady, differ)
		}
	}
}

func TestClassifyEditSeparatesTypingDeletingAndTheRest(t *testing.T) {
	for name, c := range map[string]struct {
		old  editState
		text string
		want editKind
	}{
		"a character typed at the caret": {editState{"ab", 2, 2}, "abc", editType},
		"typed in the middle":            {editState{"ac", 1, 1}, "abc", editType},
		"a character typed elsewhere":    {editState{"ab", 0, 0}, "abc", editOther},
		"a newline":                      {editState{"ab", 2, 2}, "ab\n", editOther},
		"a backspace":                    {editState{"abc", 3, 3}, "ab", editDelete},
		"a delete forward":               {editState{"abc", 0, 0}, "bc", editDelete},
		"a paste":                        {editState{"ab", 2, 2}, "abxyz", editOther},
		"a replaced selection":           {editState{"abc", 3, 0}, "x", editOther},
		"a cut":                          {editState{"abcd", 4, 1}, "a", editOther},
	} {
		if got := classifyEdit(c.old, c.text); got != c.want {
			t.Errorf("%s: kind %d, want %d", name, got, c.want)
		}
	}
}
