//go:build !windows

package chat

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestSetViewRefuses every switch the session cannot make.
func TestSetViewRefuses(t *testing.T) {
	r := newRig(t, providers.OMP)
	if kinds, _ := r.svc.Kinds(t.Context(), api.None{}); !slices.Contains(kinds, providers.OMP) || slices.Contains(kinds, providers.Crush) {
		t.Errorf("Kinds = %v", kinds)
	}
	if err := r.setView("sideways"); err == nil {
		t.Error("an unknown view was accepted")
	}
	if _, err := r.svc.SetView(t.Context(), api.ChatViewOptions{ID: "missing", View: api.ViewChat}); err == nil {
		t.Error("a missing session switched")
	}
	if _, err := r.svc.History(t.Context(), api.SessionIDOptions{ID: "s1"}); err == nil {
		t.Error("History of a terminal session succeeded")
	}
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1"}); err == nil {
		t.Error("Answer with no chat succeeded")
	}
	r.term.ReportChatState("s1", stateBusy, "", "")
	if err := r.setView(api.ViewChat); err == nil {
		t.Error("a switch during the terminal's turn was accepted")
	}
	r.term.ReportChatState("s1", stateDone, "", "")

	release, _ := r.svc.claim("s1")
	if err := r.setView(api.ViewChat); err == nil {
		t.Error("a switch during another switch was accepted")
	}
	release()

	_, _ = r.db.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "crush", Label: "C", Kind: providers.Crush, NextSeq: 3})
	if _, err := r.svc.SetView(t.Context(), api.ChatViewOptions{ID: "crush", View: api.ViewChat}); err == nil {
		t.Error("a Crush session became a chat")
	}

	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Errorf("switching a chat to a chat = %v, want nil", err)
	}
	r.svc.End("s1")
	if r.svc.running("s1") != nil {
		t.Error("End left the chat running")
	}
	r.svc.End("s1")
}

// TestAutoApproveAndBirthEffort: a checkout that skips permissions answers
// yes itself where the ACP server has no flag for it, and the effort the
// session was born with is set over the protocol.
func TestAutoApproveAndBirthEffort(t *testing.T) {
	r := newRig(t, providers.OMP)
	_, _ = r.db.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.omp.skip-permissions", Value: "true"})
	_ = r.db.SetSessionEffort("s1", "low")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if !strings.Contains(r.fakeLog(), `"configId":"thinking","sessionId":"ses-new","value":"low"`) {
		t.Errorf("the birth effort was not set: %q", r.fakeLog())
	}
	r.prompt("perm")
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, "selected:once")
	for _, it := range r.history() {
		if it.Kind == api.ItemPermission {
			t.Error("an auto-approved question became an item")
		}
	}
	if opt, ok := allowOption([]api.ChatPermitOption{{ID: "r", Kind: api.PermitRejectOnce}}); ok {
		t.Errorf("allowOption picked %+v from a reject-only question", opt)
	}
}

// TestSkipPermissionsAnswersEveryAsk: a checkout that skips permissions
// answers yes to whatever its provider still asks, a flag on the argv or not:
// Claude Code launched with --dangerously-skip-permissions still asks through
// can_use_tool, default_to_no included.
func TestSkipPermissionsAnswersEveryAsk(t *testing.T) {
	for name, tc := range map[string]struct {
		provider, prompt string
	}{
		"claude":            {providers.Claude, "perm"},
		"claude default no": {providers.Claude, "perm-no"},
		"codex":             {providers.Codex, "perm"},
		"acp":               {providers.OMP, "perm"},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, tc.provider)
			_, _ = r.db.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider." + tc.provider + ".skip-permissions", Value: "true"})
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.prompt(tc.prompt)
			r.turnEnd(0)
			for _, it := range r.history() {
				if it.Kind == api.ItemPermission {
					t.Errorf("the ask became an item: %+v", it.Permission)
				}
			}
		})
	}
}

// TestShutdownStopsEveryChat ends every process on the way out.
func TestShutdownStopsEveryChat(t *testing.T) {
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	c := r.svc.running("s1")
	r.svc.Shutdown()
	select {
	case <-c.proc.Done():
	default:
		t.Fatal("Shutdown returned with a chat running")
	}
}

// TestPermissionDefaultNo: a permission is default-no only when its provider
// asks that no single key allow it, which Claude Code's default_to_no does
// and Codex's and ACP's requests have no field for.
func TestPermissionDefaultNo(t *testing.T) {
	for name, tc := range map[string]struct {
		provider string
		prompt   string
		want     bool
	}{
		"claude":            {providers.Claude, "perm", false},
		"claude default no": {providers.Claude, "perm-no", true},
		"codex":             {providers.Codex, "perm", false},
		"acp":               {providers.OMP, "perm", false},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, tc.provider)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.prompt(tc.prompt)
			if got := r.waitPermission().Permission.DefaultNo; got != tc.want {
				t.Errorf("DefaultNo = %v, want %v", got, tc.want)
			}
		})
	}
}
