package main

import (
	"log"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// Settings › Subagents: per enabled provider, whether its subagents open as
// cards, and on its page what a card brings of the user's own setup beyond
// lich and the project (internal/subagent, cards.go). Global only.

const (
	sectionSubagents = "subagents"
	// subagentsKey names the section's one searchable row; the settings live
	// in the subagent card store, not under this key.
	subagentsKey = "subagent.cards"
)

// subagentSettings is the screen's state: the store it reads, the provider
// whose page is open, and every control drawn.
type subagentSettings struct {
	store    *subagentCardsStore
	open     string // the provider whose page shows, "" for the list
	rows     map[string]*ui.Clickable
	switches map[string]*ui.SwitchState
	tips     map[string]*ui.Tooltip
	back     ui.Clickable
}

func (u *subagentSettings) init() {
	if u.rows == nil {
		u.rows, u.switches, u.tips = map[string]*ui.Clickable{}, map[string]*ui.SwitchState{}, map[string]*ui.Tooltip{}
	}
}

func (u *subagentSettings) switchOf(id string) *ui.SwitchState {
	sw, ok := u.switches[id]
	if !ok {
		sw = new(ui.SwitchState)
		u.switches[id] = sw
	}
	return sw
}

func (u *subagentSettings) tipOf(id string) *ui.Tooltip {
	t, ok := u.tips[id]
	if !ok {
		t = new(ui.Tooltip)
		u.tips[id] = t
	}
	return t
}

// subagentsPaneRows is the list under its title, or a provider's page.
func (s *settingsScreen) subagentsPaneRows(th *ui.Theme, sec settingsSection) []layout.Widget {
	u := &s.subagents
	u.init()
	if s.scope != "" {
		return []layout.Widget{s.pageTitle(th, sec), s.globalElsewhere(th, true)}
	}
	if u.store == nil {
		return nil
	}
	enabled := s.providers.snapshot().enabled()
	if u.open != "" {
		for _, p := range enabled {
			if c, ok := u.store.provider(p.id); ok && p.id == u.open {
				return s.subagentProviderRows(th, sec, p, c)
			}
		}
		u.open = ""
	}
	rows := []layout.Widget{titled(th, sec.label(), i18n.T("settings.subagentsSettings.subtitle")),
		groupHeading(th, i18n.T("settings.settings.section.providers"), true)}
	for _, p := range enabled {
		if c, ok := u.store.provider(p.id); ok {
			rows = append(rows, func(gtx C) D { return s.subagentListRow(gtx, th, p, c) })
		}
	}
	return rows
}

// titled is a page's title with its subtitle under it.
func titled(th *ui.Theme, title, subtitle string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, th.Text(settingsTitle, title).Weight(font.SemiBold).Leading(32).Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, th.Text(ui.TextXS, subtitle).Muted().Wrapped().Layout)
			}),
		)
	}
}

// subagentListRow is one provider: its mark, name and what a card brings,
// the cards switch drawn over the area that opens its page, and the chevron.
func (s *settingsScreen) subagentListRow(gtx C, th *ui.Theme, p providerInfo, c api.SubagentCardProvider) D {
	u := &s.subagents
	open := clickableOf(u.rows, p.id)
	if open.Clicked(gtx) {
		u.open = p.id
		s.pane.ScrollTo(0)
		s.invalidate()
	}
	sw := s.cardsSwitch(gtx, p.id, c.Cards)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return open.Layout(gtx, func(gtx C) D {
				if open.Hovered() {
					ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
				}
				return D{Size: gtx.Constraints.Min}
			})
		}),
		layout.Stacked(func(gtx C) D {
			text := func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextSM, p.name).Weight(font.Medium).Layout),
					layout.Rigid(th.Text(ui.TextXS, subagentDetail(c, p.name)).Muted().Layout),
				)
			}
			return layout.Inset{Left: providerRowPadX, Right: providerRowPadX, Top: providerRowPadY, Bottom: providerRowPadY}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, p.id, providerMarkBox, providerMarkCell) }),
					ui.Gap(2),
					layout.Flexed(1, text),
					ui.Gap(2),
					layout.Rigid(th.Switch(sw).Layout),
					ui.Gap(3),
					layout.Rigid(func(gtx C) D { return icProviderOpen.Layout(gtx, ui.Space(4), th.MutedForeground) }),
				)
			})
		}),
	)
}

// cardsSwitch is provider's cards switch, its press filed and sent.
func (s *settingsScreen) cardsSwitch(gtx C, provider string, on bool) *ui.SwitchState {
	u := &s.subagents
	sw := u.switchOf(provider)
	if sw.Update(gtx) {
		u.store.setCards(provider, sw.Value)
		s.invalidate() // the rows above were built before the press
		opts := api.SubagentSetCardsOptions{Provider: provider, On: sw.Value}
		go s.saveSubagents(func() error { return s.client.SetSubagentCards(s.ctx, opts) })
		return sw
	}
	sw.Value = on
	return sw
}

