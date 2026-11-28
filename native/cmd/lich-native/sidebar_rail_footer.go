package main

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// railPanelWidth is the width of a panel off the rail: the open footer's, so
// a row reads the same in both.
const railPanelWidth unit.Dp = 256

// railFooter is the rail's last two buttons, what the open panel's footer
// shows: the project's services, as a panel of the same rows, and the
// account, as the same panel.
type railFooter struct {
	services      ui.Clickable
	servicesPanel ui.DropdownPanelState
	profile       ui.Clickable
}

func (r *sidebarRail) holdsKeys() bool {
	return r.foot.servicesPanel.Opened()
}

// footer lays the rail's footer out with a project on screen: a hairline,
// Services, and the account when the open footer shows its row.
func (r *sidebarRail) footer(gtx C, th *ui.Theme, s *sidebar, m *model, act *sidebarAction) D {
	p, ok := m.project()
	if !ok {
		return D{}
	}
	list, console := m.services.of(p.ID), m.consoleShown()
	children := []layout.FlexChild{
		ui.VGap(2),
		layout.Rigid(func(gtx C) D { return r.servicesButton(gtx, th, s, p.ID, list, console, act) }),
	}
	if a, ok := s.profile.account(m); ok {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D { return r.profileButton(gtx, th, s, m, a, act) }))
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (r *sidebarRail) servicesButton(gtx C, th *ui.Theme, s *sidebar, projectID string, list []backend.Service, console string, act *sidebarAction) D {
	t := tallyServices(list)
	tip := th.Tooltip(s.root, s.hint("rail:services"), railServicesHint(t))
	tip.Side = ui.SideRight
	panel := th.DropdownPanel(s.root, &r.foot.servicesPanel, railPanelWidth, func(gtx C, th *ui.Theme) D {
		d := s.services.panelContent(gtx, th, s.root, projectID, list, console, act)
		if act.service.op != serviceNone {
			r.foot.servicesPanel.Close()
		}
		return d
	})
	return tip.Layout(gtx, func(gtx C) D {
		return panel.Layout(gtx, func(gtx C) D {
			return r.foot.services.Layout(gtx, func(gtx C) D {
				bg := railFill(th, r.foot.servicesPanel.Opened(), r.foot.services.Hovered())
				fg := th.MutedForeground
				if t.total == 0 {
					fg = th.Over(ui.Alpha(fg, 0.55))
				}
				return ui.SquareFace(gtx, bg, func(gtx C) D {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx C) D { return icServices.Layout(gtx, ui.Space(4), fg) }),
						layout.Expanded(func(gtx C) D { return railServicesBadge(gtx, th.On(bg), t) }),
					)
				})
			})
		})
	})
}

// railServicesHint is the Services button's tooltip: the count the closed
// footer row shows, in words.
func railServicesHint(t servicesTally) string {
	if t.total == 0 {
		return i18n.T("sidebar.services.title")
	}
	return i18n.T("sidebar.services.railHint", i18n.Args{"up": strconv.Itoa(t.up), "total": strconv.Itoa(t.total)})
}

// railServicesBadge is the open footer's count, up/total, on the icon's
// bottom right: red when one failed, the running tone while one runs, muted
// otherwise, none with no service. Its plate keeps the count readable against
// the button.
func railServicesBadge(gtx C, th *ui.Theme, t servicesTally) D {
	box := gtx.Constraints.Min
	if t.total == 0 {
		return D{Size: box}
	}
	c := th.MutedForeground
	switch {
	case t.failed > 0:
		c = th.Destructive
	case t.up > 0:
		c = th.TonePass
	}
	label := th.Text(ui.Text2XS, strconv.Itoa(t.up)+"/"+strconv.Itoa(t.total)).Mono().In(c).Weight(font.SemiBold)
	lgtx := gtx
	lgtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(ui.Space(8)), gtx.Dp(ui.Space(4)))}
	m := op.Record(gtx.Ops)
	d := label.Layout(lgtx)
	call := m.Stop()
	pad := gtx.Dp(railCountPad)
	plate := d.Size.Add(image.Pt(2*pad, 0))
	at := box.Sub(plate)
	defer op.Offset(at).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, th.Surface, clip.UniformRRect(image.Rectangle{Max: plate}, gtx.Dp(ui.RadiusSM)).Op(gtx.Ops))
	defer op.Offset(image.Pt(pad, 0)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: box}
}

// railCountPad is the plate's room on either side of the count.
const railCountPad = unit.Dp(2)

// railFill is a rail button's fill: accent while its popup is open, the
// rail's hover while hovered.
func railFill(th *ui.Theme, open, hovered bool) color.NRGBA {
	switch {
	case open:
		return th.Accent
	case hovered:
		return th.Over(ui.Alpha(th.Accent, railHoverFill))
	}
	return th.Surface
}

// profileButton is the account row's avatar, or its provider's glyph with no
// profiles, opening the panel the row opens.
func (r *sidebarRail) profileButton(gtx C, th *ui.Theme, s *sidebar, m *model, a sidebarAccount, act *sidebarAction) D {
	f := a.profile
	name := a.plan.Name
	if f.book != nil {
		name = f.book.nameOf(f.choice.shown)
	}
	tip := th.Tooltip(s.root, s.hint("rail:profile"), name)
	tip.Side = ui.SideRight
	panel := s.profile.dropdown(th, s.root, m, a, act, railPanelWidth)
	return tip.Layout(gtx, func(gtx C) D {
		return panel.Layout(gtx, func(gtx C) D {
			return r.foot.profile.Layout(gtx, func(gtx C) D {
				bg := railFill(th, s.profile.panel.Opened(), r.foot.profile.Hovered())
				on := th.On(bg)
				return ui.SquareFace(gtx, bg, func(gtx C) D {
					if f.book == nil {
						return providerIcon(a.kind).Layout(gtx, ui.Space(4), on.MutedForeground)
					}
					return profileAvatar(gtx, on, f.book.look(f.choice.shown), ui.Space(4))
				})
			})
		})
	})
}

// panelContent is the open footer's list as the rail's panel draws it: a
// header with the count and the all button, the rows, and New service.
func (f *servicesFooter) panelContent(gtx C, th *ui.Theme, root *ui.Root, projectID string, list []backend.Service, console string, act *sidebarAction) D {
	if f.add.Clicked(gtx) {
		act.service = serviceRequest{op: serviceAdd, project: projectID}
	}
	t := tallyServices(list)
	if f.all.Clicked(gtx) {
		act.service = serviceRequest{op: allServicesOp(t), project: projectID}
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return f.panelHeader(gtx, th, root, t) }),
		layout.Rigid(func(gtx C) D { return f.rowList(gtx, th, root, list, console, act) }),
		layout.Rigid(func(gtx C) D { return f.addRow(gtx, th) }),
	)
}

func (f *servicesFooter) panelHeader(gtx C, th *ui.Theme, root *ui.Root, t servicesTally) D {
	h := gtx.Dp(ui.Space(8))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	children := []layout.FlexChild{
		layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(i18n.T("sidebar.services.title"))).Leading(text2XSLeading).Muted().Weight(font.SemiBold).Layout),
		ui.Gap(1.5),
		layout.Rigid(servicesTallyLine(th, t)),
		layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
	}
	if t.total > 0 {
		children = append(children, layout.Rigid(func(gtx C) D { return f.allButton(gtx, th, root, t) }))
	}
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = h
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
