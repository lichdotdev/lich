package main

import (
	"image/color"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// railServicesWidth is the services panel's width off the rail: the open
// footer's, so a row reads the same in both.
const railServicesWidth unit.Dp = 256

// railFooter is the rail's last two buttons, what the open panel's footer
// shows: the project's services, as a panel of the same rows, and the
// profile, as the same menu.
type railFooter struct {
	services      widget.Clickable
	servicesPanel ui.DropdownPanelState
	profile       widget.Clickable
	profileMenu   ui.MenuState
}

func (r *sidebarRail) holdsKeys() bool {
	return r.foot.servicesPanel.Opened() || r.foot.profileMenu.Opened()
}

// footer lays the rail's footer out: a hairline, Services when a project is
// on screen, and the profile when its provider has two logins to pick from.
func (r *sidebarRail) footer(gtx C, th *ui.Theme, s *sidebar, m *model, act *sidebarAction) D {
	p, ok := m.project()
	if !ok {
		return D{}
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return railHairline(gtx, th) })}
	list, console := m.services.of(p.ID), m.consoleShown()
	children = append(children, layout.Rigid(func(gtx C) D { return r.servicesButton(gtx, th, s, p.ID, list, console, act) }))
	if f, ok := m.focusedProfile(); ok {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D { return r.profileButton(gtx, th, s, m, f, act) }))
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (r *sidebarRail) servicesButton(gtx C, th *ui.Theme, s *sidebar, projectID string, list []backend.Service, console string, act *sidebarAction) D {
	t := tallyServices(list)
	tip := th.Tooltip(s.root, s.hint("rail:services"), railServicesHint(t))
	tip.Side = ui.SideRight
	panel := th.DropdownPanel(s.root, &r.foot.servicesPanel, railServicesWidth, func(gtx C, th *ui.Theme) D {
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
				return squareFace(gtx, bg, func(gtx C) D {
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

// railServicesBadge is the dot on the icon's corner: red when one failed,
// the running tone while one runs, none otherwise. It carries a ring of the
// button's fill so it reads off the icon's stroke under it.
func railServicesBadge(gtx C, th *ui.Theme, t servicesTally) D {
	size := gtx.Constraints.Min
	c := th.TonePass
	switch {
	case t.failed:
		c = th.Destructive
	case t.up == 0:
		return D{Size: size}
	}
	r := float32(gtx.Dp(3))
	center := f32.Pt(float32(size.X)-r, r)
	if !t.failed {
		pulseRing(gtx, center, r, float32(gtx.Dp(3)), c)
	}
	ui.StrokeArc(gtx, center, r+float32(gtx.Dp(1)), float32(gtx.Dp(2)), 0, 2*math.Pi, th.Surface)
	ui.StrokeArc(gtx, center, r/2, r, 0, 2*math.Pi, c)
	return D{Size: size}
}

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

func (r *sidebarRail) profileButton(gtx C, th *ui.Theme, s *sidebar, m *model, f profileFocus, act *sidebarAction) D {
	look := f.book.look(f.choice.shown)
	tip := th.Tooltip(s.root, s.hint("rail:profile"), f.book.nameOf(f.choice.shown))
	tip.Side = ui.SideRight
	menu := th.DropdownMenu(s.root, &r.foot.profileMenu, m.footerEntries(th, f, act))
	return tip.Layout(gtx, func(gtx C) D {
		return menu.Layout(gtx, func(gtx C) D {
			return r.foot.profile.Layout(gtx, func(gtx C) D {
				bg := railFill(th, r.foot.profileMenu.Opened(), r.foot.profile.Hovered())
				return squareFace(gtx, bg, func(gtx C) D { return profileAvatar(gtx, th.On(bg), look, ui.Space(4)) })
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
		layout.Rigid(th.Text(ui.TextXS, i18n.T("sidebar.services.title")).Muted().Layout),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D { return servicesCount(gtx, th, t) }),
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
