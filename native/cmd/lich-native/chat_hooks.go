package main

import (
	"cmp"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// hookKey names a hook in the detail: Codex's own key, or what tells
// Claude Code's apart.
func hookKey(h api.ChatHook) string {
	if h.Key != "" {
		return h.Key
	}
	return h.Event + "|" + h.Matcher + "|" + h.Source + "|" + h.Command
}

// hookActions is what the detail offers on h, of what its provider takes:
// trusting one not trusted as it is now, turning one on or off, nothing on
// an organization's.
func hookActions(h api.ChatHook, l api.ChatHookList) []string {
	var acts []string
	add := func(a string, ok bool) {
		if ok && slices.Contains(l.Actions, a) {
			acts = append(acts, a)
		}
	}
	untrusted := h.Trust == api.HookUntrusted || h.Trust == api.HookModified
	add(api.HookTrust, untrusted && !h.Managed)
	add(api.HookEnable, h.Off && !untrusted && !h.Managed)
	add(api.HookDisable, !h.Off && !h.Managed)
	return acts
}

func hookActLabel(a string) string {
	switch a {
	case api.HookTrust:
		return i18n.T("chat.hooks.act.trust")
	case api.HookEnable:
		return i18n.T("chat.plugins.act.enable")
	}
	return i18n.T("chat.plugins.act.disable")
}

// hookActs runs the /hooks panel's rows and detail, and gives toolsActs the
// rows shown and what Enter does on one.
func (u *window) hookActs(gtx C, p *chatPane, id string) (int, func(int)) {
	t := &p.tools
	t.mu.Lock()
	l := t.set.hooks
	t.mu.Unlock()
	if t.detail == "" {
		return len(l.Hooks), func(i int) { t.detail, t.focus = hookKey(l.Hooks[i]), true }
	}
	i := slices.IndexFunc(l.Hooks, func(h api.ChatHook) bool { return hookKey(h) == t.detail })
	if t.back.Clicked(gtx) || i < 0 {
		t.detail, t.focus = "", true
		return 0, noRowEnter
	}
	h := l.Hooks[i]
	for _, a := range hookActions(h, l) {
		if t.act(chatPluginAction{action: a}.key()).Clicked(gtx) {
			go func() {
				u.chatFailed(u.client.ChatHookAct(u.ctx, api.ChatHookActOptions{ID: id, Key: h.Key, Action: a, Hash: h.Hash}))
				u.fetchTools(p, id)
			}()
		}
	}
	return 0, noRowEnter
}

// hooksView is the /hooks panel: the hooks by event, or the one open.
func (u *window) hooksView(gtx C, th *ui.Theme, p *chatPane, sv sessionView) D {
	t := &p.tools
	t.mu.Lock()
	l := t.set.hooks
	t.mu.Unlock()
	if !l.Reported {
		return toolsNote(gtx, th, i18n.T("chat.hooks.none", i18n.Args{"provider": u.providerName(sv)}))
	}
	if i := slices.IndexFunc(l.Hooks, func(h api.ChatHook) bool { return hookKey(h) == t.detail }); i >= 0 {
		return t.hookDetail(gtx, th, l.Hooks[i], l)
	}
	t.ensureRows(len(l.Hooks))
	var lines []layout.Widget
	event := ""
	for i, h := range l.Hooks {
		if h.Event != event {
			event = h.Event
			lines = append(lines, hookEventLine(th, h))
		}
		lines = append(lines, func(gtx C) D {
			return pickerRow(gtx, th, &t.rows[i], i == t.cursor, func(gtx C, th *ui.Theme) D { return hookRow(gtx, th, h) })
		})
	}
	if len(l.Hooks) == 0 {
		lines = append(lines, func(gtx C) D { return toolsNote(gtx, th, i18n.T("chat.hooks.noHooks")) })
	}
	var notes []string
	if l.Blocked != "" {
		notes = append(notes, hooksBlockedNote(l.Blocked))
	}
	if len(l.Errors) > 0 {
		notes = append(notes, i18n.T("chat.permissions.skipped", i18n.Args{"files": strings.Join(l.Errors, "; ")}))
	}
	head := func(gtx C) D {
		return toolsHead(gtx, th, i18n.T("chat.hooks.title"), i18n.T("chat.plugins.part.hooks", i18n.Args{"count": len(l.Hooks)}), nil)
	}
	return t.scrolled(gtx, th, head, lines, footNotes(th, notes))
}

func hookEventLine(th *ui.Theme, h api.ChatHook) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(2), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, h.Event).Weight(font.SemiBold).Layout),
				ui.Gap(1.5),
				layout.Flexed(1, th.Text(ui.TextXS, h.Summary).Muted().Truncated().Layout),
			)
		})
	}
}

