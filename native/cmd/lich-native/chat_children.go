package main

import (
	"cmp"
	"image"
	"image/color"
	"slices"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// chatChildrenPanel is the panel's kind in chatTools.
	chatChildrenPanel = "children"
	chipChildren      = "children"
	// chatChildMarks is how many of the children's marks the chip stacks.
	chatChildMarks = 3
)

var (
	chatChildMark    = ui.Space(4.5)
	chatChildMarkGap = ui.Space(1)
	chatChildDot     = ui.Space(2.5)
)

// childrenOf are the open sessions session id opened, in every project:
// those waiting on the person first, then those working, then the rest.
func (m *model) childrenOf(id string, now time.Time) []sessionView {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []sessionView
	for _, p := range m.projects {
		for _, s := range p.Sessions {
			if s.OriginSessionID == id {
				out = append(out, m.viewOf(p, s, now))
			}
		}
	}
	slices.SortStableFunc(out, func(a, b sessionView) int { return cmp.Compare(childRank(a), childRank(b)) })
	return out
}

// childRank orders a child: waiting on the person, working, then the rest,
// which the list shows below a rule.
func childRank(sv sessionView) int {
	switch sv.status {
	case statusWaiting:
		return 0
	case statusBusy, statusCompacting:
		return 1
	}
	return 2
}

// childDot is the color of a child's state.
func childDot(th *ui.Theme, sv sessionView) color.NRGBA {
	switch sv.status {
	case statusWaiting:
		return th.ToneWait
	case statusBusy, statusCompacting:
		return th.Primary
	case statusDone:
		return th.TonePass
	}
	return th.MutedForeground
}

// childrenChip counts the sessions this chat's session opened, their marks
// stacked before it, and how many wait on the person; none without children.
func (u *window) childrenChip(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	kids := u.model.childrenOf(sv.ID, gtx.Now)
	if len(kids) == 0 {
		return D{}
	}
	p.childrenChip.pressed = p.tools.kind == chatChildrenPanel
	waiting := 0
	for _, k := range kids {
		if k.status == statusWaiting {
			waiting++
		}
	}
	title := i18n.T("chat.children.title")
	return footerButton(gtx, th, &u.root, &p.childrenChip, title, true, false, func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		row := []layout.FlexChild{}
		for i, k := range kids[:min(len(kids), chatChildMarks)] {
			if i > 0 {
				row = append(row, ui.Gap(0.5))
			}
			row = append(row, layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, childKind(k), footerGlyph, markPixel) }))
		}
		row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, i18n.T("chat.children.count", i18n.Args{"count": len(kids)})).In(fg).Layout))
		if waiting > 0 {
			row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, "· "+i18n.T("chat.children.waiting", i18n.Args{"count": waiting})).In(th.ToneWait).Layout))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// childKind is the provider a child runs.
func childKind(sv sessionView) string {
	if sv.agent != "" {
		return sv.agent
	}
	return sv.Kind
}

// childrenChipActs opens or shuts the panel on a click of the chip.
func (u *window) childrenChipActs(gtx C, p *chatPane) {
	if !p.childrenChip.click.Clicked(gtx) {
		return
	}
	if p.tools.kind == chatChildrenPanel {
		p.tools.shut()
		return
	}
	t := &p.tools
	t.open(chatChildrenPanel)
	t.mu.Lock()
	t.loaded, t.base = true, p.editor.Len()
	t.mu.Unlock()
}

// childrenActs runs the panel: a row opens its session.
func (u *window) childrenActs(gtx C, p *chatPane, id string) (int, func(int)) {
	kids := u.model.childrenOf(id, gtx.Now)
	return len(kids), func(i int) {
		p.tools.shut()
		u.openSession(kids[i].projectID, kids[i].ID)
	}
}

func (u *window) childrenView(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	t := &p.tools
	kids := u.model.childrenOf(sv.ID, gtx.Now)
	t.ensureRows(len(kids))
	lines := make([]layout.Widget, len(kids))
	for i, k := range kids {
		row := func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return u.childRow(gtx, th, k) })
		}
		lines[i] = row
		if childRank(k) == 2 && i > 0 && childRank(kids[i-1]) < 2 {
			lines[i] = func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1), Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D { return chatHairline(gtx, th) })
					}),
					layout.Rigid(row))
			}
		}
	}
	head := func(gtx C) D { return toolsHead(gtx, th, i18n.T("chat.children.title"), "", nil) }
	return t.scrolled(gtx, th, head, lines, nil)
}

// childRow is a child: its provider's mark with its state's dot on the
// corner, its name, the provider, model, branch and how long it has been in
// its state, and what it waits on in ToneWait. One no longer working reads
// muted.
func (u *window) childRow(gtx C, th *ui.Theme, k sessionView) D {
	ink := th.Foreground
	if childRank(k) == 2 {
		ink = th.MutedForeground
	}
	lines := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextSM, k.Label).In(ink).Truncated().Layout),
		layout.Rigid(th.Text(ui.TextXS, u.childMeta(k)).Muted().Truncated().Layout),
	}
	if k.status == statusWaiting && k.reason != "" {
		lines = append(lines, layout.Rigid(th.Text(ui.TextXS, cleanLine(k.reason)).In(th.ToneWait).Truncated().Layout))
	}
	return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return childMark(gtx, th, k) }),
			ui.Gap(2.5),
			layout.Flexed(1, func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...) }),
		)
	})
}

// childMeta is "Codex · GPT-5.6 · fix-412 · 4m": the provider, the model,
// the branch and how long the child has been in its state.
func (u *window) childMeta(k sessionView) string {
	kind := childKind(k)
	parts := []string{kind}
	if info, ok := u.model.providers.find(kind); ok && info.name != "" {
		parts[0] = info.name
	}
	if model := cmp.Or(k.running, k.Model); model != "" {
		parts = append(parts, model)
	}
	if k.git.Branch != "" {
		parts = append(parts, k.git.Branch)
	}
	switch {
	case k.aged:
		parts = append(parts, chatElapsed(k.age))
	case k.idled:
		parts = append(parts, i18n.T("chat.children.ended", i18n.Args{"time": chatElapsed(k.idle)}))
	}
	return strings.Join(parts, " · ")
}

// childMark is k's provider mark with its state's dot on the lower right
// corner, ringed in the row's surface.
func childMark(gtx C, th *ui.Theme, k sessionView) D {
	box := gtx.Dp(chatChildMark)
	d := providerGlyph(gtx, th, childKind(k), chatChildMark, markPixel)
	dot, ring := gtx.Dp(chatChildDot), gtx.Dp(unit.Dp(1.5))
	at := image.Pt(box-dot+ring, box-dot+ring)
	paint.FillShape(gtx.Ops, th.Surface, clip.Ellipse{Min: at.Sub(image.Pt(ring, ring)), Max: at.Add(image.Pt(dot+ring, dot+ring))}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, childDot(th, k), clip.Ellipse{Min: at, Max: at.Add(image.Pt(dot, dot))}.Op(gtx.Ops))
	return D{Size: image.Pt(d.Size.X+gtx.Dp(chatChildMarkGap), d.Size.Y)}
}
