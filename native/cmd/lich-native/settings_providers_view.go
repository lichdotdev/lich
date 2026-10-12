package main

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var (
	providerRowPadX   = ui.Space(2)
	providerRowPadY   = ui.Space(2.5)
	providerAddPadY   = ui.Space(3)
	providerHeadIcon  = ui.Space(5.5) // ProviderIcon size={22}
	providerTriggerW  = ui.Space(64)  // ProviderSelect's w-64
	providerAddInsetR = ui.Space(9)   // pr-9: under the switches, not the chevrons
	icProviderOpen    = icons.Lucide("chevron-right")
	icProviderBack    = icons.Lucide("chevron-left")
	icProviderAdd     = icons.Lucide("plus")
	icProviderDocs    = icons.Lucide("external-link")
	icProviderCheck   = icons.Lucide("refresh-cw")
)

// providerPaneRows are ProvidersPane's rows under the pane's header, base of
// them already drawn: the list, or one provider's screen when its row was
// opened and the provider is still on.
func (s *settingsScreen) providerPaneRows(th *ui.Theme, values scopedValues, base int) []layout.Widget {
	m := s.providers.snapshot()
	if !m.loaded {
		return []layout.Widget{th.Text(settingsDescSize, providersDetecting).Muted().Layout}
	}
	if p, ok := s.providerScreen(); ok {
		return s.providerScreenRows(th, values, m, p, base)
	}
	return s.providerListRows(th, values, m, base)
}

// providerScreen is the provider whose screen is open. One turned off while
// its screen was open resolves back to the list, as the nav resolves an id it
// can no longer place.
func (s *settingsScreen) providerScreen() (providerInfo, bool) {
	p, ok := s.providers.snapshot().find(s.providers.open)
	return p, ok && p.enabled
}

// providerRow is rowFor plus what drawRow does to a row it draws: Use Global,
// and the light the search leaves on the row it chose.
func (s *settingsScreen) providerRow(gtx C, e settingEntry, values scopedValues) settingRow {
	r := s.rowFor(e, values)
	if r.reset != nil && r.reset.Clicked(gtx) {
		s.useGlobal(e.key, s.scope)
		s.refresh()
	}
	if s.lit == e.title && gtx.Now.Before(s.litUntil) {
		r.lit = min(1, float32(s.litUntil.Sub(gtx.Now))/float32(settingsLitFadingFor))
		gtx.Execute(op.InvalidateCmd{})
	}
	return r
}

// scrollToBlock scrolls to the block titled title, at the pane's row index
// at, when the search chose it.
func (s *settingsScreen) scrollToBlock(title string, at int) {
	rows := sectionByID(sectionProviders).rows
	if s.scrollTo >= 0 && s.scrollTo < len(rows) && rows[s.scrollTo].title == title {
		s.pane.ScrollTo(at)
		s.scrollTo = -1
	}
}

// providerListRows are the list: the default provider, the enabled providers
// each a row that opens its screen, and the fold of the ones that are off.
func (s *settingsScreen) providerListRows(th *ui.Theme, values scopedValues, m providersMachine, base int) []layout.Widget {
	pr := &s.providers
	enabled := m.enabled()
	var off []providerInfo
	for _, p := range m.list {
		if !p.enabled {
			off = append(off, p)
		}
	}
	// Opened by hand, except on a machine with nothing on: there the list of
	// what could be turned on is all this pane has to say.
	if !pr.addingSet {
		pr.adding, pr.addingSet = len(enabled) == 0, true
	}
	var rows []layout.Widget
	if len(enabled) > 0 {
		s.scrollToBlock(providerDefaultTitle, base+len(rows))
		rows = append(rows, func(gtx C) D { return s.providerDefaultRow(gtx, th, values, m, true) },
			func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(th.Separator), ui.VGap(4))
			})
	}
	for _, p := range enabled {
		rows = append(rows, func(gtx C) D { return s.providerListRow(gtx, th, values, m, p) })
	}
	if len(off) > 0 {
		rows = append(rows, func(gtx C) D { return s.providerAddToggle(gtx, th, len(off)) })
		if pr.adding {
			rows = append(rows, func(gtx C) D { return s.providerAddList(gtx, th, off) })
		}
	}
	return rows
}

