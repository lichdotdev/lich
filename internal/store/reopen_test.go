package store

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

// TestReopenWorktreeSessionTakesTheNewestParkedRow pins which row a resume
// consumes when a worktree has been parked more than once — a keep-the-worktree
// close after an earlier resume leaves both rows behind. The newest is the
// conversation the user was actually in; resuming the older one would continue a
// transcript they left turns ago, and it is the one row the query orders for.
func TestReopenWorktreeSessionTakesTheNewestParkedRow(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "base", Label: "Session 1", Kind: "claude", NextSeq: 2})

	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt1", Label: "older", Kind: "claude", Path: "/wt/foo", NextSeq: 3})
	_ = svc.SetProviderSession("wt1", "conv-older")
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "wt1", ActiveID: "base"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt2", Label: "newer", Kind: "claude", Path: "/wt/foo", NextSeq: 4})
	_ = svc.SetProviderSession("wt2", "conv-newer")
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "wt2", ActiveID: "base"})

	restored, err := svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/foo", NewSessionID: "wt3"})
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenWorktreeSession = nil, want the parked session")
	}
	if restored.Label != "newer" || restored.ProviderSessionID != "conv-newer" {
		t.Errorf("restored = %+v, want the newest park (newer/conv-newer)", restored)
	}

	// Only one park is consumed: the older row is still there for its own
	// resume, and the resumed one is not duplicated.
	var parked int
	if err := svc.db.QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE path = ? AND is_open = 0`, "/wt/foo",
	).Scan(&parked); err != nil {
		t.Fatalf("count parked: %v", err)
	}
	if parked != 1 {
		t.Errorf("parked rows after resume = %d, want the older one alone", parked)
	}
}

// TestReopenHandsBackTheCardThatHoldsTheConversation: a parked row whose
// conversation an open session already runs is a stale copy of that card (a
// `/resume` typed in another card, then closed), and reopening it would put two
// cards on one transcript. Both doors answer with the open card instead, so the
// window focuses it, and leave the parked copy where it was.
func TestReopenHandsBackTheCardThatHoldsTheConversation(t *testing.T) {
	doors := map[string]func(svc *Service) (*Session, error){
		"by id": func(svc *Service) (*Session, error) {
			return svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "copy", NewSessionID: "fresh"})
		},
		"by worktree": func(svc *Service) (*Session, error) {
			return svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/foo", NewSessionID: "fresh"})
		},
	}
	for name, reopen := range doors {
		t.Run(name, func(t *testing.T) {
			svc := newTestStore(t)
			_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
			_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "holder", Label: "auth-refactor", Kind: "claude", Path: "/wt/foo", NextSeq: 2})
			_ = svc.SetProviderSession("holder", "conv-shared")
			_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "copy", Label: "Session 3", Kind: "claude", Path: "/wt/foo", NextSeq: 3})
			_ = svc.SetProviderSession("copy", "conv-shared")
			_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "copy", ActiveID: "holder"})

			restored, err := reopen(svc)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if restored == nil || restored.ID != "holder" {
				t.Fatalf("reopen = %+v, want the open card holding the conversation", restored)
			}
			var open, parked int
			if err := svc.db.QueryRow(
				`SELECT COUNT(*) FILTER (WHERE is_open = 1), COUNT(*) FILTER (WHERE is_open = 0)
				   FROM sessions WHERE provider_session_id = ?`, "conv-shared",
			).Scan(&open, &parked); err != nil {
				t.Fatalf("count rows: %v", err)
			}
			if open != 1 || parked != 1 {
				t.Errorf("rows on the conversation = %d open, %d parked; want the holder open and the copy still parked", open, parked)
			}
		})
	}
}

// TestConversationHolderNamesAnotherOpenCard: a session reporting a
// conversation is told which other card already runs it, never itself, and
// never a parked row, which runs nothing.
func TestConversationHolderNamesAnotherOpenCard(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "holder", Label: "auth-refactor", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "resumer", Label: "Claude 4", Kind: "claude", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "parked", Label: "old", Kind: "claude", NextSeq: 4})
	_ = svc.SetProviderSession("holder", "conv-shared")
	_ = svc.SetProviderSession("resumer", "conv-shared")
	_ = svc.SetProviderSession("parked", "conv-parked")
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "parked", ActiveID: "holder"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "fresh", Label: "Claude 5", Kind: "claude", NextSeq: 5})
	_ = svc.SetProviderSession("fresh", "conv-parked")

	cases := []struct{ conversation, asking, want string }{
		{"conv-shared", "resumer", "holder"},
		{"conv-shared", "holder", "resumer"},
		{"conv-parked", "fresh", ""},
		{"conv-nobody", "fresh", ""},
	}
	for _, c := range cases {
		got, err := svc.ConversationHolder(c.conversation, c.asking)
		if err != nil {
			t.Fatalf("ConversationHolder(%q, %q): %v", c.conversation, c.asking, err)
		}
		if got != c.want {
			t.Errorf("ConversationHolder(%q, %q) = %q, want %q", c.conversation, c.asking, got, c.want)
		}
	}
}
