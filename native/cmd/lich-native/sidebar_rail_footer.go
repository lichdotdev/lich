package main

import (
	"image/color"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
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
	search        widget.Clickable
	searchPanel   ui.DropdownPanelState
}

func (r *sidebarRail) holdsKeys() bool {
	return r.foot.servicesPanel.Opened() || r.foot.profileMenu.Opened() || r.foot.searchPanel.Opened()
}

// footer lays the rail's footer out: a hairline, a new session, then, with a
// project on screen, Services and the profile when its provider has two
// logins to pick from. New session sits here, not at the top, so the rail's
// tree starts where the open sidebar's does.
func (r *sidebarRail) footer(gtx C, th *ui.Theme, s *sidebar, m *model, act *sidebarAction) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return th.RailHairline(gtx) }),
		layout.Rigid(func(gtx C) D { return r.searchButton(gtx, th, s) }),
		ui.VGap(1),
		// New session is not named for the morph: the open sidebar's sits at
		// its top, and flying it corner to corner reads as a jump.
		layout.Rigid(r.button(th, s, &r.add, icPlus, i18n.T("sidebar.sidebarRail.newSession"), "")),
	}
	if p, ok := m.project(); ok {
		list, console := m.services.of(p.ID), m.consoleShown()
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D { return r.servicesButton(gtx, th, s, p.ID, list, console, act) }))
		if f, ok := m.focusedProfile(); ok {
			children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D { return r.profileButton(gtx, th, s, m, f, act) }))
		}
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

// railServicesBadge is the dot on the icon's corner: red when one failed,
// the running tone while one runs, none otherwise. It carries a ring of the
// button's fill so it reads off the icon's stroke under it.
func railServicesBadge(gtx C, th *ui.Theme, t servicesTally) D {
	switch {
	case t.failed:
		return th.CornerDot(gtx, th.Destructive)
	case t.up == 0:
		return D{Size: gtx.Constraints.Min}
	}
	return th.PulsingCornerDot(gtx, th.TonePass)
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
				return ui.SquareFace(gtx, bg, func(gtx C) D { return profileAvatar(gtx, th.On(bg), look, ui.Space(4)) })
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

// railSearchWidth is the filter panel's width off the rail: the open
// sidebar's field and chips at the width they wrap at there.
const railSearchWidth unit.Dp = 256

// searchButton is the rail's filter: the open sidebar's field and phase chips
// in a panel off the rail, filtering the rail as they filter the open
// sidebar. The filter outlives the panel, which a lit button with a dot
// says; it ends where the open one does, Escape on an empty field.
func (r *sidebarRail) searchButton(gtx C, th *ui.Theme, s *sidebar) D {
	h := &s.header
	tip := th.Tooltip(s.root, s.hint("rail:filter"), i18n.T("sidebar.sessionSidebar.filter"))
	tip.Side = ui.SideRight
	panel := th.DropdownPanel(s.root, &r.foot.searchPanel, railSearchWidth, func(gtx C, th *ui.Theme) D {
		return s.filterRow(gtx, th, r.view)
	})
	was := r.foot.searchPanel.Opened()
	d := tip.Layout(gtx, func(gtx C) D {
		return panel.Layout(gtx, func(gtx C) D {
			return r.foot.search.Layout(gtx, func(gtx C) D {
				narrows := h.filter().narrows()
				bg := railFill(th, r.foot.searchPanel.Opened() || narrows, r.foot.search.Hovered())
				return ui.SquareFace(gtx, bg, func(gtx C) D {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx C) D { return icSearch.Layout(gtx, ui.Space(4), th.MutedForeground) }),
						layout.Expanded(func(gtx C) D {
							if !narrows || r.foot.searchPanel.Opened() {
								return D{Size: gtx.Constraints.Min}
							}
							on := th.On(bg)
							return on.CornerDot(gtx, on.Foreground)
						}),
					)
				})
			})
		})
	})
	r.searchOpened(gtx, h, was)
	return d
}

// searchOpened follows the panel opening and closing: opening it opens the
// filter with the field focused, closing it on an empty filter closes the
// filter, and an Escape that closed the filter closes the panel.
func (r *sidebarRail) searchOpened(gtx C, h *sidebarHeader, was bool) {
	open := r.foot.searchPanel.Opened()
	switch {
	case open && !was && !h.open:
		h.toggle()
		gtx.Execute(op.InvalidateCmd{})
	case open && !was:
		h.focusQuery = true
		gtx.Execute(op.InvalidateCmd{})
	case open && !h.open:
		r.foot.searchPanel.Close()
	case !open && was && !h.filter().narrows():
		h.open = false
	}
	if !open {
		h.searchFocused = false
	}
}
