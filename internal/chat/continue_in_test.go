//go:build !windows

package chat

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/ompprofile"
	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestContinueInMovesAChat: Continue in on a session drawn as a chat ends its
// process, moves the conversation and opens the chat again on it in the target
// profile, where the terminal used to be the only side it worked from.
func TestContinueInMovesAChat(t *testing.T) {
	for _, tc := range []struct {
		kind, conv string
		create     func(string) (profilestore.Profile, error)
		write      func(r *rig, id string)
	}{
		{providers.Claude, "claude-conv", claudeprofile.Create, func(r *rig, id string) {
			r.writeClaudeTranscript(id, `{"type":"user","message":{"content":"hi"}}`)
		}},
		{providers.Codex, "codex-conv", codexprofile.Create, (*rig).writeCodexRollout},
		{providers.OMP, "omp-conv", ompprofile.Create, (*rig).writeOMPSession},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			r := newRig(t, tc.kind)
			_ = r.db.SetProviderSession("s1", tc.conv)
			tc.write(r, tc.conv)
			work, err := tc.create("Work")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			first := r.svc.running("s1")

			if _, err := r.term.ContinueIn(t.Context(), api.ContinueInOptions{ID: "s1", Profile: work.Slug}); err != nil {
				t.Fatalf("ContinueIn: %v", err)
			}
			if next := r.svc.running("s1"); next == nil || next == first {
				t.Fatal("the chat was not opened again")
			}
			if got, _ := r.db.SessionProfile("s1"); got != work.Slug {
				t.Errorf("row profile %q, want %q", got, work.Slug)
			}
			if n := r.term.XDGProfileSessions(tc.kind, work.Slug); n != 1 {
				t.Errorf("%d sessions run in %s, want the chat", n, work.Slug)
			}
			if _, lost := r.item(api.ItemNotice, "did not come back"); lost {
				t.Error("the move lost the conversation")
			}
			if r.db.SessionView("s1") != api.ViewChat {
				t.Error("the session left its chat")
			}
		})
	}
}
