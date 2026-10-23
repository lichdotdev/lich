// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// A card that reports a conversation another open card runs (a `/resume`
// typed in its TUI) is the one the window warns about, naming the holder.
func TestASessionStartOnAHeldConversationTellsTheWindow(t *testing.T) {
	t.Parallel()
	hub, rec := newProbeHub(t)
	svc := New(stubBins{holder: "holder"}, nil, hub)

	if err := svc.onSessionStart("resumer", "conv-shared", providers.Claude); err != nil {
		t.Fatalf("onSessionStart = %v, want nil", err)
	}
	waitFor(t, func() bool { _, ok := rec.payloadOf(api.SessionDuplicateEvent.Name); return ok }, "the duplicate event")
	payload, _ := rec.payloadOf(api.SessionDuplicateEvent.Name)
	got, _ := payload.(map[string]any)
	if got["id"] != "resumer" || got["heldBy"] != "holder" {
		t.Errorf("duplicate event = %v, want resumer held by holder", payload)
	}
}

func TestASessionStartOnItsOwnConversationRaisesNoWarning(t *testing.T) {
	t.Parallel()
	hub, rec := newProbeHub(t)
	svc := New(stubBins{}, nil, hub)

	if err := svc.onSessionStart("s1", "conv-own", providers.Claude); err != nil {
		t.Fatalf("onSessionStart = %v, want nil", err)
	}
	// The agent event is pushed after the check, on the same socket, so once
	// it is in a warning would have been too.
	waitFor(t, func() bool { _, ok := rec.payloadOf(api.SessionAgentEvent.Name); return ok }, "the agent event")
	if payload, ok := rec.payloadOf(api.SessionDuplicateEvent.Name); ok {
		t.Errorf("duplicate event = %v, want none for a conversation nobody else runs", payload)
	}
}

func TestASessionStartTellsTheWindowTheConversation(t *testing.T) {
	t.Parallel()
	hub, rec := newProbeHub(t)
	svc := New(stubBins{}, nil, hub)

	if err := svc.onSessionStart("s1", "conv-own", providers.Claude); err != nil {
		t.Fatalf("onSessionStart = %v, want nil", err)
	}
	waitFor(t, func() bool { _, ok := rec.payloadOf(api.SessionConversationEvent.Name); return ok }, "the conversation event")
	payload, _ := rec.payloadOf(api.SessionConversationEvent.Name)
	got, _ := payload.(map[string]any)
	if got["id"] != "s1" || got["providerSessionId"] != "conv-own" {
		t.Errorf("conversation event = %v, want s1 running conv-own", payload)
	}
}
