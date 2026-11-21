package main

import (
	"image"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The scope rows' measures: a provider's mark at the sidebar's 1dp a cell,
// at most scopeMaxMarks of them, so the path keeps the room.
var (
	scopeMarkBox  = ui.Space(4)
	scopeMarkGap  = ui.Space(0.5)
	scopeMaxMarks = 4
)

// scopeSelector is the button at the top of the nav, Global or the project
// being edited, and the menu it opens: Global, then each open project with
// its path, the agents open in it and how many settings it changed.
func (s *settingsScreen) scopeSelector(gtx C, th *ui.Theme, values scopedValues) D {
	n := &s.nav
	for _, id := range append([]string{""}, projectIDs(s.projects)...) {
		if clickableOf(n.scopeRows, id).Clicked(gtx) {
			s.scope = id
			n.scopeMenu.Close()
		}
	}
	pop := th.DropdownPanel(s.root, &n.scopeMenu, navScopeMenuW, func(gtx C, th *ui.Theme) D {
		return s.scopeMenu(gtx, th, values)
	})
	return pop.Layout(gtx, func(gtx C) D { return s.scopeButton(gtx, th) })
}

func projectIDs(projects []settingsProject) []string {
	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.id
	}
	return ids
}

// scopeButton is the trigger, as tall as two lines: the scope's name over its
// path, the agents open in it and the chevrons, on the nav's hover fill.
// "Scope" is its tooltip; the menu it opens says the rest.
func (s *settingsScreen) scopeButton(gtx C, th *ui.Theme) D {
	n := &s.nav
	tip := th.Tooltip(s.root, &n.scopeTip, i18n.T("settings.settingsNav.scopeLabel"))
	tip.Side = ui.SideBottom
	return tip.Layout(gtx, func(gtx C) D {
		return n.scopeHover.Layout(gtx, func(gtx C) D {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(navScopeHeight))
			gtx.Constraints = layout.Exact(size)
			bg := th.Surface
			if n.scopeHover.Hovered() || n.scopeMenu.Opened() {
				bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
				ui.Fill(gtx, bg, ui.RadiusMD)
			}
			on := th.On(bg)
			p, _ := s.scopeProject()
			return layout.Inset{Left: navRowPadX, Right: navRowPadX}.Layout(gtx, func(gtx C) D {
				return layout.W.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, scopeName(on, scopeTitle(p), scopePath(p), font.Medium)),
						layout.Rigid(scopeLive(on, p)),
						ui.Gap(1.5),
						layout.Rigid(func(gtx C) D { return icScopeChevron.Layout(gtx, ui.Space(4), on.MutedForeground) }),
					)
				})
			})
		})
	})
}

func (s *settingsScreen) scopeProject() (settingsProject, bool) {
	for _, p := range s.projects {
		if p.id == s.scope {
			return p, true
		}
	}
	return settingsProject{}, false
}

// scopeTitle and scopePath are a scope's two lines: Global over "every
// project", a project's name over its path.
func scopeTitle(p settingsProject) string {
	if p.id == "" {
		return i18n.T("settings.settingsNav.scopeGlobal")
	}
	return p.name
}

func scopePath(p settingsProject) string {
	if p.id == "" {
		return i18n.T("settings.settingsNav.everyProject")
	}
	return shortPath(p.path)
}

// scopeName is the name over the muted mono line under it, both cut with an
// ellipsis when the row is short.
func scopeName(th *ui.Theme, name, line string, weight font.Weight) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextSM, name).Weight(weight).Layout),
			layout.Rigid(th.Text(ui.TextXS, line).Muted().Mono().Layout),
		)
	}
}

// scopeLive is what runs in a project: the souls while one of its agents
// works, then each provider open there by its pixel mark.
func scopeLive(th *ui.Theme, p settingsProject) layout.Widget {
	return func(gtx C) D {
		var children []layout.FlexChild
		if p.working {
			children = append(children, layout.Rigid(func(gtx C) D {
				side := gtx.Dp(soulsSize)
				gtx.Constraints = layout.Exact(image.Pt(side, side))
				return soulsSpinner(gtx, th)
			}))
		}
		for i, kind := range p.agents {
			if i == scopeMaxMarks {
				break
			}
			children = append(children, layout.Rigid(layout.Spacer{Width: scopeMarkGap}.Layout), layout.Rigid(func(gtx C) D {
				return providerGlyph(gtx, th, kind, scopeMarkBox, markPixel)
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

func (s *settingsScreen) scopeMenu(gtx C, th *ui.Theme, values scopedValues) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.scopeRow(gtx, th, settingsProject{}, 0) }),
		layout.Rigid(func(gtx C) D { return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, th.Separator) }),
	}
	for _, p := range s.projects {
		children = append(children, layout.Rigid(func(gtx C) D {
			return s.scopeRow(gtx, th, p, projectChanges(p.id, values))
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// scopeMenuIcon is the menu's 15px icons.
var scopeMenuIcon = ui.Space(3.75)

// changesCount is how many settings a project changed, in mono after its
// agents, a dot between them; "" for none.
func changesCount(n int, agents bool) string {
	switch {
	case n == 0:
		return ""
	case agents:
		return "· " + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// scopeRow is one menu row: the name over the path, the agents open in the
// project, how many settings it changed, muted, and the check on the scope
// being edited.
func (s *settingsScreen) scopeRow(gtx C, th *ui.Theme, p settingsProject, changed int) D {
	click := clickableOf(s.nav.scopeRows, p.id)
	changes := changesCount(changed, p.working || len(p.agents) > 0)
	return th.PopoverRow(click, func(gtx C, th *ui.Theme) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, scopeName(th, scopeTitle(p), scopePath(p), font.Normal)),
			layout.Rigid(scopeLive(th, p)),
			layout.Rigid(func(gtx C) D {
				if changes == "" {
					return D{}
				}
				return layout.Inset{Left: ui.Space(1.5)}.Layout(gtx, th.Text(ui.TextXS, changes).Muted().Mono().Layout)
			}),
			layout.Rigid(func(gtx C) D {
				if p.id != s.scope {
					return D{}
				}
				return layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D { return icScopeCheck.Layout(gtx, scopeMenuIcon, th.Foreground) })
			}),
		)
	}).Layout(gtx)
}
