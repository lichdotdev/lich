package store

import "testing"

// TestReopenWorktreeSessionTakesTheNewestParkedRow pins which row a resume
// consumes when a worktree has been parked more than once — a keep-the-worktree
// close after an earlier resume leaves both rows behind. The newest is the
// conversation the user was actually in; resuming the older one would continue a
// transcript they left turns ago, and it is the one row the query orders for.
func TestReopenWorktreeSessionTakesTheNewestParkedRow(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")

	_ = svc.AddSession("p1", "wt1", "older", "claude", "/wt/foo", 3, "")
	_ = svc.SetProviderSession("wt1", "conv-older")
	_ = svc.CloseSession("p1", "wt1", "base")
	_ = svc.AddSession("p1", "wt2", "newer", "claude", "/wt/foo", 4, "")
	_ = svc.SetProviderSession("wt2", "conv-newer")
	_ = svc.CloseSession("p1", "wt2", "base")

	restored, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt3")
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
		"by id":       func(svc *Service) (*Session, error) { return svc.ReopenSession("copy", "fresh") },
		"by worktree": func(svc *Service) (*Session, error) { return svc.ReopenWorktreeSession("p1", "/wt/foo", "fresh") },
	}
	for name, reopen := range doors {
		t.Run(name, func(t *testing.T) {
			svc := newTestStore(t)
			_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
			_ = svc.AddSession("p1", "holder", "auth-refactor", "claude", "/wt/foo", 2, "")
			_ = svc.SetProviderSession("holder", "conv-shared")
			_ = svc.AddSession("p1", "copy", "Session 3", "claude", "/wt/foo", 3, "")
			_ = svc.SetProviderSession("copy", "conv-shared")
			_ = svc.CloseSession("p1", "copy", "holder")

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
