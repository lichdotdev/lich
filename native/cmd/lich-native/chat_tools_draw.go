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

const (
	// toolsMetaGap is the gap, in spacing steps, before a row's muted end.
	toolsMetaGap = 3
	// toolsNameShare is how much of a row its name may take.
	toolsNameShare = 0.6
)

var (
	icToolsPending = icons.Lucide("circle-dashed")
	icToolsFailed  = icons.Lucide("x")
	icToolsAuth    = icons.Lucide("circle-alert")
	icToolsOff     = icons.Lucide("circle")
	icToolsBack    = icons.Lucide("chevron-left")
	icToolsClose   = icons.Lucide("x")
	toolsMaxH      = unit.Dp(460)
	toolsLabelW    = ui.Space(32)
	toolsGlyph     = ui.Space(4)
	toolsHeadH     = ui.Space(10)
	// toolsCloseRoom is the head's end the close stands in: its SizeIconXS
	// square between two Space(1).
	toolsCloseRoom  = ui.Space(8)
	toolsGroupOrder = []string{"user", "project", "local", "claudeai", "other"}
)

// chatToolsPanel draws the open panel over the composer's box, as wide as it
// and a gap above it, deferred so it stands over the conversation, no
// taller than room: the slash list's place.
func (u *window) chatToolsPanel(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool, room int) {
	t := &p.tools
	if !t.opened() {
		return
	}
	u.pollTools(gtx, p, sv.ID)
	g := gtx
	lift := p.panelLift()
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, max(min(gtx.Dp(toolsMaxH), room-lift), 0))}
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		return withKeys(gtx, th, t, u.toolsFootAct(th, p, sv), func(gtx C) D {
			return u.toolsView(gtx, th, p, sv, working)
		})
	})
	call := macro.Stop()
	m := op.Record(gtx.Ops)
	op.Offset(image.Pt(0, -d.Size.Y-gtx.Dp(ui.Space(2))-lift)).Add(gtx.Ops)
	call.Add(gtx.Ops)
	t.watchOutside(gtx, d.Size)
	op.Defer(gtx.Ops, m.Stop())
}

// toolsFootAct is the action the open view puts at the end of the keys' line:
// a running shell's Stop, nothing elsewhere.
func (u *window) toolsFootAct(th *ui.Theme, p *chatPane, sv sessionView) layout.Widget {
	if p.tools.kind == chatShellsPanel {
		return u.shellStopAct(th, p, sv.ID)
	}
	return nil
}

// toolsView is the open panel's screen.
func (u *window) toolsView(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	t := &p.tools
	t.mu.Lock()
	mcp, skills, plugins, loaded := t.mcp, t.skills, t.plugins, t.loaded
	memory, agents := t.mem.list, t.mem.agents
	t.mu.Unlock()
	switch {
	case t.kind == chatPlanPanel:
		return u.planView(gtx, th, p, sv.ID, working)
	case t.kind == chatShellsPanel:
		return u.shellsView(gtx, th, p, sv)
	case t.kind == chatChildrenPanel:
		return u.childrenView(gtx, th, p, sv)
	case t.kind == chatWorkflowsPanel:
		return u.workflowsView(gtx, th, p, sv)
	case !loaded:
		return toolsNote(gtx, th, i18n.T("chat.tools.loading"))
	case t.kind == chatMCPCommand:
		return u.mcpView(gtx, th, p, sv, mcp)
	case t.kind == chatPluginCommand:
		return u.pluginsView(gtx, th, p, sv, plugins)
	case t.kind == chatPermissionsCommand:
		return u.permissionsView(gtx, th, p, sv)
	case t.kind == chatHooksCommand:
		return u.hooksView(gtx, th, p, sv)
	case t.kind == chatMemoryCommand:
		return u.memoryView(gtx, th, p, sv, memory)
	case t.kind == chatAgentsCommand:
		return u.agentsView(gtx, th, p, sv, agents)
	}
	return u.skillsView(gtx, th, p, sv, skills)
}

