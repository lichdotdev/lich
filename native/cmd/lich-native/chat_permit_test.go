package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// permitShapes are a permission's answers as each chat path hands them over:
// Claude Code's by kind alone (claude.go), Codex's by its decisions
// (codex_question.go), and ACP's in the agent's words, an always first and a
// reject_always last, as opencode and oh-my-pi send them (acp.go).
var permitShapes = map[string]struct {
	options []api.ChatPermitOption
	allow   string
	reject  string
}{
	"claude": {[]api.ChatPermitOption{
		{ID: api.PermitAllowOnce, Kind: api.PermitAllowOnce},
		{ID: api.PermitAllowAlways, Kind: api.PermitAllowAlways},
		{ID: api.PermitRejectOnce, Kind: api.PermitRejectOnce},
	}, api.PermitAllowOnce, api.PermitRejectOnce},
	"codex": {[]api.ChatPermitOption{
		{ID: "accept", Kind: api.PermitAllowOnce},
		{ID: "acceptForSession", Kind: api.PermitAllowAlways},
		{ID: "decline", Kind: api.PermitRejectOnce},
	}, "accept", "decline"},
	"acp": {[]api.ChatPermitOption{
		{ID: "always", Label: "Always allow", Kind: api.PermitAllowAlways},
		{ID: "once", Label: "Allow once", Kind: api.PermitAllowOnce},
		{ID: "reject", Label: "Reject", Kind: api.PermitRejectOnce},
		{ID: "never", Label: "Never", Kind: "reject_always"},
	}, "once", "reject"},
}

// noAnswer fails when anything answered the permission.
func noAnswer(t *testing.T, f *chatBackend, what string) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	if n := f.callsOf(api.ChatAnswer.Name); n != 0 {
		t.Errorf("%s answered the permission (%d calls)", what, n)
	}
}

// Enter answers nothing, on the box and on an empty composer alike.
func TestEnterNeverAnswersAPermission(t *testing.T) {
	for name, s := range permitShapes {
		t.Run(name, func(t *testing.T) {
			u, r, f := permitFrames(t, backend.ChatPermission{Options: s.options})
			r.press(key.NameReturn)
			r.router.Source().Execute(key.FocusCmd{Tag: &u.chats.pane("s1").editor})
			r.frame()
			r.press(key.NameReturn)
			noAnswer(t, f, "Enter")
		})
	}
}

// Digit n answers with the provider's nth option, always included, and each
// button carries its digit.
func TestDigitsAnswerTheirOption(t *testing.T) {
	for name, s := range permitShapes {
		for i, o := range s.options {
			t.Run(name+"/"+o.ID, func(t *testing.T) {
				_, r, f := permitFrames(t, backend.ChatPermission{Options: s.options})
				if !r.shows(strconv.Itoa(i + 1)) {
					t.Errorf("no keycap %d:\n%s", i+1, r.drawn())
				}
				r.press(key.Name(strconv.Itoa(i + 1)))
				if body := f.waitCall(t, api.ChatAnswer.Name); !strings.Contains(body, `"optionId":"`+o.ID+`"`) {
					t.Errorf("%d answered %s, want %s", i+1, body, o.ID)
				}
			})
		}
	}
}

// Esc answers with the first one-time reject and says so in its button
// (contract changed: Esc's cap stands in the reject, no "denies" legend).
func TestEscAnswersTheFirstOneTimeReject(t *testing.T) {
	for name, s := range permitShapes {
		t.Run(name, func(t *testing.T) {
			_, r, f := permitFrames(t, backend.ChatPermission{Options: s.options})
			if !r.shows("Esc") || r.shows("denies") {
				t.Errorf("no Esc hint:\n%s", r.drawn())
			}
			writeShot(t, r, "chat-permit-"+name+".png")
			r.press(key.NameEscape)
			if body := f.waitCall(t, api.ChatAnswer.Name); !strings.Contains(body, `"optionId":"`+s.reject+`"`) {
				t.Errorf("Esc answered %s, want %s", body, s.reject)
			}
		})
	}
}

