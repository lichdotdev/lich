package main

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// codexHookToast says Codex is skipping lich-plugin's PreToolUse hook, so its
// subagents run natively rather than as cards, with a button to the session's
// /hooks panel. lich does not trust the hook itself: the panel is where the
// person reviews its new content and trusts it.
func (m *model) codexHookToast(sessionID string) ui.Toast {
	t := ui.Toast{ID: "codex-hook-untrusted", Variant: ui.ToastWarning, Title: i18n.T("chat.codexHook.title"), Description: i18n.T("chat.codexHook.steps")}
	if sv, ok := m.sessionViewOf(sessionID); ok && m.reviewHooks != nil {
		t.Action = &ui.ToastButton{Label: i18n.T("chat.codexHook.review"), OnClick: func() { m.reviewHooks(sv.projectID, sv.ID) }}
	}
	return t
}

// reviewHooks brings session id up with its /hooks panel open. A session on
// its terminal is switched to its chat first, the only view that has the
// panel, which opens once the chat is drawn (enterChat).
func (u *window) reviewHooks(projectID, id string) {
	u.focusSession(projectID, id)
	sv, ok := u.model.sessionViewOf(id)
	if !ok {
		return
	}
	if chatViewOf(sv) == api.ViewChat {
		u.openTools(u.chats.pane(id), id, chatHooksCommand)
		return
	}
	u.hooksOnChat = id
	u.switchView(id, api.ViewChat)
}