// toolsNote is the panel's empty state: a list with nothing in it, or a
// provider that reports nothing for the command.
func toolsNote(gtx C, th *ui.Theme, s string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	e := th.InlineEmptyState(s, "", nil)
	e.Pad = ui.Space(3)
	return e.Layout(gtx)
}

// toolsHead is the panel's title, its count muted beside it and an optional
// control at the end.
func toolsHead(gtx C, th *ui.Theme, title, count string, end layout.Widget) D {
	return toolsHeadRow(gtx, ui.Space(2), []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextSM, title).Weight(font.SemiBold).Layout),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextXS, count).Muted().Truncated().Layout),
	}, end)
}

// toolsBackHead is a detail's head: Back to the list, a ghost SizeXS button
// with a chevron, in the title's place, then the title, sub muted beside it
// and an optional control at the end.
func toolsBackHead(gtx C, th *ui.Theme, back *ui.Clickable, label string, title ui.LabelStyle, sub string, end layout.Widget) D {
	b := th.Button(back, label)
	b.Icon, b.Variant, b.Size = icToolsBack, ui.VariantGhost, ui.SizeXS
	return toolsHeadRow(gtx, ui.Space(1), []layout.FlexChild{
		layout.Rigid(b.Layout),
		ui.Gap(2),
		layout.Rigid(title.Truncated().Layout),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextXS, sub).Muted().Truncated().Layout),
	}, end)
}

// toolsName is a row's name, cut at toolsNameShare of the row so a long one
// leaves its facts in view.
func toolsName(name layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Max.X = int(float32(gtx.Constraints.Max.X) * toolsNameShare)
		return name(gtx)
	}
}

// toolsHeadRow is a head's line, toolsHeadH tall, its control at the end.
func toolsHeadRow(gtx C, left unit.Dp, row []layout.FlexChild, end layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	if end != nil {
		row = append(row, layout.Rigid(end))
	}
	return layout.Inset{Left: left, Right: toolsCloseRoom}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = gtx.Dp(toolsHeadH)
		return layout.W.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	})
}

// toolsGroupLine is a group's label over its rows, uppercase as the
// palette's, and its file muted beside it.
func toolsGroupLine(th *ui.Theme, label, file string) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(2.5), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			row := []layout.FlexChild{layout.Rigid(th.Text(ui.Text2XS, strings.ToUpper(label)).Muted().Weight(font.SemiBold).Leading(text2XSLeading).Layout)}
			if file != "" {
				row = append(row, ui.Gap(1.5), layout.Flexed(1, th.Text(ui.Text2XS, file).Muted().Truncated().Leading(text2XSLeading).Layout))
			}
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, row...)
		})
	}
}

// toolsKeysLine is the panel's foot: Escape's hint, since the panel never
// holds the focus that would show it, at the right, or at the left with act
// at the right.
func toolsKeysLine(gtx C, hint, act layout.Widget) D {
	fill := layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} })
	row := []layout.FlexChild{fill, layout.Rigid(hint)}
	if act != nil {
		row = []layout.FlexChild{layout.Rigid(hint), fill, layout.Rigid(act)}
	}
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(3), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// withKeys is view over the keys' line, set apart by space alone: Escape's
// hint, and act at the line's end where the view has one. The panel's close
// stands at the head's end, over the view, so every head keeps its room.
func withKeys(gtx C, th *ui.Theme, t *chatTools, act layout.Widget, view layout.Widget) D {
	m := op.Record(gtx.Ops)
	keys := toolsKeysLine(gtx, th.KeyHint("Esc", toolsEscHint(t)).Layout, act)
	keysCall := m.Stop()
	g := gtx
	g.Constraints.Max.Y = max(gtx.Constraints.Max.Y-keys.Size.Y, 0)
	g.Constraints.Min.Y = 0
	d := view(g)
	toolsClose(gtx, th, t, max(d.Size.X, keys.Size.X))
	defer op.Offset(image.Pt(0, d.Size.Y)).Push(gtx.Ops).Pop()
	keysCall.Add(gtx.Ops)
	return D{Size: image.Pt(max(d.Size.X, keys.Size.X), d.Size.Y+keys.Size.Y)}
}