// saveSubagents sends a press, and on a refusal says so and reads the
// settings again so the screen shows what holds.
func (s *settingsScreen) saveSubagents(save func() error) {
	if err := save(); err != nil {
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.subagentsSettings.saveFailed", i18n.Args{"error": backend.ErrorText(err)})})
		if err := s.subagents.store.load(s.ctx, s.client); err != nil {
			log.Printf("subagents: reload: %v", err)
		}
	}
	s.invalidate()
}

// subagentDetail is a provider row's one line: what a card of it brings.
func subagentDetail(c api.SubagentCardProvider, provider string) string {
	switch {
	case !c.Cards:
		return i18n.T("settings.subagentsSettings.detailOff")
	case !c.Lean:
		return i18n.T("settings.subagentsSettings.detailEverything", i18n.Args{"provider": provider})
	}
	var on []string
	for _, it := range c.Items {
		if it.On {
			on = append(on, subagentItemName(it))
		}
	}
	if len(on) == 0 {
		return i18n.T("settings.subagentsSettings.detailNothing")
	}
	return i18n.T("settings.subagentsSettings.detailBrings", i18n.Args{"items": strings.Join(on, ", ")})
}

// subagentItemName is how an item is named on a row and in a card's tooltip.
func subagentItemName(it api.SubagentCardItem) string {
	if it.Kind == api.SubagentBringAlso {
		name, _ := subagentAlsoText(it.ID)
		return name
	}
	return it.Name
}

// subagentAlsoText is an all-or-nothing item's name and the line under it.
func subagentAlsoText(id string) (name, detail string) {
	switch id {
	case api.SubagentAlsoMemory:
		return i18n.T("settings.subagentsSettings.memory"), i18n.T("settings.subagentsSettings.memoryDetail")
	case api.SubagentAlsoClaudeSetup:
		return i18n.T("settings.subagentsSettings.claudeSetup"), i18n.T("settings.subagentsSettings.claudeSetupDetail")
	case api.SubagentAlsoExtensions:
		return i18n.T("settings.subagentsSettings.extensions"), i18n.T("settings.subagentsSettings.extensionsDetail")
	case api.SubagentAlsoSkills:
		return i18n.T("settings.subagentsSettings.skills"), i18n.T("settings.subagentsSettings.skillsDetail")
	case api.SubagentAlsoRules:
		return i18n.T("settings.subagentsSettings.rules"), i18n.T("settings.subagentsSettings.rulesDetail")
	}
	return id, ""
}

// subagentProviderRows is a provider's page: the way back, its mark, name and
// cards switch, the line saying what its cards are, and, while they are on,
// what one can also bring.
func (s *settingsScreen) subagentProviderRows(th *ui.Theme, sec settingsSection, p providerInfo, c api.SubagentCardProvider) []layout.Widget {
	rows := []layout.Widget{s.subagentBack(th, sec), s.subagentTitle(th, p, c)}
	if !c.Cards || !c.Lean {
		return rows
	}
	first := true
	for _, kind := range []string{api.SubagentBringMCP, api.SubagentBringPlugin, api.SubagentBringAlso} {
		items := itemsOfKind(c.Items, kind)
		if len(items) == 0 {
			continue
		}
		rows = append(rows, subagentGroupHeading(th, kind, items, first))
		first = false
		for i, it := range items {
			rows = append(rows, func(gtx C) D { return s.subagentItemRow(gtx, th, p.id, it, i == 0) })
		}
	}
	if p.id == "opencode" {
		rows = append(rows, func(gtx C) D {
			return layout.Inset{Left: rowPadX, Top: ui.Space(3)}.Layout(gtx, th.Text(ui.TextXS, i18n.T("settings.subagentsSettings.opencodePlugins")).Muted().Wrapped().Layout)
		})
	}
	return rows
}

