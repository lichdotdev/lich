package main

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/omartelo/lich/native/ui"
)

// scopeHoverFaint is the mockup's --hover-faint, the scope button's rest fill.
const scopeHoverFaint = 0.04

// scopeSelector is the button at the top of the nav, Global or the project
// being edited, and the menu it opens: Global, then each open project with
// its color and how many settings it changed.
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

// scopeButton is the h-11 rounded-lg trigger: the globe or the project's
// swatch, the "Scope"/"Project" caption over the value, and the chevrons.
func (s *settingsScreen) scopeButton(gtx C, th *ui.Theme) D {
	n := &s.nav
	return n.scopeHover.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(navScopeHeight))
		gtx.Constraints = layout.Exact(size)
		bg := th.Over(ui.Alpha(th.Foreground, scopeHoverFaint))
		if n.scopeHover.Hovered() || n.scopeMenu.Opened() {
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		ui.Fill(gtx, bg, ui.RadiusLG)
		insetBorder(gtx, size, gtx.Dp(ui.RadiusLG), th.Over(th.Border))
		on := th.On(bg)
		caption, value := scopeLabelGlobal, scopeGlobal
		lead := func(gtx C) D { return icScopeGlobal.Layout(gtx, ui.Space(4), on.MutedForeground) }
		if p, ok := s.scopeProject(); ok {
			caption, value = scopeLabelProject, p.name
			lead = func(gtx C) D { return swatch(gtx, projectSwatch(p.id)) }
		}
		return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(lead),
				ui.Gap(2.5),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(on.Text(navSmall, caption).Muted().Leading(13).Layout),
						layout.Rigid(on.Text(ui.TextSM, value).Weight(font.Medium).Leading(17).Layout),
					)
				}),
				layout.Rigid(func(gtx C) D { return icScopeChevron.Layout(gtx, ui.Space(4), on.MutedForeground) }),
			)
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

func (s *settingsScreen) scopeMenu(gtx C, th *ui.Theme, values scopedValues) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return s.scopeRow(gtx, th, "", func(gtx C, th *ui.Theme) D {
				return icScopeGlobal.Layout(gtx, scopeMenuIcon, th.MutedForeground)
			}, scopeGlobal, "")
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1)}.Layout(gtx,
				th.Text(ui.TextXS, scopeOpenProjects).Muted().Layout)
		}),
	}
	for _, p := range s.projects {
		children = append(children, layout.Rigid(func(gtx C) D {
			return s.scopeRow(gtx, th, p.id, func(gtx C, _ *ui.Theme) D { return swatch(gtx, projectSwatch(p.id)) },
				p.name, changesLabel(projectChanges(p.id, values)))
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// scopeMenuIcon is the menu's 15px icons.
var scopeMenuIcon = ui.Space(3.75)

func changesLabel(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return scopeChangeOne
	}
	return fmt.Sprintf(scopeChangeMany, n)
}

// scopeRow is one menu row: lead, the name, the change count in amber and
// the check on the scope being edited.
func (s *settingsScreen) scopeRow(gtx C, th *ui.Theme, id string, lead func(gtx C, th *ui.Theme) D, name, changes string) D {
	click := clickableOf(s.nav.scopeRows, id)
	return popoverRow(gtx, th, click, func(gtx C, th *ui.Theme) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(scopeMenuIcon), gtx.Dp(scopeMenuIcon)))
				return layout.Center.Layout(gtx, func(gtx C) D { return lead(gtx, th) })
			}),
			ui.Gap(2),
			layout.Flexed(1, th.Text(ui.TextSM, name).Layout),
			layout.Rigid(func(gtx C) D {
				if changes == "" {
					return D{}
				}
				return layout.Inset{Left: ui.Space(2)}.Layout(gtx, th.Text(navSmall, changes).In(th.ToneWait).Layout)
			}),
			layout.Rigid(func(gtx C) D {
				if id != s.scope {
					return D{}
				}
				return layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D { return icScopeCheck.Layout(gtx, scopeMenuIcon, th.Foreground) })
			}),
		)
	})
}