// hookState is a hook's glyph and whether it runs, in words.
func hookState(th *ui.Theme, h api.ChatHook) (layout.Widget, string) {
	ic, ink, state := icCheck, th.TonePass, i18n.T("chat.hooks.state.on")
	switch {
	case h.Trust == api.HookUntrusted || h.Trust == api.HookModified:
		ic, ink, state = icToolsAuth, th.ToneWait, i18n.T("chat.hooks.state.untrusted")
		if h.Trust == api.HookModified {
			state = i18n.T("chat.hooks.state.modified")
		}
	case h.Off:
		ic, ink, state = icToolsOff, th.MutedForeground, i18n.T("chat.hooks.state.off")
	}
	return func(gtx C) D { return ic.Layout(gtx, toolsGlyph, ink) }, state
}

// hookRow is a hook: what it shows, the tools it is for, and where it is
// configured.
func hookRow(gtx C, th *ui.Theme, h api.ChatHook) D {
	glyph, _ := hookState(th, h)
	label := cmp.Or(h.Label, h.Command)
	name := th.Text(ui.TextSM, cleanLine(label))
	if h.Off {
		name = name.Muted()
	}
	end := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, hookSourceLabel(h)).Muted().Layout)}
	return descRow(gtx, th, glyph, name.Truncated().Layout, h.Matcher, end)
}

func hookSourceLabel(h api.ChatHook) string {
	if h.Plugin != "" {
		return h.Plugin
	}
	return permSourceLabel(h.Source)
}

// hookDetail is a hook: when it runs, what it runs, where it is configured
// and whether it runs, with what can be done to it, or, for a provider that
// changes its hooks only in their files, where.
func (t *chatTools) hookDetail(gtx C, th *ui.Theme, h api.ChatHook, l api.ChatHookList) D {
	glyph, state := hookState(th, h)
	lines := []layout.Widget{
		mcpFact(th, i18n.T("chat.tools.fact.status"), func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(glyph), ui.Gap(1.5), layout.Rigid(th.Text(ui.TextSM, state).Layout))
		}),
		mcpFact(th, i18n.T("chat.hooks.fact.event"), th.Text(ui.TextSM, h.Event).Layout),
	}
	text := func(label, value string, mono bool) {
		if value == "" {
			return
		}
		l := th.Text(ui.TextSM, value).Wrapped()
		if mono {
			l = th.Text(ui.TextXS, value).Mono().Wrapped()
		}
		lines = append(lines, mcpFact(th, label, l.Layout))
	}
	text(i18n.T("chat.hooks.fact.matcher"), h.Matcher, true)
	text(i18n.T("chat.hooks.fact.condition"), h.Condition, true)
	text(i18n.T("chat.hooks.fact.type"), h.Type, false)
	text(i18n.T("chat.hooks.fact.runs"), h.Command, true)
	if h.Timeout > 0 {
		text(i18n.T("chat.hooks.fact.timeout"), i18n.T("chat.hooks.seconds", i18n.Args{"count": h.Timeout}), false)
	}
	text(i18n.T("chat.permissions.fact.source"), hookSourceLabel(h), false)
	acts := hookActions(h, l)
	var foot layout.Widget
	switch {
	case len(acts) > 0:
		foot = func(gtx C) D { return t.pluginButtons(gtx, th, hookButtons(acts), "") }
	case len(l.Actions) == 0:
		foot = toolsFootNote(th, i18n.T("chat.hooks.readOnly"))
	case h.Managed:
		foot = toolsFootNote(th, i18n.T("chat.hooks.managed"))
	}
	return t.scrolled(gtx, th, t.detailHead(th, i18n.T("chat.hooks.back"), cleanLine(cmp.Or(h.Label, h.Command))), lines, foot)
}

// hookButtons is acts as the detail's buttons, which pluginButtons draws.
func hookButtons(acts []string) []chatPluginAction {
	out := make([]chatPluginAction, len(acts))
	for i, a := range acts {
		out[i] = chatPluginAction{action: a, label: hookActLabel(a)}
	}
	return out
}

func hooksBlockedNote(why string) string {
	switch why {
	case api.HooksOffByPolicy:
		return i18n.T("chat.hooks.blocked.policy")
	case api.HooksManagedOnly:
		return i18n.T("chat.hooks.blocked.managedOnly")
	case api.HooksSafeMode:
		return i18n.T("chat.hooks.blocked.safeMode")
	case api.HooksBareMode:
		return i18n.T("chat.hooks.blocked.bareMode")
	}
	return i18n.T("chat.hooks.blocked.off")
}
