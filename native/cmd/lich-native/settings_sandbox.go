package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// The sandbox keys: the grants mirror store.sshAgentKey and store.ghTokenKey,
// which the spawn reads; the rung is providers-store.ts sandboxKey.
const (
	sshAgentKey = "sandbox.ssh-agent"
	ghTokenKey  = "sandbox.gh-token"
	// sandboxLadderKey names the confined block's row. It is no setting: the
	// block draws one rung row per provider, each scoped on its own key.
	sandboxLadderKey = "sandbox.ladder"
)

// ponytail: English until the native window has i18n (settings.sandboxSettings,
// env.sandbox).
const (
	sandboxSummary     = "An empty home, the machine read-only, writes only in the checkout. The network stays on."
	confinedTitle      = "Which sessions run confined"
	confinedAlso       = "isolate bwrap seatbelt"
	confinedHint       = "Ask puts the question in the New session menu and the New worktree dialog. A session opened where nobody can answer — by another session, or through the MCP tool — is confined. Terminals are never confined; the sandbox is for agents working unattended."
	carryTitle         = "What a confined session may carry in"
	sshTitle           = "SSH agent"
	sshAlso            = "push keys"
	sshDesc            = "git push works inside. Signs with every identity in your agent, against any host, for as long as the session runs."
	sshLoaded          = "Loaded: "
	sshNone            = "Nothing loaded."
	ghTitle            = "GitHub token"
	ghAlso             = "gh credentials"
	ghDesc             = "gh works inside. The agent can read the token out of its environment and spend it outside this repository."
	ghAccountAs        = "As %s."
	ghActiveAccount    = "As gh's active account."
	rungConfirmTitle   = "Confine fewer %s sessions"
	rungConfirmDesc    = "%s — %s"
	rungConfirm        = "Leave unconfined"
	canConfineText     = "This machine can confine sessions"
	cannotConfineText  = "This machine cannot confine sessions"
	statusDetailPrefix = "— "
	grantOn            = "On"
	grantOff           = "Off"
)

// The rungs, ordered by how much of the machine a confined session's
// provider leaves out of the sandbox: "off" confines nothing.
const (
	rungOff        = "off"
	rungAsk        = "ask"
	rungWorktrees  = "worktrees"
	rungEverywhere = "everywhere"
)

var sandboxRungs = []struct{ value, label, exposes string }{
	{rungOff, "Off", "Every session runs on the machine."},
	{rungAsk, "Ask", "A session whose box you untick runs on the machine."},
	{rungWorktrees, "Worktrees", "Sessions in the project directory run on the machine."},
	{rungEverywhere, "Everywhere", ""},
}

func sandboxKey(providerID string) string { return "provider." + providerID + ".sandbox" }

// sandboxLevel is providers-store.ts sandboxLevel: anything unknown is off.
func sandboxLevel(raw string) string {
	for _, r := range sandboxRungs {
		if r.value == raw {
			return raw
		}
	}
	return rungOff
}

// loosens is climbsToRiskier over the ladder: a move toward fewer confined
// sessions, the only one confirmed. Re-choosing the same rung is no move.
func loosens(current, next string) bool {
	rank := func(v string) int {
		return slices.IndexFunc(sandboxRungs, func(r struct{ value, label, exposes string }) bool { return r.value == v })
	}
	return rank(next) < rank(current)
}

func rungLabel(value string) string {
	for _, r := range sandboxRungs {
		if r.value == value {
			return r.label
		}
	}
	return value
}

// rungExposes is what a rung leaves running on the machine, said only when
// it is being climbed to.
func rungExposes(value string) string {
	for _, r := range sandboxRungs {
		if r.value == value {
			return r.exposes
		}
	}
	return ""
}

func grantLabel(raw string) string {
	if raw == "true" {
		return grantOn
	}
	return grantOff
}

// effectiveValue is what a spawn in scope reads, as the backend's
// projectSetting: the project's value, else the global one.
func effectiveValue(values scopedValues, globals map[string]string, key, scope string) string {
	if v, ok := values.set(key, scope); ok && scope != "" {
		return v
	}
	return globals[key]
}

// ghAccountLine names the account whose token a confined session would get:
// the project's vcs.account login, else gh's active one.
func ghAccountLine(account string) string {
	login := account
	if _, after, ok := strings.Cut(account, "/"); ok {
		login = after
	}
	if login == "" {
		return ghActiveAccount
	}
	return fmt.Sprintf(ghAccountAs, login)
}