// providerDefaultRow is DefaultProviders: one row, in the scope system. A
// project's own default is a change from Global like any other overridable
// row, and Use Global is what clears it.
func (s *settingsScreen) providerDefaultRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, first bool) D {
	e := settingEntry{title: providerDefaultTitle, key: providerDefaultKey, scope: scopeOverridable}
	r := s.providerRow(gtx, e, values)
	list := m.launchList()
	global := resolveProjectDefaultProvider(list, m.globals[providerDefaultKey], "")
	shown := global
	if s.scope != "" {
		stored, _ := values.set(providerDefaultKey, s.scope)
		shown = resolveProjectDefaultProvider(list, m.globals[providerDefaultKey], stored)
	}
	r.description, r.globalValue = providerDefaultDesc, m.nameOf(global)
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return s.providerPicker(gtx, th, m, shown) })
}

// providerPicker is ProviderSelect: the name of the chosen provider in an
// outline trigger, the enabled ones with their marks in the list under it.
func (s *settingsScreen) providerPicker(gtx C, th *ui.Theme, m providersMachine, shown string) D {
	pr := &s.providers
	pop := th.DropdownPanel(s.root, &pr.picker, providerTriggerW, func(gtx C, th *ui.Theme) D {
		var rows []layout.FlexChild
		for _, p := range m.enabled() {
			rows = append(rows, layout.Rigid(func(gtx C) D { return s.providerOption(gtx, th, p, p.id == shown) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
	return pop.Layout(gtx, func(gtx C) D { return selectTrigger(gtx, th, m.nameOf(shown), providerTriggerW) })
}

// providerOption is one SelectItem: the mark and the name, checked when
// chosen; picking it stores it and closes the list.
func (s *settingsScreen) providerOption(gtx C, th *ui.Theme, p providerInfo, chosen bool) D {
	pr := &s.providers
	if pr.options == nil {
		pr.options = map[string]*widget.Clickable{}
	}
	click := clickableOf(pr.options, p.id)
	if click.Clicked(gtx) {
		s.setProviderDefault(p.id)
		pr.picker.Close()
	}
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(fontRowHeight)))
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return providerIcon(p.id).Layout(gtx, ui.Space(3.5), on.Foreground) }),
				ui.Gap(2),
				layout.Flexed(1, on.Text(ui.TextSM, p.name).Layout),
				layout.Rigid(func(gtx C) D {
					if !chosen {
						return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)}
					}
					return icCheck.Layout(gtx, ui.Space(4), on.Foreground)
				}),
			)
		})
	})
}

