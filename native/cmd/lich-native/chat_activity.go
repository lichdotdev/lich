package main

import (
	"image"
	"slices"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/native/ui"
)

// What runs for a chat while its turn goes on, the agent's task list, the
// sessions it opened and its background shells, stands in one strip over the
// composer's box, only while something is there: the task running on the
// left, a pill per kind on the right. Each opens its panel over the strip.

// chatActivityPanels are the panels the strip opens: they stand over it, so
// the line that opened one stays in view.
var chatActivityPanels = []string{chatPlanPanel, chatChildrenPanel, chatShellsPanel}

// outsideReach is how far past the panel a press is read: the whole window.
const outsideReach = 1 << 15

// chatActivity is the strip: the task line, then the subagents' and the
// shells' pills; nothing while none of them has anything.
func (u *window) chatActivity(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	m := op.Record(gtx.Ops)
	g := gtx
	g.Constraints.Min = image.Point{}
	pills := layout.Flex{Alignment: layout.Middle}.Layout(g,
		layout.Rigid(func(gtx C) D { return u.childrenChip(gtx, th, p, sv) }),
		layout.Rigid(func(gtx C) D { return u.shellsChip(gtx, th, p, sv) }),
	)
	pillsCall := m.Stop()
	m = op.Record(gtx.Ops)
	room := gtx.Constraints.Max.X
	if pills.Size.X > 0 {
		room -= pills.Size.X + gtx.Dp(ui.Space(1))
	}
	g.Constraints.Max.X = max(room, 0)
	plan := u.chatPlan(g, th, p, sv.ID, working)
	planCall := m.Stop()
	if pills.Size.Y == 0 && plan.Size.Y == 0 {
		return D{}
	}
	h := max(pills.Size.Y, plan.Size.Y)
	planCall.Add(gtx.Ops)
	t := op.Offset(image.Pt(gtx.Constraints.Max.X-pills.Size.X, (h-pills.Size.Y)/2)).Push(gtx.Ops)
	pillsCall.Add(gtx.Ops)
	t.Pop()
	gap := gtx.Dp(ui.Space(1))
	return D{Size: image.Pt(gtx.Constraints.Max.X, h+gap)}
}

// panelLift is how far over the box p's panel stands: over the strip for
// one of the strip's own.
func (p *chatPane) panelLift() int {
	if slices.Contains(chatActivityPanels, p.tools.kind) {
		return p.stripH
	}
	return 0
}

// watchOutside reads presses around the open panel, size big from where it
// was drawn, to the window's edges: four bands that never lie under the
// panel, passing each press on to what is under it.
func (t *chatTools) watchOutside(gtx C, size image.Point) {
	const r = outsideReach
	bands := [len(t.outside)]image.Rectangle{
		image.Rect(-r, -r, r, 0),
		image.Rect(-r, size.Y, r, r),
		image.Rect(-r, 0, 0, size.Y),
		image.Rect(size.X, 0, r, size.Y),
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	for i, b := range bands {
		area := clip.Rect(b).Push(gtx.Ops)
		event.Op(gtx.Ops, &t.outside[i])
		area.Pop()
	}
}

// outsideActs shuts the panel on a press outside it, as one outside a
// popover does, but not on the strip's line or pill that opened it: their
// click shuts it.
func (u *window) outsideActs(gtx C, p *chatPane) {
	t := &p.tools
	for i := range t.outside {
		for {
			ev, ok := gtx.Event(pointer.Filter{Target: &t.outside[i], Kinds: pointer.Press})
			if !ok {
				break
			}
			if _, isPointer := ev.(pointer.Event); !isPointer || !t.opened() {
				continue
			}
			if p.plan.toggle.Hovered() || p.shellsChip.click.Hovered() || p.childrenChip.click.Hovered() {
				continue
			}
			t.shut()
		}
	}
}
