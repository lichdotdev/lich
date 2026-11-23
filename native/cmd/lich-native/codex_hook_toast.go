package main

import (
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// codexHookToastID keeps the warning and its explanation one toast.
const codexHookToastID = "codex-hook-untrusted"

// codexHookToast says Codex is skipping lich-plugin's PreToolUse hook, so its
// subagents run natively rather than as cards, with a button that explains
// how to trust the hook again. lich does not trust it itself: Codex's own
// /hooks is where the person reviews the hook's new content.
func (m *model) codexHookToast(sessionID string) ui.Toast {
	return ui.Toast{
		ID: codexHookToastID, Variant: ui.ToastWarning,
		Title:  i18n.T("chat.codexHook.title"),
		Action: &ui.ToastButton{Label: i18n.T("chat.codexHook.how"), OnClick: func() { m.showToast(m.codexHookHowToast(sessionID)) }},
	}
}

// codexHookHowToast is the explanation, in place of the warning, with the
// session to open.
func (m *model) codexHookHowToast(sessionID string) ui.Toast {
	t := ui.Toast{ID: codexHookToastID, Variant: ui.ToastWarning, Title: i18n.T("chat.codexHook.howTitle"), Description: i18n.T("chat.codexHook.howSteps")}
	if sv, ok := m.sessionViewOf(sessionID); ok && m.focusSession != nil {
		t.Action = &ui.ToastButton{Label: i18n.T("chat.codexHook.open"), OnClick: func() { m.focusSession(sv.projectID, sv.ID) }}
	}
	return t
}