// providerListRow is ProviderRow: the whole row opens the provider's screen,
// and the switch is drawn over the opening area rather than inside it, so a
// click that turns the provider off does not also walk into it.
func (s *settingsScreen) providerListRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo) D {
	st := s.providers.state(p.id)
	if st.open.Clicked(gtx) {
		s.providers.open = p.id
	}
	if st.toggle.Update(gtx) {
		s.setProviderEnabled(p.id, st.toggle.Value)
	}
	st.toggle.Value = p.enabled
	plan, hasPlan := s.planOf(p.id)
	summary, mono := planSummary(plan, hasPlan), false
	if summary == "" {
		summary, mono = p.binary, true
	}
	projectDefault := ""
	if s.scope != "" {
		projectDefault, _ = values.set(providerDefaultKey, s.scope)
	}
	tags := append(providerTags(p.id, projectDefault, m.skipLevel(p.id)), s.profileTags(p.id, values, m)...)

	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return st.open.Layout(gtx, func(gtx C) D {
				if st.open.Hovered() {
					ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
				}
				return D{Size: gtx.Constraints.Min}
			})
		}),
		layout.Stacked(func(gtx C) D {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx C) D { return providerIcon(p.id).Layout(gtx, ui.Space(4), th.Foreground) }),
				ui.Gap(3),
				layout.Rigid(th.Text(ui.TextSM, p.name).Weight(font.Medium).Layout),
				ui.Gap(3),
			}
			sum := th.Text(ui.TextXS, summary).Muted()
			if mono {
				sum = sum.Mono()
			}
			children = append(children, layout.Flexed(1, sum.Layout))
			for _, tag := range tags {
				children = append(children, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, tag).Muted().Layout))
			}
			children = append(children,
				ui.Gap(2),
				layout.Rigid(th.Switch(&st.toggle).Layout),
				ui.Gap(3),
				layout.Rigid(func(gtx C) D { return icProviderOpen.Layout(gtx, ui.Space(4), th.MutedForeground) }),
			)
			return layout.Inset{Left: providerRowPadX, Right: providerRowPadX, Top: providerRowPadY, Bottom: providerRowPadY}.Layout(gtx,
				func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...) })
		}),
	)
}

// providerAddToggle is the fold that holds the providers that are off.
func (s *settingsScreen) providerAddToggle(gtx C, th *ui.Theme, count int) D {
	pr := &s.providers
	if pr.addToggle.Clicked(gtx) {
		pr.adding = !pr.adding
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return pr.addToggle.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		ink := th.MutedForeground
		if pr.addToggle.Hovered() {
			bg, ink = th.Over(ui.Alpha(th.Accent, ghostHoverFill)), th.Foreground
		}
		lead := icProviderAdd
		if pr.adding {
			lead = icChevronDown
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: providerRowPadX, Right: providerRowPadX, Top: providerRowPadY, Bottom: providerRowPadY}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return lead.Layout(gtx, ui.Space(4), ink) }),
						ui.Gap(2),
						layout.Rigid(th.Text(ui.TextSM, providersAdd).In(ink).Layout),
						ui.Gap(2),
						layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf(providersMoreFound, count)).Muted().Layout),
					)
				})
			},
		)
	})
}

// providerAddList is what the fold opens on: Check again over a row per
// provider that is off.
func (s *settingsScreen) providerAddList(gtx C, th *ui.Theme, off []providerInfo) D {
	pr := &s.providers
	if pr.recheck.Clicked(gtx) {
		pr.recheckPath(s)
	}
	pr.mu.Lock()
	checking, failure := pr.rechecking, pr.recheckErr
	pr.mu.Unlock()
	check := th.Button(&pr.recheck, providersCheckAgain)
	check.Variant, check.Size, check.Icon, check.Disabled = ui.VariantGhost, ui.SizeSM, icProviderCheck, checking
	if checking {
		check.Text = providersChecking
	}
	head := []layout.FlexChild{layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} })}
	if failure != "" {
		head = append(head, layout.Rigid(th.Text(ui.TextXS, failure).In(th.Destructive).Layout), ui.Gap(2))
	}
	head = append(head, layout.Rigid(check.Layout))
	rows := []layout.FlexChild{layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, head...) })}
	for i, p := range off {
		rows = append(rows, layout.Rigid(func(gtx C) D { return s.providerToggleRow(gtx, th, p, i == 0) }))
	}
	return layout.Inset{Left: providerRowPadX, Right: providerAddInsetR}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// providerToggleRow is ProviderToggleRow: what the provider is, whether the
