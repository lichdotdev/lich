package main

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.
const (
	scopeLabelGlobal  = "Scope"
	scopeGlobal       = "Global"
	scopeOpenProjects = "Open projects"
	scopeChangeOne    = "1 change"
	scopeChangeMany   = "%d changes"
	scopeNote         = "Only the settings a project can change are on. The rest are the same everywhere."
)

// The nav's geometry and its dimmed rows' opacity, from the mockup.
const navDimOpacity = 0.45

var (
	navRowHeight   = ui.Space(8)
	navRowPadX     = ui.Space(2.5)
	navScopeHeight = ui.Space(10)
	navScopeMenuW  = unit.Dp(212)
	navBadgeSize   = unit.Dp(18)
	navBadgeFill   = float32(0.2)
	navSmall       = unit.Sp(11)
	navHitPath     = unit.Sp(10.5)
	icScopeGlobal  = icons.Lucide("globe")
	icScopeChevron = icons.Lucide("chevrons-up-down")
	icScopeCheck   = icons.Lucide("check")
	icNavSearch    = icons.Lucide("search")
)

// settingsNav is the nav island's state: the scope selector, the search
// field and the rows under it.
type settingsNav struct {
	scopeMenu  ui.DropdownPanelState
	scopeHover widget.Clickable
	scopeRows  map[string]*widget.Clickable
	query      widget.Editor
	clear      widget.Clickable
	cursor     int
	searched   string // the query the cursor was placed for
	rows       map[string]*widget.Clickable
	hits       []widget.Clickable
	list       widget.List
	closeTip   ui.Tooltip
	scopeTip   ui.Tooltip
}

func (n *settingsNav) init() {
	n.scopeRows, n.rows = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}
	n.list.Axis = layout.Vertical
}

// navLayout is the nav island: py-3 px-2, the scope selector, the search
// field, the sections (or the results while searching), and the footer group.
func (s *settingsScreen) navLayout(gtx C, th *ui.Theme, values scopedValues) D {
	n := &s.nav
	hits := searchSettings(n.query.Text())
	if q := n.query.Text(); q != n.searched {
		n.searched, n.cursor = q, 0
	}
	s.searchKeys(gtx, hits)
	return layout.Inset{Top: ui.Space(2), Left: ui.Space(2), Right: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					return s.scopeSelector(gtx, th, values)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					field := th.Input(&n.query, settingsSearchHint)
					field.Leading, field.LeadingSize, field.Height, field.TextSize, field.Clear = icNavSearch, unit.Dp(15), ui.Space(8), 13, &n.clear
					return field.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx C) D {
				if strings.TrimSpace(n.query.Text()) != "" {
					return s.results(gtx, th, hits)
				}
				return s.sectionList(gtx, th, values, false)
			}),
			layout.Rigid(func(gtx C) D { return s.navFooter(gtx, th, values) }),
		)
	})
}

// searchKeys is onSearchKey: the arrows move through the results, Enter
// opens the one they are on, Escape empties the field.
func (s *settingsScreen) searchKeys(gtx C, hits []settingHit) {
	n := &s.nav
	names := []key.Name{key.NameEscape, key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter}
	filters := make([]event.Filter, len(names))
	for i, name := range names {
		filters[i] = key.Filter{Focus: &n.query, Name: name}
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		n.cursor = s.searchKey(gtx, e.Name, hits, n.cursor)
	}
}

// searchKey carries out one key and answers where the cursor is after it.
func (s *settingsScreen) searchKey(gtx C, name key.Name, hits []settingHit, cursor int) int {
	if name == key.NameEscape {
		s.nav.query.SetText("")
		return 0
	}
	if len(hits) == 0 {
		return 0
	}
	switch name {
	case key.NameDownArrow:
		return (cursor + 1) % len(hits)
	case key.NameUpArrow:
		return (cursor + len(hits) - 1) % len(hits)
	}
	s.choose(gtx, hits[min(cursor, len(hits)-1)])
	return cursor
}

// sectionList is the nav's sections, the scope note under them in a
// project's scope.
func (s *settingsScreen) sectionList(gtx C, th *ui.Theme, values scopedValues, footer bool) D {
	var children []layout.FlexChild
	for _, sec := range settingsSections {
		if sec.footer != footer {
			continue
		}
		children = append(children, layout.Rigid(func(gtx C) D { return s.sectionRow(gtx, th, sec, values) }), ui.VGap(0.5))
	}
	if !footer && s.scope != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(2.5), Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				l := th.Text(ui.TextXS, scopeNote).Muted().Leading(17.4)
				l.MaxLines = 0
				return l.Layout(gtx)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// navFooter is the footer group: a separator, then Updates and Help.
func (s *settingsScreen) navFooter(gtx C, th *ui.Theme, values scopedValues) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Separator),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return s.sectionList(gtx, th, values, true) }),
	)
}

