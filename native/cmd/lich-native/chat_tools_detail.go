package main

import (
	"slices"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// mcpDetail is one server: its state, sign-in, version, where it runs and is
// configured and its tools, then what can be done to it. Escape or Back
// goes back to the list.
func (u *window) mcpDetail(gtx C, th *ui.Theme, t *chatTools, s api.ChatMCPServer) D {
	head := func(gtx C) D {
		_, ink, state := mcpStatus(th, s)
		if s.Status == api.MCPConnected || s.Status == api.MCPDisabled {
			ink = th.MutedForeground
		}
		return toolsBackHeadIn(gtx, th, &t.back, i18n.T("chat.tools.back"), th.Text(ui.TextSM, s.Name).Weight(font.SemiBold), state, ink, nil)
	}
	lines := mcpFacts(th, s)
	if t.showTools {
		for _, tool := range s.Tools {
			lines = append(lines, mcpToolLine(th, tool))
		}
	}
	return t.scrolled(gtx, th, head, lines, nil)
}

// mcpActions is what the panel offers on s, of what its provider takes:
// signing in where it lacks one, reconnecting what is on, and turning it on
// or off, but never lich's own server.
func mcpActions(s api.ChatMCPServer, offered []string) []string {
	var acts []string
	add := func(a string, ok bool) {
		if ok && slices.Contains(offered, a) {
			acts = append(acts, a)
		}
	}
	add(api.MCPSignIn, s.Auth == api.MCPNotSignedIn || s.Status == api.MCPNeedsAuth)
	add(api.MCPReconnect, s.Status != api.MCPDisabled)
	add(api.MCPEnable, s.Status == api.MCPDisabled && !s.Fixed)
	add(api.MCPDisable, s.Status != api.MCPDisabled && !s.Fixed)
	return acts
}

func mcpActLabel(a string) string {
	switch a {
	case api.MCPSignIn:
		return i18n.T("chat.tools.act.signIn")
	case api.MCPReconnect:
		return i18n.T("chat.tools.act.reconnect")
	case api.MCPEnable:
		return i18n.T("chat.tools.act.enable")
	}
	return i18n.T("chat.tools.act.disable")
}

// mcpFoot is the actions of the server the detail shows, for the keys'
// line: View tools and turning it off as ghosts, then the rest, the one that
// mends its state filled at the end.
func (t *chatTools) mcpFoot(th *ui.Theme, l api.ChatMCPList) layout.Widget {
	i := slices.IndexFunc(l.Servers, func(s api.ChatMCPServer) bool { return s.Name == t.detail })
	if t.detail == "" || i < 0 {
		return nil
	}
	s := l.Servers[i]
	var ghosts, rest []layout.FlexChild
	button := func(c *ui.Clickable, label string, v ui.Variant) layout.FlexChild {
		b := th.Button(c, label)
		b.Variant, b.Size = v, ui.SizeSM
		return layout.Rigid(b.Layout)
	}
	if len(s.Tools) > 0 {
		label := i18n.T("chat.tools.act.viewTools")
		if t.showTools {
			label = i18n.T("chat.tools.act.hideTools")
		}
		ghosts = append(ghosts, button(&t.toolsBtn, label, ui.VariantGhost))
	}
	mends := mcpMends(s)
	for _, a := range mcpActions(s, l.Actions) {
		switch {
		case a == api.MCPDisable:
			ghosts = append(ghosts, button(t.act(a), mcpActLabel(a), ui.VariantGhost))
		case a == mends:
		default:
			rest = append(rest, button(t.act(a), mcpActLabel(a), ui.VariantSecondary))
		}
	}
	if slices.Contains(mcpActions(s, l.Actions), mends) {
		rest = append(rest, button(t.act(mends), mcpActLabel(mends), ui.VariantDefault))
	}
	return footButtons(append(ghosts, rest...))
}

// mcpMends is the action that mends s's state: signing in where it needs
// one, reconnecting a failure, turning on one that is off.
func mcpMends(s api.ChatMCPServer) string {
	switch {
	case s.Status == api.MCPNeedsAuth || s.Auth == api.MCPNotSignedIn:
		return api.MCPSignIn
	case s.Status == api.MCPFailed:
		return api.MCPReconnect
	case s.Status == api.MCPDisabled:
		return api.MCPEnable
	}
	return ""
}

// footButtons is a row of buttons for the keys' line, Space(1.5) apart.
func footButtons(buttons []layout.FlexChild) layout.Widget {
	if len(buttons) == 0 {
		return nil
	}
	var row []layout.FlexChild
	for i, b := range buttons {
		if i > 0 {
			row = append(row, ui.Gap(1.5))
		}
		row = append(row, b)
	}
	return func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...) }
}

// mcpFacts is a server's facts, a label and its value a line, the error
// under them.
func mcpFacts(th *ui.Theme, s api.ChatMCPServer) []layout.Widget {
	var lines []layout.Widget
	text := func(label, value string, mono bool) {
		if value == "" {
			return
		}
		l := th.Text(ui.TextSM, value).Truncated()
		if mono {
			l = th.Text(ui.TextXS, value).Mono().Truncated()
		}
		lines = append(lines, mcpFact(th, label, l.Layout))
	}
	switch s.Auth {
	case api.MCPSignedIn:
		text(i18n.T("chat.tools.fact.auth"), i18n.T("chat.tools.auth.signedIn"), false)
	case api.MCPNotSignedIn:
		text(i18n.T("chat.tools.fact.auth"), i18n.T("chat.tools.auth.notSignedIn"), false)
	}
	text(i18n.T("chat.tools.fact.version"), s.Version, false)
	text(i18n.T("chat.tools.fact.target"), s.Target, true)
	// a file reads in mono as the target does; the group a server falls in
	// when no file is known is a word
	if s.Config != "" {
		text(i18n.T("chat.tools.fact.config"), s.Config, true)
	} else {
		text(i18n.T("chat.tools.fact.config"), toolsGroupLabel(toolsGroup(s.Scope)), false)
	}
	text(i18n.T("chat.tools.fact.tools"), i18n.T("chat.tools.toolCount", i18n.Args{"count": len(s.Tools)}), false)
	if s.Error != "" {
		lines = append(lines, func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1)}.Layout(gtx, th.Text(ui.TextXS, s.Error).In(th.Destructive).Wrapped().Layout)
		})
	}
	return lines
}

func mcpFact(th *ui.Theme, label string, value layout.Widget) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Dp(toolsLabelW)
					gtx.Constraints.Max.X = gtx.Constraints.Min.X
					return th.Text(ui.TextSM, label).Muted().Truncated().Layout(gtx)
				}),
				layout.Flexed(1, value),
			)
		})
	}
}

func mcpToolLine(th *ui.Theme, tool api.ChatMCPTool) layout.Widget {
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, tool.Name).Mono().Layout),
				ui.Gap(2),
				layout.Flexed(1, th.Text(ui.TextXS, cleanLine(tool.Description)).Muted().Truncated().Layout),
			)
		})
	}
}