// machine has it, and the switch. One that is not installed cannot be turned
// on, but one already on stays togglable, so it is never trapped on by a
// binary that moved.
func (s *settingsScreen) providerToggleRow(gtx C, th *ui.Theme, p providerInfo, first bool) D {
	st := s.providers.state(p.id)
	if st.toggle.Update(gtx) {
		s.setProviderEnabled(p.id, st.toggle.Value)
	}
	st.toggle.Value = p.enabled
	if st.docs.Clicked(gtx) {
		s.openExternal(p.docs)
	}
	sw := th.Switch(&st.toggle)
	sw.Disabled = !p.installed && !p.enabled
	sub := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, installSummary(p)).Muted().Layout)}
	if !p.installed {
		sub = []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, providersNotFound).Muted().Layout)}
		if p.docs != "" {
			sub = append(sub, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, "·").Muted().Layout), ui.Gap(1.5),
				layout.Rigid(func(gtx C) D { return docsLink(gtx, th, &st.docs, providersHowToGet) }))
		}
	}
	row := func(gtx C) D {
		return layout.Inset{Top: providerAddPadY, Bottom: providerAddPadY}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return providerIcon(p.id).Layout(gtx, ui.Space(4), th.Foreground) }),
				ui.Gap(3),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(th.Text(ui.TextSM, p.name).Weight(font.Medium).Layout),
						layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, sub...) }),
					)
				}),
				ui.Gap(4),
				layout.Rigid(sw.Layout),
			)
		})
	}
	if first {
		return row(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(th.Separator), layout.Rigid(row))
}

// docsLink is ProviderDocsLink: the page documenting the provider, text and
// the arrow out of the window.
func docsLink(gtx C, th *ui.Theme, click *widget.Clickable, label string) D {
	return click.Layout(gtx, func(gtx C) D {
		ink := th.MutedForeground
		if click.Hovered() {
			ink = th.Foreground
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, label).In(ink).Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return icProviderDocs.Layout(gtx, ui.Space(3), ink) }),
		)
	})
}

// providerHeader is ProviderDetail's head: the way back, then the provider's
// mark, name and documentation link, and the switch that turns it off.
func (s *settingsScreen) providerHeader(th *ui.Theme, p providerInfo) []layout.Widget {
	back := func(gtx C) D {
		pr := &s.providers
		if pr.back.Clicked(gtx) {
			pr.open = ""
		}
		return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return pr.back.Layout(gtx, func(gtx C) D {
				ink := th.MutedForeground
				if pr.back.Hovered() {
					ink = th.Foreground
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icProviderBack.Layout(gtx, ui.Space(3.5), ink) }),
					ui.Gap(1),
					layout.Rigid(th.Text(ui.TextXS, providerBack).In(ink).Layout),
				)
			})
		})
	}
	title := func(gtx C) D {
		pr := &s.providers
		if pr.header.Update(gtx) {
			s.setProviderEnabled(p.id, pr.header.Value)
		}
		pr.header.Value = p.enabled
		if pr.docs.Clicked(gtx) {
			s.openExternal(p.docs)
		}
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return providerIcon(p.id).Layout(gtx, providerHeadIcon, th.Foreground) }),
			ui.Gap(3),
			layout.Rigid(th.Text(settingsTitle, p.name).Weight(font.SemiBold).Leading(32).Layout),
		}
		if p.docs != "" {
			children = append(children, ui.Gap(4), layout.Rigid(func(gtx C) D { return docsLink(gtx, th, &pr.docs, providerDocsLabel) }))
		}
		children = append(children,
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(th.Text(ui.TextXS, providerEnabledLabel).Muted().Layout),
			ui.Gap(2),
			layout.Rigid(th.Switch(&pr.header).Layout),
		)
		return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}
	return []layout.Widget{back, title}
}

