package main

import (
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	icChatCopy = icMenuCopy
	// chatCopySlot is the height of the line a message's time stands on, which
	// the copy button fits in without pushing; the button itself is a little
	// larger than the line and reaches past it.
	chatCopySlot = ui.Space(4)
	chatCopyHit  = ui.Space(5)
)

// chatCopiedFor is how long the copy button shows its check.
const chatCopiedFor = 1500 * time.Millisecond

// chatCopy is one message's copy button and the pointer over it, kept across
// frames.
type chatCopy struct {
	click    widget.Clickable
	tip      ui.Tooltip
	hovered  bool
	copiedAt time.Time
}

func (v *chatView) copyOf(id string) *chatCopy {
	c, ok := v.copies[id]
	if !ok {
		if v.copies == nil {
			v.copies = map[string]*chatCopy{}
		}
		c = new(chatCopy)
		v.copies[id] = c
	}
	return c
}

// hover reads the pointer over the area w is drawn in and lays w out knowing
// whether it is there. The area passes every press on to what is under it.
func (c *chatCopy) hover(gtx C, w func(gtx C, over bool) D) D {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &c.hovered, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			c.hovered = e.Kind == pointer.Enter
		}
	}
	macro := op.Record(gtx.Ops)
	d := w(gtx, c.hovered)
	call := macro.Stop()
	call.Add(gtx.Ops)
	// Over the message, not under it: a press on the button under the pointer
	// would otherwise end the hover, and the button with it.
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &c.hovered)
	pass.Pop()
	area.Pop()
	return d
}

// button is the icon-only copy button: Copy until pressed, then a check for
// chatCopiedFor, its tooltip saying so, putting text on the clipboard as the
// window does everywhere. It takes the room of the line it stands on.
func (c *chatCopy) button(gtx C, th *ui.Theme, root *ui.Root, text string) D {
	if c.click.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		c.copiedAt = gtx.Now
	}
	done := !c.copiedAt.IsZero() && gtx.Now.Sub(c.copiedAt) < chatCopiedFor
	label, glyph := i18n.T("chat.copy.copy"), icChatCopy
	if done {
		gtx.Execute(op.InvalidateCmd{At: c.copiedAt.Add(chatCopiedFor)})
		label, glyph = i18n.T("chat.copy.copied"), icCheck
	}
	slot, hit := gtx.Dp(chatCopySlot), gtx.Dp(chatCopyHit)
	d := th.Tooltip(root, &c.tip, label).Layout(gtx, func(gtx C) D {
		off := op.Offset(image.Pt(0, (slot-hit)/2)).Push(gtx.Ops)
		defer off.Pop()
		gtx.Constraints = layout.Exact(image.Pt(hit, hit))
		return c.click.Layout(gtx, func(gtx C) D {
			fg := th.MutedForeground
			if c.click.Hovered() {
				ui.Fill(gtx, th.Accent, ui.RadiusSM)
				fg = th.AccentForeground
			}
			return layout.Center.Layout(gtx, func(gtx C) D { return glyph.Layout(gtx, ui.Space(3), fg) })
		})
	})
	return D{Size: image.Pt(d.Size.X, slot)}
}
