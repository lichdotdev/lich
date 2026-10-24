package main

import (
	"context"
	"log"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// spawnVerdict is what a session's first view does.
type spawnVerdict uint8

const (
	spawnPlain  spawnVerdict = iota // nothing in the way
	spawnAsk                        // a conversation is still there: raise the prompt
	spawnResume                     // the provider is set to resume it
	spawnFresh                      // the conversation is gone: spawn without it
	spawnPark                       // the checkout is gone: nothing to spawn into
)

// Restore choices, the values of restoreKey.
const (
	restoreAsk    = "ask"
	restoreResume = "resume"
	restoreFresh  = "fresh"
)

// resumableKinds are the providers whose resume lich wires.
var resumableKinds = []string{"claude", "codex", "antigravity", "omp", "opencode", "crush", "cursor", "kiro"}

// spawnProbe is the backend reads the gate asks, *backend.Client in the
// window and a fake in tests.
type spawnProbe interface {
	Live(ctx context.Context, id string) (bool, error)
	WorkdirMissing(ctx context.Context, cwd string) (bool, error)
	ResumeAvailable(ctx context.Context, kind, providerSessionID, cwd string) (bool, error)
	GetSetting(ctx context.Context, key, scope string) (string, error)
	SetSetting(ctx context.Context, key, scope, value string) error
}

// restoreKey is the global setting holding what a restored card of provider
// kind does when its conversation is still there.
func restoreKey(kind string) string { return "provider." + kind + ".on-restore" }

// restoreChoice reads a stored value: anything but the two answers is ask.
func restoreChoice(value string) string {
	if value == restoreResume || value == restoreFresh {
		return value
	}
	return restoreAsk
}

// resumable reports whether s carries a conversation its spawn should ask
// about. Shell sessions never do, whatever id a hand-run CLI left on the row.
func resumable(s backend.Session) bool {
	return s.ProviderSessionID != "" && slices.Contains(resumableKinds, s.Kind)
}

// spawnDecision settles the checkout first: a session with no directory has
// nothing to resume either. The checks fail safe in opposite directions: a
// workdir check that errors spawns (the PTY reports its own failure), a resume
// or choice read that errors asks (losing a live conversation to an answer
// nobody gave costs more).
func spawnDecision(ctx context.Context, probe spawnProbe, cwd string, s backend.Session) spawnVerdict {
	missing, err := probe.WorkdirMissing(ctx, cwd)
	logGateErr("workdir check", err)
	if missing && err == nil {
		return spawnPark
	}
	if !resumable(s) {
		return spawnPlain
	}
	available, err := probe.ResumeAvailable(ctx, s.Kind, s.ProviderSessionID, cwd)
	logGateErr("resume check", err)
	if !available && err == nil {
		return spawnFresh
	}
	stored, err := probe.GetSetting(ctx, restoreKey(s.Kind), "")
	logGateErr("restore choice", err)
	switch {
	case err != nil:
		return spawnAsk
	case restoreChoice(stored) == restoreResume:
		return spawnResume
	case restoreChoice(stored) == restoreFresh:
		return spawnPlain
	}
	return spawnAsk
}

func logGateErr(what string, err error) {
	if err != nil {
		log.Printf("spawn gate: %s: %v", what, err)
	}
}

// spawnGate holds each session's first spawn until spawnDecision has
// answered, the way TerminalHost does: one session at a time, its resume
// prompt a single dialog. released maps a session whose spawn may go to the
// conversation it resumes, "" for fresh.
type spawnGate struct {
	probe      spawnProbe
	invalidate func()
	// park is keepSession: the row is closed, not deleted, since the checkout
	// may come back and the row still carries the conversation.
	park func(id string)
	// notify raises a toast: what the gate decided on the user's behalf.
	notify func(ui.Toast)

	mu       sync.Mutex
	deciding string
	asking   *backend.Session
	released map[string]string
}

// release answers whether s may spawn and with which resume id, starting
// its decision when nothing is being decided; false holds the spawn.
func (g *spawnGate) release(ctx context.Context, s sessionView) (resume string, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if resume, ok := g.released[s.ID]; ok {
		return resume, true
	}
	if g.deciding == "" && g.asking == nil {
		g.deciding = s.ID
		go g.decide(ctx, s)
	}
	return "", false
}

// decide runs the gate for s. A session already running (a window reopened
// on a backend that kept it) is past the gate: Start is a no-op for it, and
// asking about its conversation would ask about one still going.
func (g *spawnGate) decide(ctx context.Context, s sessionView) {
	verdict := spawnPlain
	live, err := g.probe.Live(ctx, s.ID)
	logGateErr("live check", err)
	if !live {
		verdict = spawnDecision(ctx, g.probe, s.path, s.Session)
	}
	g.mu.Lock()
	g.deciding = ""
	switch verdict {
	case spawnPark:
		g.mu.Unlock()
		g.park(s.ID)
		g.notify(ui.Toast{Title: i18n.T("session.spawnGate.checkoutGone")})
		g.invalidate()
		return
	case spawnAsk:
		session := s.Session
		g.asking = &session
	case spawnResume:
		g.releaseLocked(s.ID, s.ProviderSessionID)
	default:
		g.releaseLocked(s.ID, "")
	}
	g.mu.Unlock()
	if verdict == spawnFresh {
		g.notify(ui.Toast{Title: i18n.T("session.spawnGate.conversationGone")})
	}
	g.invalidate()
}

// unseen is release for a spawn nobody is looking at: the resume id s starts
// with, false when the decision needs the user (the resume prompt, a checkout
// gone) and only s's first view may take it.
func (g *spawnGate) unseen(ctx context.Context, s sessionView) (resume string, ok bool) {
	live, err := g.probe.Live(ctx, s.ID)
	logGateErr("live check", err)
	if live {
		return "", true
	}
	switch spawnDecision(ctx, g.probe, s.path, s.Session) {
	case spawnPlain:
		return "", true
	case spawnResume:
		return s.ProviderSessionID, true
	case spawnFresh:
		g.notify(ui.Toast{Title: i18n.T("session.spawnGate.conversationGone")})
		return "", true
	}
	return "", false
}

func (g *spawnGate) releaseLocked(id, resume string) {
	if g.released == nil {
		g.released = map[string]string{}
	}
	g.released[id] = resume
}

// prompt is the session whose resume prompt is up, nil for none.
func (g *spawnGate) prompt() *backend.Session {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.asking
}

// answer closes the prompt and releases its spawn. remember stores choice
// as the provider's default; a dismiss passes false, being nobody's choice.
func (g *spawnGate) answer(ctx context.Context, choice string, remember bool) {
	g.mu.Lock()
	s := g.asking
	g.asking = nil
	if s == nil {
		g.mu.Unlock()
		return
	}
	resume := ""
	if choice == restoreResume {
		resume = s.ProviderSessionID
	}
	g.releaseLocked(s.ID, resume)
	g.mu.Unlock()
	g.invalidate()
	if !remember {
		return
	}
	if err := g.probe.SetSetting(ctx, restoreKey(s.Kind), "", choice); err != nil {
		g.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.host.rememberFailed", i18n.Args{"error": backend.ErrorText(err)})})
	}
}

// forgetAll is forget for every session: a restarted backend runs none of
// them.
func (g *spawnGate) forgetAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.released, g.asking = nil, nil
}

// forget drops what the gate knows about a session that left the workspace,
// so an id coming back goes through the gate again.
func (g *spawnGate) forget(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.released, id)
	if g.asking != nil && g.asking.ID == id {
		g.asking = nil
	}
}