// providerScreenRows are one provider's blocks, each present only where the
// provider has the setting.
func (s *settingsScreen) providerScreenRows(th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, base int) []layout.Widget {
	var rows []layout.Widget
	first := true
	add := func(title string, draw func(gtx C, first bool) D) {
		s.scrollToBlock(title, base+len(rows))
		isFirst := first
		first = false
		rows = append(rows, func(gtx C) D { return draw(gtx, isFirst) })
	}
	claude := p.id == claudeKind && s.providers.claude.book != nil
	if claude {
		s.ensureProfiles()
		add(profileTitle, func(gtx C, first bool) D { return s.profilePickerRow(gtx, th, values, m, first) })
		add(profilesTitle, func(gtx C, first bool) D { return s.profilesBlock(gtx, th, values, first) })
		slug := s.scopeProfile(values, m)
		if plan, ok := s.claudePlan(slug); ok {
			add(planTitle, func(gtx C, first bool) D { return s.claudePlanBlock(gtx, th, values, slug, plan, first) })
		}
	} else {
		if plan, ok := s.planOf(p.id); ok {
			add(planTitle, func(gtx C, first bool) D { return s.planBlock(gtx, th, values, plan, first, "") })
		}
		add(binTitle, func(gtx C, first bool) D { return s.binBlock(gtx, th, values, m, p, first) })
	}
	if hasSkipFlag(p.id) {
		add(skipTitle, func(gtx C, first bool) D { return s.skipBlock(gtx, th, values, m, p, first) })
	}
	if supportsUltracode(p.id) {
		add(ultracodeTitle, func(gtx C, first bool) D { return s.ultracodeRow(gtx, th, values, m, p, first) })
	}
	if supportsSubagentCards(p.id) {
		add(subagentTitle, func(gtx C, first bool) D { return s.subagentRow(gtx, th, values, m, p, first) })
	}
	if hasPasteUnfold(p.id) {
		add(pasteTitle, func(gtx C, first bool) D { return s.pasteRow(gtx, th, values, m, p, first) })
	}
	if canResume(p.id) {
		add(restoreTitle, func(gtx C, first bool) D { return s.restoreBlock(gtx, th, values, m, p, first) })
	}
	if claude {
		add(binTitle, func(gtx C, first bool) D { return s.advancedBinaryFold(gtx, th, values, m, p, first) })
	}
	return rows
}

// switchRow is a global-only switch under a provider's screen: seen from a
// project it is dimmed with no control, as every global row is.
func (s *settingsScreen) switchRow(gtx C, th *ui.Theme, values scopedValues, first bool, title, key, desc string, sw *ui.SwitchState, on bool) D {
	r := s.providerRow(gtx, settingEntry{title: title, key: key}, values)
	r.description = desc
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	if sw.Update(gtx) {
		s.writeProviderKey(key, "", strconv.FormatBool(sw.Value), nil)
	}
	sw.Value = on
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return th.Switch(sw).Layout(gtx) })
}

func (s *settingsScreen) ultracodeRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	key := providerUltracodeKey(p.id)
	return s.switchRow(gtx, th, values, first, ultracodeTitle, key, fmt.Sprintf(ultracodeDesc, p.name), &s.providers.ultra, m.globals[key] == "true")
}

// subagentRow is on until turned off, so only a stored "false" reads as off.
func (s *settingsScreen) subagentRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	key := providerSubagentKey(p.id)
	return s.switchRow(gtx, th, values, first, subagentTitle, key, fmt.Sprintf(subagentDesc, p.name), &s.providers.agents, m.globals[key] != "false")
}

func (s *settingsScreen) pasteRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	key := providerPasteKey(p.id)
	desc := fmt.Sprintf(pasteDescNextSession, p.name)
	if pasteUnfoldTiming(p.id) == pasteNextPaste {
		desc = fmt.Sprintf(pasteDescNextPaste, p.name, p.name)
	}
	return s.switchRow(gtx, th, values, first, pasteTitle, key, desc, &s.providers.unfold, m.globals[key] == "true")
}

// skipBlock is the skip ladder. The pair of keys is the storage and the rung
// is the choice, so one rung writes both.
func (s *settingsScreen) skipBlock(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	e := settingEntry{title: skipTitle, key: providerSkipKey(p.id, false)}
	r := s.providerRow(gtx, e, values)
	r.description = fmt.Sprintf(skipDesc, p.name, skipFlags[p.id])
	level := m.skipLevel(p.id)
	return s.ladderBlock(gtx, th, r, first, &s.providers.skip, skipRungs, level, func(next string) { s.chooseSkip(p, level, next) })
}

