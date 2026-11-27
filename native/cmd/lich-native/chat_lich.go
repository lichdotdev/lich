package main

import (
	"image"
	"strings"

	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// lichPrefixes are the ways a chat spells a tool of lich's own MCP server,
// longest first (toolline.go measures each harness's).
var lichPrefixes = []string{"mcp__lich__", "mcp__lich_", "lich_"}

var (
	icLichOpen    = icons.Lich("open")
	icLichSend    = icons.Lich("send")
	icLichWait    = icons.Lich("wait")
	icLichClose   = icons.Lich("close")
	icLichSession = icons.Lich("session")
	icLichBrowser = icons.Lich("browser")
	icLichService = icons.Lich("service")
	icLichCode    = icons.Lich("code")
)

// lichCall is how the chat draws a call of one lich tool: its family's mark,
// its verb's keys, past and running, "" for a tool that is its own name, and
// atSession for a tool whose target is a session, drawn as a link to its card.
type lichCall struct {
	mark          icons.Glyph
	past, running string
	atSession     bool
}

var lichCalls = map[string]lichCall{
	"open_session":        {icLichOpen, "chat.view.lich.openedSession", "chat.view.lich.openingSession", false},
	"send_to_session":     {icLichSend, "chat.view.lich.sentTo", "chat.view.lich.sendingTo", true},
	"reply_to_session":    {icLichSend, "chat.view.lich.replied", "chat.view.lich.replying", false},
	"handoff_session":     {icLichSend, "", "", false},
	"wait_for_answer":     {icLichWait, "chat.view.lich.waited", "chat.view.lich.waiting", false},
	"schedule_prompt":     {icLichWait, "", "", false},
	"close_session":       {icLichClose, "chat.view.lich.closed", "chat.view.lich.closing", true},
	"close_project":       {icLichClose, "", "", false},
	"rename_session":      {icLichSession, "chat.view.lich.renamed", "chat.view.lich.renaming", true},
	"control_session":     {icLichSession, "chat.view.lich.adjusted", "chat.view.lich.adjusting", true},
	"file_session":        {icLichSession, "chat.view.lich.filed", "chat.view.lich.filing", true},
	"list_sessions":       {icLichSession, "", "", false},
	"list_worktrees":      {icLichSession, "", "", false},
	"list_folders":        {icLichSession, "", "", false},
	"rename_folder":       {icLichSession, "", "", false},
	"color_folder":        {icLichSession, "", "", false},
	"start_service":       {icLichService, "chat.view.lich.started", "chat.view.lich.starting", false},
	"stop_service":        {icLichService, "chat.view.lich.stopped", "chat.view.lich.stopping", false},
	"restart_service":     {icLichService, "chat.view.lich.restarted", "chat.view.lich.restarting", false},
	"force_stop_service":  {icLichService, "", "", false},
	"start_all_services":  {icLichService, "", "", false},
	"stop_all_services":   {icLichService, "", "", false},
	"create_service":      {icLichService, "", "", false},
	"edit_service":        {icLichService, "", "", false},
	"delete_service":      {icLichService, "", "", false},
	"list_services":       {icLichService, "", "", false},
	"read_service_output": {icLichService, "", "", false},
	"browser_open":        {icLichBrowser, "chat.view.lich.openedPage", "chat.view.lich.openingPage", false},
	"browser_navigate":    {icLichBrowser, "chat.view.lich.wentTo", "chat.view.lich.goingTo", false},
	"show_code":           {icLichCode, "chat.view.lich.showed", "chat.view.lich.showing", false},
	"mark_thread_fixed":   {icLichCode, "", "", false},
	"read_check_log":      {icLichCode, "", "", false},
}

// lichTool is the lich tool a chat's tool name calls, false for another's.
// Every browser tool is one family, so one not listed still wears its mark.
func lichTool(name string) (string, lichCall, bool) {
	for _, prefix := range lichPrefixes {
		tool, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		if call, ok := lichCalls[tool]; ok {
			return tool, call, true
		}
		if strings.HasPrefix(tool, "browser_") {
			return tool, lichCall{mark: icLichBrowser}, true
		}
	}
	return "", lichCall{}, false
}

// lichVerb is a lich tool's row verb, in the running form while it runs; a
// tool with no verb of its own is its name.
func lichVerb(tool string, call lichCall, running bool) string {
	switch {
	case call.past == "":
		return tool
	case running:
		return i18n.T(call.running)
	}
	return i18n.T(call.past)
}

// lichOutcome draws a lich call whose target is an open session with that
// session's pill in place of the name the agent wrote, a click away from its
// card. A failed call keeps its row, which says why.
func (u *window) lichOutcome(p *chatPane, sv sessionView) func(backend.ChatItem) *chatOpened {
	return func(it backend.ChatItem) *chatOpened {
		t := it.Tool
		if t == nil || t.State == api.ToolFailed || t.Title == "" {
			return nil
		}
		if _, call, ok := lichTool(t.Name); !ok || !call.atSession {
			return nil
		}
		id, label, ok := u.model.sessionCalled(sv.projectID, t.Title)
		if !ok {
			return nil
		}
		th := u.theme
		return &chatOpened{target: func(gtx C) D {
			// The pill's own inset would push its name past the column the
			// other calls' targets start at.
			defer op.Offset(image.Pt(-gtx.Dp(chatPillInset), 0)).Push(gtx.Ops).Pop()
			return u.openedPill(gtx, th, p.note("lich:"+it.ID), id, th.Text(ui.TextXS, label).Mono(), th.MutedForeground)
		}}
	}
}

// chatOutcome is how a call was opened as more than a row: a subagent lich
// made a card of, else a lich call at a session.
func (u *window) chatOutcome(p *chatPane, sv sessionView) (outcome func(backend.ChatItem) *chatOpened, vetoed func(backend.ChatItem) bool) {
	veto, lich := u.vetoOutcome(p, sv), u.lichOutcome(p, sv)
	outcome = func(it backend.ChatItem) *chatOpened {
		if o := veto(it); o != nil {
			return o
		}
		return lich(it)
	}
	return outcome, func(it backend.ChatItem) bool { return veto(it) != nil }
}
