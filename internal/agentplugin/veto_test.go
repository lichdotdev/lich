package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/shquote"
)

// The release before the subagent hook, which ships no veto-subagent.sh: an
// install of it that asked for the script would 404 and install nothing.
const preVetoRelease = "0.21.1"

// From SubagentHookRelease, Kiro's agent asks lich before each of its two
// subagent tools (docs/hooks/subagent-cards.md); an older release's agent
// registers no veto.
func TestKiroAgentRegistersTheVetoFromTheHookRelease(t *testing.T) {
	for version, want := range map[string]bool{SubagentHookRelease: true, preVetoRelease: false} {
		hooks := decodeKiroAgent(t, version, "/opt/lich/hooks")["hooks"].(map[string]any)
		var matchers []string
		for _, entry := range hooks["preToolUse"].([]any) {
			e := entry.(map[string]any)
			if e["command"] == kiroCommand("/opt/lich/hooks", vetoScript, providers.Kiro) {
				matchers = append(matchers, e["matcher"].(string))
			}
		}
		slices.Sort(matchers)
		if got := slices.Equal(matchers, []string{"subagent", "use_subagent"}); got != want {
			t.Errorf("%s: veto matchers %v, want registered %v", version, matchers, want)
		}
		if has := slices.Contains(kiroScripts(version), vetoScript); has != want {
			t.Errorf("%s: kiroScripts carries the veto %v, want %v", version, has, want)
		}
	}
}

// From SubagentHookRelease, crushrc runs the veto on the agent tool, with
// longer than a report to wait: lich may open a session before it answers.
func TestCrushrcRegistersTheVetoFromTheHookRelease(t *testing.T) {
	command := shquote.Quote(shquote.Quote(filepath.Join("/opt/lich/hooks", "veto-subagent.sh")) + " crush")
	veto := "hook add PreToolUse --name lich-subagent-cards --matcher '^agent$' --command " + command + " --timeout 15"
	if rc := crushrcBlock(SubagentHookRelease, "/opt/lich/hooks", ""); !strings.Contains(rc, veto) {
		t.Errorf("crushrc does not run the veto as the contract says:\n%s", rc)
	}
	if rc := crushrcBlock(preVetoRelease, "/opt/lich/hooks", ""); strings.Contains(rc, "veto-subagent") {
		t.Errorf("crushrc of a release without the veto runs it:\n%s", rc)
	}
}

func TestCrushInstallWritesTheVeto(t *testing.T) {
	files := map[string]string{}
	for _, hook := range crushHooksFor(SubagentHookRelease) {
		files[taggedAt(SubagentHookRelease, hook.script)] = "#!/bin/sh\nexit 0\n"
	}
	for _, script := range crushSourcedScripts {
		files[taggedAt(SubagentHookRelease, script)] = "# sourced\n"
	}
	s, _ := fileServerAt(t, SubagentHookRelease, files)
	_, scriptDir := installCrush(t, s, "")
	for _, name := range []string{"veto-subagent.sh", "conversation-id.sh"} {
		if _, err := os.Stat(filepath.Join(scriptDir, name)); err != nil {
			t.Errorf("the veto's %s was not installed: %v", name, err)
		}
	}
}

// Antigravity's own hooks.json names the veto from SubagentHookRelease, so the
// install ships the script beside it; an older release is not asked for one.
func TestAntigravityInstallShipsTheVetoFromTheHookRelease(t *testing.T) {
	dir := antigravityHome(t)
	files := map[string]string{taggedAt(SubagentHookRelease, antigravityHooksFile): `{"lich":{}}`}
	for _, script := range antigravityScriptsFor(SubagentHookRelease) {
		files[taggedAt(SubagentHookRelease, script)] = "#!/bin/sh\nexit 0\n"
	}
	s, _ := fileServerAt(t, SubagentHookRelease, files)
	antigravityCLI(t, s)
	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.Antigravity}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, antigravityScriptDir, "veto-subagent.sh")); err != nil {
		t.Errorf("the veto was not installed: %v", err)
	}
	if slices.Contains(antigravityScriptsFor(preVetoRelease), vetoScript) {
		t.Error("a release without the veto is asked for it")
	}
}