// sectionRow is one nav row: the icon, the label and the override count,
// dimmed in a project's scope when the section has nothing a project sets.
func (s *settingsScreen) sectionRow(gtx C, th *ui.Theme, sec settingsSection, values scopedValues) D {
	click := clickableOf(s.nav.rows, sec.id)
	if click.Clicked(gtx) {
		s.openSection(sec.id)
	}
	badge := sectionBadge(sec, s.scope, s.projects, values)
	dim := s.scope != "" && !sec.projectScoped()
	return navRow(gtx, th, click, sec.id == s.section, func(gtx C, th *ui.Theme) D {
		fg, icon := th.Foreground, th.MutedForeground
		if dim {
			fg = th.Over(ui.Alpha(th.MutedForeground, navDimOpacity))
			icon = fg
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return sec.icon.Layout(gtx, ui.Space(4), icon) }),
			ui.Gap(2.5),
			layout.Flexed(1, th.Text(ui.TextSM, sec.label).In(fg).Layout),
			layout.Rigid(func(gtx C) D { return countBadge(gtx, th, badge) }),
		)
	})
}

// navRow is a h-8 rounded-md row: accent while on, accent/50 hovered.
func navRow(gtx C, th *ui.Theme, click *widget.Clickable, on bool, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		switch {
		case on:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				gtx.Constraints.Min.Y = gtx.Dp(navRowHeight)
				return layout.Inset{Left: navRowPadX, Right: navRowPadX}.Layout(gtx, func(gtx C) D {
					return layout.W.Layout(gtx, func(gtx C) D { return content(gtx, th.On(bg)) })
				})
			},
		)
	})
}

// countBadge is the mockup's amber override count, nothing for none.
func countBadge(gtx C, th *ui.Theme, n int) D {
	if n == 0 {
		return D{}
	}
	label := th.Text(navSmall, strconv.Itoa(n)).In(th.ToneWait).Weight(font.SemiBold).Leading(18)
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5)}.Layout(gtx, label.Layout)
	call := macro.Stop()
	size := image.Pt(max(d.Size.X, gtx.Dp(navBadgeSize)), gtx.Dp(navBadgeSize))
	box := gtx
	box.Constraints = layout.Exact(size)
	ui.Fill(box, th.Over(ui.Alpha(th.ToneWait, navBadgeFill)), navBadgeSize/2)
	defer op.Offset(image.Pt((size.X-d.Size.X)/2, (size.Y-d.Size.Y)/2)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return D{Size: size}
}

// results are the nav while the field has text: the count, then each hit as
// its path over its title, the one the arrows are on filled.
func (s *settingsScreen) results(gtx C, th *ui.Theme, hits []settingHit) D {
	n := &s.nav
	if len(hits) == 0 {
		return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5), Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			l := th.Text(ui.TextXS, fmt.Sprintf(settingsNoMatch, strings.TrimSpace(n.query.Text()))).Muted()
			l.MaxLines = 0
			return l.Layout(gtx)
		})
	}
	if len(n.hits) < len(hits) {
		n.hits = make([]widget.Clickable, len(hits))
	}
	for i := range hits {
		if n.hits[i].Clicked(gtx) {
			n.cursor = i
			s.choose(gtx, hits[i])
		}
	}
	count := fmt.Sprintf(settingsResultMany, len(hits))
	if len(hits) == 1 {
		count = fmt.Sprintf(settingsResultOne, 1)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Bottom: ui.Space(0.5)}.Layout(gtx, th.Text(navHitPath, strings.ToUpper(count)).Muted().Mono().Layout)
		}),
		layout.Flexed(1, func(gtx C) D {
			return n.list.Layout(gtx, len(hits), func(gtx C, i int) D {
				return layout.Inset{Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return s.hitRow(gtx, th, i, hits[i]) })
			})
		}),
	)
}

func (s *settingsScreen) hitRow(gtx C, th *ui.Theme, i int, h settingHit) D {
	click := &s.nav.hits[i]
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		switch {
		case i == s.nav.cursor:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		on := th.On(bg)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: navRowPadX, Right: navRowPadX, Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
					children := []layout.FlexChild{}
					if p := h.path(); p != "" {
						children = append(children, layout.Rigid(on.Text(navHitPath, p).Muted().Mono().Layout))
					}
					children = append(children, layout.Rigid(on.Text(ui.TextSM, h.title).Leading(19).Layout))
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
				})
			},
		)
	})
}
