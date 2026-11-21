package main

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// toolsMetaGap is the gap, in spacing steps, before a row's muted end.
const toolsMetaGap = 3

var (
	icToolsPending  = icons.Lucide("circle-dashed")
	icToolsFailed   = icons.Lucide("x")
	icToolsAuth     = icons.Lucide("circle-alert")
	icToolsOff      = icons.Lucide("circle")
	icToolsBack     = icons.Lucide("chevron-left")
	toolsMaxH       = unit.Dp(460)
	toolsLabelW     = ui.Space(32)
	toolsGlyph      = ui.Space(3.5)
	toolsGroupOrder = []string{"user", "project", "local", "claudeai", "other"}
)

// chatToolsPanel draws the open panel over the composer's box, as wide as it
// and a gap above it, deferred so it stands over the conversation, no
// taller than room: the slash list's place.
func (u *window) chatToolsPanel(gtx C, th *ui.Theme, p *chatPane, sv sessionView, room int) {
	t := &p.tools
	if !t.opened() {
		return
	}
	u.pollTools(gtx, p, sv.ID)
	t.mu.Lock()
	mcp, skills, loaded := t.mcp, t.skills, t.loaded
	t.mu.Unlock()
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, max(min(gtx.Dp(toolsMaxH), room), 0))}
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		switch {
		case !loaded:
			return toolsNote(gtx, th, i18n.T("chat.tools.loading"))
		case t.kind == chatMCPCommand:
			return u.mcpView(gtx, th, p, sv, mcp)
		}
		return u.skillsView(gtx, th, p, sv, skills)
	})
	call := macro.Stop()
	m := op.Record(gtx.Ops)
	op.Offset(image.Pt(0, -d.Size.Y-gtx.Dp(ui.Space(2)))).Add(gtx.Ops)
	call.Add(gtx.Ops)
	op.Defer(gtx.Ops, m.Stop())
}

func toolsNote(gtx C, th *ui.Theme, s string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.UniformInset(ui.Space(2)).Layout(gtx, th.Text(ui.TextSM, s).Muted().Wrapped().Layout)
}

// toolsHead is the panel's title, its count muted beside it and an optional
// control at the end.
func toolsHead(gtx C, th *ui.Theme, title, count string, end layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	row := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextSM, title).Weight(font.SemiBold).Layout),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextXS, count).Muted().Layout),
	}
	if end != nil {
		row = append(row, layout.Rigid(end))
	}
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(1), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// toolsGroup is where a server's scope files it in the list.
func toolsGroup(scope string) string {
	if slices.Contains(toolsGroupOrder, scope) {
		return scope
	}
	return "other"
}

func toolsGroupLabel(g string) string {
	switch g {
	case "user":
		return i18n.T("chat.tools.group.user")
	case "project":
		return i18n.T("chat.tools.group.project")
	case "local":
		return i18n.T("chat.tools.group.local")
	case "claudeai":
		return i18n.T("chat.tools.group.claudeai")
	}
	return i18n.T("chat.tools.group.misc")
}

// mcpOrdered is the servers in the order the list draws them: by group, each
// in the provider's order.
func mcpOrdered(servers []api.ChatMCPServer) []api.ChatMCPServer {
	var out []api.ChatMCPServer
	for _, g := range toolsGroupOrder {
		for _, s := range servers {
			if toolsGroup(s.Scope) == g {
				out = append(out, s)
			}
		}
	}
	return out
}

// mcpStatus is a server's glyph and its state in words.
func mcpStatus(th *ui.Theme, s api.ChatMCPServer) (*icons.Icon, color.NRGBA, string) {
	switch s.Status {
	case api.MCPConnected:
		return icCheck, th.TonePass, i18n.T("chat.tools.status.connected")
	case api.MCPPending:
		return icToolsPending, th.MutedForeground, i18n.T("chat.tools.status.pending")
	case api.MCPFailed:
		return icToolsFailed, th.Destructive, i18n.T("chat.tools.status.failed")
	case api.MCPNeedsAuth:
		return icToolsAuth, th.ToneWait, i18n.T("chat.tools.status.needsAuth")
	}
	return icToolsOff, th.MutedForeground, i18n.T("chat.tools.status.disabled")
}

func (u *window) mcpView(gtx C, th *ui.Theme, p *chatPane, sv sessionView, l api.ChatMCPList) D {
	t := &p.tools
	if !l.Reported {
		return toolsNote(gtx, th, i18n.T("chat.tools.mcpNone", i18n.Args{"provider": u.providerName(sv)}))
	}
	servers := mcpOrdered(l.Servers)
	if i := slices.IndexFunc(servers, func(s api.ChatMCPServer) bool { return s.Name == t.detail }); i >= 0 {
		return u.mcpDetail(gtx, th, t, servers[i], l.Actions)
	}
	t.ensureRows(len(servers))
	var lines []layout.Widget
	group := ""
	for i, s := range servers {
		if g := toolsGroup(s.Scope); g != group {
			group = g
			lines = append(lines, mcpGroupLine(th, g, s.Config))
		}
		lines = append(lines, func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return mcpRow(gtx, th, s) })
		})
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.tools.mcpTitle"), i18n.T("chat.tools.serverCount", i18n.Args{"count": len(servers)}), nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

func mcpGroupLine(th *ui.Theme, g, config string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Top: ui.Space(2), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			row := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, toolsGroupLabel(g)).Weight(font.SemiBold).Layout)}
			if config != "" {
				row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, config).Muted().Truncated().Layout))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
}