func agentKeysLine(keys []string) string {
	if len(keys) == 0 {
		return sshNone
	}
	return sshLoaded + strings.Join(keys, " · ")
}

// cannotConfineCopy is sandbox-copy.ts: the reason after the em dash and the
// advice under the strip, per platform.
func cannotConfineCopy(goos string) (reason, advice string) {
	switch goos {
	case "windows":
		return "lich has no sandbox backend on Windows", "There is nothing to install — every session runs on the machine."
	case "darwin":
		return "sandbox-exec is not available", "macOS ships /usr/bin/sandbox-exec, so a machine without a working one is broken in a way lich cannot repair. Every session runs on the machine."
	}
	return "bubblewrap is not installed", "Install bubblewrap and reopen lich. Until then every session runs on the machine."
}

// sandboxRungEntries are every provider's rung, scoped on its own key. They
// are counted for the badges and loaded per project whether or not the
// provider is enabled; the pane draws the enabled ones.
func sandboxRungEntries() []settingEntry {
	var entries []settingEntry
	for kind := range providerIcons {
		if kind != "shell" {
			entries = append(entries, settingEntry{key: sandboxKey(kind), scope: scopeOverridable})
		}
	}
	slices.SortFunc(entries, func(a, b settingEntry) int { return strings.Compare(a.key, b.key) })
	return entries
}

// sandboxMachine is what the backend answered about the machine and the
// global values the pane draws against. Which providers it draws rungs for is
// the provider book's.
type sandboxMachine struct {
	loaded    bool
	backend   string // "bwrap", "sandbox-exec", "" when it cannot confine
	agentKeys []string
	globals   map[string]string
}

// sandboxSettings is the Sandbox pane's state.
type sandboxSettings struct {
	mu      sync.Mutex
	machine sandboxMachine

	rungs   map[string]*widget.Enum
	ssh, gh ui.SwitchState
	// change is the loosening the dialog asks about while asking; it stays
	// after an answer so the closing dialog keeps its text.
	change  rungChange
	asking  bool
	dialog  ui.DialogState
	cancel  widget.Clickable
	confirm widget.Clickable
}

// rungChange is a loosening waiting on its confirmation.
type rungChange struct{ providerID, providerName, level string }

func (s *sandboxSettings) snapshot() sandboxMachine {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.machine
}

// canConfine reports whether the grants have anything to qualify: nothing
// is drawn under a machine that cannot confine, or has not answered yet.
func (s *sandboxSettings) canConfine() bool {
	m := s.snapshot()
	return m.loaded && m.backend != ""
}

// loadSandbox rereads the machine: the backend, the agent's keys (re-read on
// every open, since ssh-add runs outside this window) and the global values
// of every provider's rung and of the grants.
func (s *settingsScreen) loadSandbox() {
	go func() {
		m, err := readSandboxMachine(s.ctx, s.client)
		if err != nil {
			log.Printf("settings: sandbox: %v", err)
			return
		}
		s.sandbox.mu.Lock()
		s.sandbox.machine = m
		s.sandbox.mu.Unlock()
		s.invalidate()
	}()
}

func readSandboxMachine(ctx context.Context, c *lichclient.Client) (sandboxMachine, error) {
	m := sandboxMachine{loaded: true, globals: map[string]string{}}
	if err := c.Call(ctx, "system.SandboxBackend", &m.backend); err != nil {
		return m, err
	}
	if err := c.Call(ctx, "system.SSHAgentKeys", &m.agentKeys); err != nil {
		return m, err
	}
	keys := []string{sshAgentKey, ghTokenKey}
	for _, e := range sandboxRungEntries() {
		keys = append(keys, e.key)
	}
	for _, key := range keys {
		var err error
		if m.globals[key], err = c.GetSetting(ctx, key, ""); err != nil {
			return m, err
		}
	}
	return m, nil
}

// writeSandbox stores value at key in the open scope, and in the values the
// pane draws from so the change shows on the next frame.
func (s *settingsScreen) writeSandbox(key, value string) {
	scope := s.scope
	if scope == "" {
		s.sandbox.mu.Lock()
		s.sandbox.machine.globals[key] = value
		s.sandbox.mu.Unlock()
	} else {
		s.mu.Lock()
		if s.values == nil {
			s.values = scopedValues{}
		}
		if s.values[key] == nil {
			s.values[key] = map[string]string{}
		}
		s.values[key][scope] = value
		s.mu.Unlock()
	}
	go func() {
		if err := s.client.SetSetting(s.ctx, key, scope, value); err != nil {
			log.Printf("settings: save %s: %v", key, err)
		}
	}()
}