// restoreBlock answers the resume prompt ahead of time. It stores the key the
// spawn gate reads (restoreKey), which is the same one the prompt's "always"
// writes, so the gate's next decision is this block's choice.
func (s *settingsScreen) restoreBlock(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	key := restoreKey(p.id)
	r := s.providerRow(gtx, settingEntry{title: restoreTitle, key: key}, values)
	r.description = fmt.Sprintf(restoreDesc, p.name)
	choice := restoreChoice(m.globals[key])
	return s.ladderBlock(gtx, th, r, first, &s.providers.restore, restoreRungs, choice,
		func(next string) { s.writeProviderKey(key, "", next, nil) })
}

// ladderBlock is a global-only choice of one rung: the segmented control, and
// under it what the chosen rung means.
func (s *settingsScreen) ladderBlock(gtx C, th *ui.Theme, r settingRow, first bool, enum *widget.Enum,
	rungs []struct{ value, label, consequence string }, current string, choose func(next string)) D {
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	if enum.Update(gtx) {
		choose(enum.Value)
	}
	enum.Value = current
	items := make([]ui.ToggleItem, len(rungs))
	consequence := ""
	for i, rung := range rungs {
		items[i] = ui.ToggleItem{Value: rung.value, Text: rung.label}
		if rung.value == current {
			consequence = rung.consequence
		}
	}
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D {
		g := th.ToggleGroup(enum, items...)
		g.Size = ui.ToggleSizeSM
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return layout.W.Layout(gtx, g.Layout) }),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, consequence).Muted()) }),
		)
	})
}

// chooseSkip writes a descent straight through and asks before a climb:
// taking the automation away is never the harder direction.
func (s *settingsScreen) chooseSkip(p providerInfo, current, next string) {
	if skipClimbs(current, next) {
		s.providers.change, s.providers.asking = skipChange{provider: p, level: next}, true
		s.invalidate() // the dialog is laid out before the pane, a frame behind the click
		return
	}
	s.writeSkip(p.id, next)
}

func (s *settingsScreen) writeSkip(kind, level string) {
	here, worktrees := skipLevelPair(level)
	s.writeProviderKey(providerSkipKey(kind, false), "", strconv.FormatBool(here), nil)
	s.writeProviderKey(providerSkipKey(kind, true), "", strconv.FormatBool(worktrees), nil)
}

// layoutProviderDialog is the climb's ConfirmDialog. The window lays it out
// with its own constraints, as every dialog.
func (s *settingsScreen) layoutProviderDialog(gtx C, th *ui.Theme) {
	pr := &s.providers
	if pr.confirm.Clicked(gtx) && pr.asking {
		s.writeSkip(pr.change.provider.id, pr.change.level)
		pr.asking = false
	}
	if pr.cancel.Clicked(gtx) {
		pr.asking = false
	}
	c := pr.change
	label, consequence := skipRung(c.level)
	on := th.On(th.Popover)
	cancel := on.Button(&pr.cancel, dialogCancel)
	cancel.Variant = ui.VariantGhost
	confirm := on.Button(&pr.confirm, skipConfirm)
	confirm.Variant = ui.VariantDestructive
	dlg := th.Dialog(&pr.dialog, pr.asking && s.open, fmt.Sprintf(skipConfirmTitle, strings.ToLower(label)),
		fmt.Sprintf(skipConfirmDesc, c.provider.name, skipFlags[c.provider.id], consequence))
	dlg.MaxWidth = ui.DialogWidthXL
	dlg.Actions = []layout.Widget{cancel.Layout, confirm.Layout}
	if dlg.Layout(gtx) {
		pr.asking = false
	}
}
