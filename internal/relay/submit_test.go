package relay

import (
	"testing"
)

func TestSubmitPastesAndSendsAtAnAgentsPrompt(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)

	if err := svc.Submit("agent", "fix the checks"); err != nil {
		t.Fatalf("Submit = %v, want nil", err)
	}
	writes := term.writesTo("agent")
	if len(writes) != 2 || writes[0] != "\x1b[200~fix the checks\x1b[201~" || writes[1] != "\r" {
		t.Errorf("writes = %q, want the paste and then Enter", writes)
	}
}

func TestSubmitHandsItToTheModWhereOneIsAttached(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("modded")
	term.attachMod("modded", ackedOK)
	svc := newRelay(merging(), term, nil)

	if err := svc.Submit("modded", "resolve the conflicts"); err != nil {
		t.Fatalf("Submit = %v, want nil", err)
	}
	if !awaitPrompts(term, "modded", 1) {
		t.Fatal("modded's mod was handed nothing, want the message")
	}
	if got := term.written("modded"); got != "" {
		t.Errorf("typed at modded = %q, want nothing: its mod took it", got)
	}
}

func TestSubmitRefusesWhatCannotTakeAPrompt(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("shell")
	svc := newRelay(merging(), term, nil)

	for _, id := range []string{"shell", "parked", "missing"} {
		if err := svc.Submit(id, "rm -rf /"); err == nil {
			t.Errorf("Submit(%q) = nil, want an error", id)
		}
	}
	if got := term.written("shell"); got != "" {
		t.Errorf("typed at shell = %q, want nothing", got)
	}
}