func itemsOfKind(items []api.SubagentCardItem, kind string) []api.SubagentCardItem {
	var out []api.SubagentCardItem
	for _, it := range items {
		if it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

// subagentGroupHeading is a group's heading, with how many of its items are
// on beside the MCP servers and Plugins ones.
func subagentGroupHeading(th *ui.Theme, kind string, items []api.SubagentCardItem, first bool) layout.Widget {
	label, count := i18n.T("settings.subagentsSettings.also"), ""
	if kind != api.SubagentBringAlso {
		label = i18n.T("settings.settings.section.mcp")
		if kind == api.SubagentBringPlugin {
			label = i18n.T("settings.settings.section.plugins")
		}
		on := 0
		for _, it := range items {
			if it.On {
				on++
			}
		}
		count = i18n.T("settings.subagentsSettings.onOf", i18n.Args{"on": on, "count": len(items)})
	}
	inset := layout.Inset{Left: rowPadX, Top: groupGapAbove, Bottom: groupGapBelow}
	if first {
		inset.Top = 0
	}
	return func(gtx C) D {
		return inset.Layout(gtx, func(gtx C) D {
			line := []layout.FlexChild{layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(label)).Muted().Weight(font.SemiBold).Layout)}
			if count != "" {
				line = append(line, ui.Gap(1.5), layout.Rigid(th.Text(ui.Text2XS, count).Muted().Mono().Layout))
			}
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, line...)
		})
	}
}

// subagentItemRow is one item with its switch: a server or plugin over where
// it comes from, an all-or-nothing item over what it is. An MCP server's row
// says on hover what one costs a card.
func (s *settingsScreen) subagentItemRow(gtx C, th *ui.Theme, provider string, it api.SubagentCardItem, first bool) D {
	u := &s.subagents
	sw := u.switchOf(provider + "/" + it.ID)
	if sw.Update(gtx) {
		u.store.setBring(provider, it.ID, sw.Value)
		s.invalidate() // the group's count was drawn before the press
		opts := api.SubagentSetBringOptions{Provider: provider, Item: it.ID, On: sw.Value}
		go s.saveSubagents(func() error { return s.client.SetSubagentBring(s.ctx, opts) })
	} else {
		sw.Value = it.On
	}
	r := settingRow{title: subagentItemName(it)}
	if it.Kind == api.SubagentBringAlso {
		_, r.description = subagentAlsoText(it.ID)
	} else {
		origin := it.Origin
		if origin == api.SubagentOriginLibrary {
			origin = i18n.T("settings.subagentsSettings.originLibrary")
		}
		r.sample = th.Text(ui.TextXS, origin).Muted().Mono().Layout
	}
	draw := func(gtx C) D {
		return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return th.Switch(sw).Layout(gtx) })
	}
	if it.Kind != api.SubagentBringMCP {
		return draw(gtx)
	}
	return th.Tooltip(s.root, u.tipOf(provider+"/"+it.ID), i18n.T("settings.subagentsSettings.mcpCost")).Layout(gtx, draw)
}

// subagentBack is the way back to the list.
func (s *settingsScreen) subagentBack(th *ui.Theme, sec settingsSection) layout.Widget {
	u := &s.subagents
	return func(gtx C) D {
		if u.back.Clicked(gtx) {
			u.open = ""
			s.invalidate()
		}
		return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return u.back.Layout(gtx, func(gtx C) D {
				ink := th.MutedForeground
				if u.back.Hovered() {
					ink = th.Foreground
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icProviderBack.Layout(gtx, ui.Space(3.5), ink) }),
					ui.Gap(1),
					layout.Rigid(th.Text(ui.TextXS, sec.label()).In(ink).Layout),
				)
			})
		})
	}
}

// subagentTitle is the provider's mark, name and cards switch, over the line
// that says what its cards are now.
func (s *settingsScreen) subagentTitle(th *ui.Theme, p providerInfo, c api.SubagentCardProvider) layout.Widget {
	u := &s.subagents
	title := func(gtx C) D {
		sw := s.cardsSwitch(gtx, p.id, c.Cards)
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return providerGlyph(gtx, th, p.id, providerMarkBox, providerMarkCell) }),
				ui.Gap(2),
				layout.Rigid(th.Text(settingsTitle, p.name).Weight(font.SemiBold).Leading(32).Layout),
				layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
				layout.Rigid(func(gtx C) D {
					// the switch says whether cards are on; its name is its tooltip
					return th.Tooltip(s.root, u.tipOf(p.id), i18n.T("settings.subagentsSettings.cardsSwitch")).Layout(gtx, th.Switch(sw).Layout)
				}),
			)
		})
	}
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(title),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Bottom: ui.Space(4)}.Layout(gtx, th.Text(ui.TextXS, subagentSubtitle(c, p.name)).Muted().Wrapped().Layout)
			}),
		)
	}
}

// subagentSubtitle is the line under a provider's name.
func subagentSubtitle(c api.SubagentCardProvider, provider string) string {
	switch {
	case !c.Cards:
		return i18n.T("settings.subagentsSettings.subtitleOff", i18n.Args{"provider": provider})
	case !c.Lean:
		return i18n.T("settings.subagentsSettings.subtitleEverything", i18n.Args{"provider": provider})
	}
	return i18n.T("settings.subagentsSettings.subtitleLean")
}