// With no one-time reject, Esc answers nothing and no hint promises it:
// a reject_always is never taken for one.
func TestEscWithNoOneTimeRejectAnswersNothing(t *testing.T) {
	_, r, f := permitFrames(t, backend.ChatPermission{Options: []api.ChatPermitOption{
		{ID: "once", Label: "Allow once", Kind: api.PermitAllowOnce},
		{ID: "never", Label: "Never", Kind: "reject_always"},
	}})
	if r.shows("denies") {
		t.Errorf("an Esc hint with nothing for Esc to take:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	noAnswer(t, f, "Esc")
}

// The suggestion is the first one-time allow, never an always; on a
// default-no ask it is the first one-time reject.
func TestThePermissionSuggestion(t *testing.T) {
	for name, s := range permitShapes {
		if got := permitSuggested(&backend.ChatPermission{Options: s.options}); got != s.allow {
			t.Errorf("%s: suggested %q, want %q", name, got, s.allow)
		}
		if got := permitSuggested(&backend.ChatPermission{Options: s.options, DefaultNo: true}); got != s.reject {
			t.Errorf("%s default no: suggested %q, want %q", name, got, s.reject)
		}
	}
	if got := permitSuggested(&backend.ChatPermission{Options: []api.ChatPermitOption{{ID: "a", Kind: api.PermitAllowAlways}}}); got != "" {
		t.Errorf("suggested the always %q", got)
	}
}

// A default-no ask takes no digit, as no single key may allow it, and Esc
// still rejects it.
func TestADefaultNoPermissionTakesNoDigit(t *testing.T) {
	for name, s := range permitShapes {
		t.Run(name, func(t *testing.T) {
			_, r, f := permitFrames(t, backend.ChatPermission{Options: s.options, DefaultNo: true})
			writeShot(t, r, "chat-permit-"+name+"-default-no.png")
			for i := range s.options {
				r.press(key.Name(strconv.Itoa(i + 1)))
			}
			noAnswer(t, f, "a digit")
			r.press(key.NameEscape)
			if body := f.waitCall(t, api.ChatAnswer.Name); !strings.Contains(body, `"optionId":"`+s.reject+`"`) {
				t.Errorf("Esc answered %s, want %s", body, s.reject)
			}
		})
	}
}

// A permission coming while a message is being typed leaves the composer the
// keys, so its digits stay text; one coming on an empty composer takes them,
// and a click back on the composer gives them back.
func TestAPermissionLeavesADraftItsKeys(t *testing.T) {
	u, r, f := chatFrames(t, api.ViewChat)
	p := u.chats.pane("s1")
	r.router.Source().Execute(key.FocusCmd{Tag: &p.editor})
	typeIn(r, p, "1. use cents")
	perm := backend.ChatPermission{Tool: backend.ChatToolCall{Name: "Edit", State: api.ToolPending}, Options: permitShapes["claude"].options}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "p1", Kind: api.ItemPermission, Turn: 1, Permission: &perm}})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.frames(2)
	if !r.router.Source().Focused(&p.editor) {
		t.Fatal("the permission took the keys from a draft")
	}
	r.press("1")
	noAnswer(t, f, "a digit typed in the composer")

	u, r, f = permitFrames(t, backend.ChatPermission{Options: permitShapes["claude"].options})
	p = u.chats.pane("s1")
	if !r.router.Source().Focused(&p.permit) {
		t.Fatal("the permission did not take the keys from an empty composer")
	}
	r.router.Source().Execute(key.FocusCmd{Tag: &p.editor})
	r.frames(2)
	if !r.router.Source().Focused(&p.editor) {
		t.Fatal("the permission took the keys back from the composer")
	}
	r.press("1")
	noAnswer(t, f, "a digit typed in the composer")
}
