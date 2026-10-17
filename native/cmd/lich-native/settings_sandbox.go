package main

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"

	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
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

// The rungs, ordered by how much of the machine a confined session's
// provider leaves out of the sandbox: "off" confines nothing.
const (
	rungOff        = "off"
	rungAsk        = "ask"
	rungWorktrees  = "worktrees"
	rungEverywhere = "everywhere"
)

var sandboxRungs = []string{rungOff, rungAsk, rungWorktrees, rungEverywhere}

func sandboxKey(providerID string) string { return "provider." + providerID + ".sandbox" }

// sandboxLevel is providers-store.ts sandboxLevel: anything unknown is off.
func sandboxLevel(raw string) string {
	if slices.Contains(sandboxRungs, raw) {
		return raw
	}
	return rungOff
}

// loosens is climbsToRiskier over the ladder: a move toward fewer confined
// sessions, the only one confirmed. Re-choosing the same rung is no move.
func loosens(current, next string) bool {
	return slices.Index(sandboxRungs, next) < slices.Index(sandboxRungs, current)
}

func rungLabel(value string) string {
	if !slices.Contains(sandboxRungs, value) {
		return value
	}
	return i18n.T("settings.sandboxSettings.rung." + value)
}

// rungExposes is what a rung leaves running on the machine, said only when
// it is being climbed to. The widest rung leaves nothing running there.
func rungExposes(value string) string {
	if value == rungEverywhere || !slices.Contains(sandboxRungs, value) {
		return ""
	}
	return i18n.T("settings.sandboxSettings.exposes." + value)
}

func grantLabel(raw string) string {
	if raw == "true" {
		return i18n.T("settings.settingsSandbox.grantOn")
	}
	return i18n.T("settings.settingsSandbox.grantOff")
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
		return i18n.T("settings.sandboxSettings.ghActiveAccount")
	}
	return i18n.T("settings.sandboxSettings.ghAccount", i18n.Args{"login": login})
}

func agentKeysLine(keys []string) string {
	if len(keys) == 0 {
		return i18n.T("settings.sandboxSettings.sshNone")
	}
	return i18n.T("settings.sandboxSettings.sshLoaded", i18n.Args{"keys": strings.Join(keys, " · ")})
}

// cannotConfineCopy is sandbox-copy.ts: the reason after the em dash and the
// advice under the strip, per platform.
func cannotConfineCopy(goos string) (reason, advice string) {
	switch goos {
	case "windows":
		return i18n.T("env.sandbox.windowsReason"), i18n.T("env.sandbox.windowsAdvice")
	case "darwin":
		return i18n.T("env.sandbox.macReason"), i18n.T("env.sandbox.macAdvice")
	}
	return i18n.T("env.sandbox.linuxReason"), i18n.T("env.sandbox.linuxAdvice")
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