func mcpRow(gtx C, th *ui.Theme, s api.ChatMCPServer) D {
	ic, ink, state := mcpStatus(th, s)
	meta := state
	if s.Status == api.MCPConnected {
		meta = i18n.T("chat.tools.toolCount", i18n.Args{"count": len(s.Tools)})
	}
	name := th.Text(ui.TextSM, s.Name)
	if s.Status == api.MCPDisabled {
		name = name.Muted()
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
		ui.Gap(2),
		layout.Rigid(name.Layout),
		ui.Gap(toolsMetaGap),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, th.Text(ui.TextXS, meta).Muted().Truncated().Layout)
		}),
	)
}

// ensureRows keeps a clickable per row.
func (t *chatTools) ensureRows(n int) {
	if len(t.rows) < n {
		t.rows = append(t.rows, make([]ui.Clickable, n-len(t.rows))...)
	}
}

// scrolled is the panel's head, its lines scrolling under it and foot
// pinned under them.
func (t *chatTools) scrolled(gtx C, th *ui.Theme, head layout.Widget, lines []layout.Widget, foot layout.Widget) D {
	t.list.Axis = layout.Vertical
	if t.moved {
		t.list.ScrollTo(max(t.cursor-2, 0))
		t.moved = false
	}
	children := []layout.FlexChild{
		layout.Rigid(head),
		layout.Rigid(func(gtx C) D {
			return th.ThinScrollArea(&t.list).Layout(gtx, len(lines), func(gtx C, i int) D { return lines[i](gtx) })
		}),
	}
	if foot != nil {
		children = append(children, layout.Rigid(th.Separator), layout.Rigid(foot))
	}
	// the panel is as tall as its lines, up to its room
	gtx.Constraints.Min.Y = 0
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func toolsFootNote(th *ui.Theme, s string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1)}.Layout(gtx, th.Text(ui.TextXS, s).Muted().Wrapped().Layout)
	}
}

// providerName is the name of sv's provider as people know it.
func (u *window) providerName(sv sessionView) string {
	if info, ok := u.model.providers.find(sv.Kind); ok && info.name != "" {
		return info.name
	}
	return sv.Kind
}

// skillMeta is what a skill's line says after its name: why it is locked,
// how it is offered when not plainly on, where it comes from and what it
// costs.
func skillMeta(s api.ChatSkill) string {
	var parts []string
	if s.Locked != "" {
		parts = append(parts, skillLockLabel(s.Locked))
	}
	switch s.State {
	case api.SkillNameOnly:
		parts = append(parts, i18n.T("chat.tools.skill.nameOnly"))
	case api.SkillUserInvocableOnly:
		parts = append(parts, i18n.T("chat.tools.skill.userOnly"))
	}
	if s.Source != "" {
		parts = append(parts, s.Source)
	}
	if s.Tokens > 0 {
		parts = append(parts, i18n.T("chat.tools.skill.tokens", i18n.Args{"count": s.Tokens}))
	}
	return strings.Join(parts, " · ")
}

func skillLockLabel(why string) string {
	switch why {
	case "author":
		return i18n.T("chat.tools.lock.author")
	case "plugin":
		return i18n.T("chat.tools.lock.plugin")
	case "policy":
		return i18n.T("chat.tools.lock.policy")
	}
	return i18n.T("chat.tools.lock.any")
}

func (u *window) skillsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView, l api.ChatSkillList) D {
	t := &p.tools
	if !l.Reported {
		return toolsNote(gtx, th, i18n.T("chat.tools.skillsNone", i18n.Args{"provider": u.providerName(sv)}))
	}
	shown := t.shownSkills(l.Skills)
	t.ensureRows(len(shown))
	var sort layout.Widget
	if slices.ContainsFunc(l.Skills, func(s api.ChatSkill) bool { return s.Tokens > 0 }) {
		label := i18n.T("chat.tools.sortName")
		if t.byTokens {
			label = i18n.T("chat.tools.sortTokens")
		}
		b := th.Button(&t.sortBtn, label)
		b.Variant, b.Size = ui.VariantGhost, ui.SizeXS
		sort = b.Layout
	}
	lines := make([]layout.Widget, len(shown))
	for i, s := range shown {
		lines[i] = func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return skillRow(gtx, th, s) })
		}
	}
	var foot layout.Widget
	if slices.ContainsFunc(l.Skills, func(s api.ChatSkill) bool { return s.Locked == "plugin" }) {
		foot = toolsFootNote(th, i18n.T("chat.tools.pluginSkills"))
	}
	head := func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return toolsHead(gtx, th, i18n.T("chat.tools.skillsTitle"), i18n.T("chat.tools.skillCount", i18n.Args{"count": len(l.Skills)}), sort)
			}),
			layout.Rigid(func(gtx C) D {
				in := th.Input(&t.query, i18n.T("chat.tools.search"))
				in.Borderless = true
				return in.Layout(gtx)
			}),
			layout.Rigid(th.Separator),
		)
	}
	return t.scrolled(gtx, th, head, lines, foot)
}

func skillRow(gtx C, th *ui.Theme, s api.ChatSkill) D {
	ic, ink := icCheck, th.TonePass
	name := th.Text(ui.TextSM, s.Name)
	if s.State == api.SkillOff {
		ic, ink, name = icToolsOff, th.MutedForeground, name.Muted()
	}
	if s.Locked != "" {
		ink = ui.Alpha(ink, 0.55)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
		ui.Gap(2),
		layout.Rigid(name.Layout),
		ui.Gap(toolsMetaGap),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, th.Text(ui.TextXS, skillMeta(s)).Muted().Truncated().Layout)
		}),
	)
}