// toolsClose is the panel's ghost ×, centred on the head's line at its end.
func toolsClose(gtx C, th *ui.Theme, t *chatTools, width int) {
	b := th.Button(&t.close, "")
	b.Icon, b.Variant, b.Size = icToolsClose, ui.VariantGhost, ui.SizeIconXS
	m := op.Record(gtx.Ops)
	d := b.Layout(gtx)
	call := m.Stop()
	at := image.Pt(width-d.Size.X-gtx.Dp(ui.Space(1)), (gtx.Dp(toolsHeadH)-d.Size.Y)/2)
	defer op.Offset(at).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// toolsEscHint is what Escape does: back from a detail, else shut the panel.
func toolsEscHint(t *chatTools) string {
	if t.detail != "" {
		return i18n.T("chat.tools.keys.back")
	}
	return i18n.T("chat.tools.keys.close")
}

// toolsPick is a panel's row, its Enter key cap at the end while the cursor
// is on it: the key stands where it acts.
func toolsPick(gtx C, th *ui.Theme, click *ui.Clickable, cursor bool, content func(gtx C, th *ui.Theme) D) D {
	return pickerRow(gtx, th, click, cursor, func(gtx C, th *ui.Theme) D {
		if !cursor {
			return content(gtx, th)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return content(gtx, th) }),
			ui.Gap(2),
			layout.Rigid(th.KeyCap("↵").Layout),
		)
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
			lines = append(lines, toolsGroupLine(th, toolsGroupLabel(g), s.Config))
		}
		lines = append(lines, func(gtx C) D {
			return toolsPick(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return mcpRow(gtx, th, s) })
		})
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.tools.mcpTitle"), i18n.T("chat.tools.serverCount", i18n.Args{"count": len(servers)}), nil)
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

func mcpRow(gtx C, th *ui.Theme, s api.ChatMCPServer) D {
	ic, ink, state := mcpStatus(th, s)
	meta := state
	if s.Status == api.MCPConnected {
		meta = i18n.T("chat.tools.toolCount", i18n.Args{"count": len(s.Tools)})
	}
	name := th.Text(ui.TextSM, s.Name).Weight(font.Medium)
	if s.Status == api.MCPDisabled {
		name = name.Muted()
	}
	desc := th.Text(ui.TextXS, "").Muted()
	if s.Status == api.MCPFailed && s.Error != "" {
		desc = th.Text(ui.TextXS, cleanLine(s.Error)).In(th.Destructive)
	}
	return toolsRow(gtx, th, ic, ink, name, desc, meta)
}

// toolsRow is a panel's row on one line: its state glyph, the name, what it
// is muted on the rest of the line and its facts at the end.
func toolsRow(gtx C, th *ui.Theme, ic *icons.Icon, ink color.NRGBA, name, desc ui.LabelStyle, meta string) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }),
		ui.Gap(2),
		layout.Rigid(toolsName(name.Truncated().Layout)),
		ui.Gap(2),
		layout.Flexed(1, desc.Truncated().Layout),
		ui.Gap(toolsMetaGap),
		layout.Rigid(th.Text(ui.TextXS, meta).Muted().Layout),
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
			return toolsPick(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return skillRow(gtx, th, s) })
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
				in.Bare, in.Height = true, pickerRowH
				return in.Layout(gtx)
			}),
			layout.Rigid(th.Separator),
		)
	}
	return t.scrolled(gtx, th, head, lines, foot)
}

func skillRow(gtx C, th *ui.Theme, s api.ChatSkill) D {
	ic, ink := icCheck, th.TonePass
	name := th.Text(ui.TextSM, s.Name).Weight(font.Medium)
	if s.State == api.SkillOff {
		ic, ink, name = icToolsOff, th.MutedForeground, name.Muted()
	}
	if s.Locked != "" {
		ink = ui.Alpha(ink, 0.55)
	}
	return toolsRow(gtx, th, ic, ink, name, th.Text(ui.TextXS, cleanLine(s.Description)).Muted(), skillMeta(s))
}
