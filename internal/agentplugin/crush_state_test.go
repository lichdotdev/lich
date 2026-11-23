package agentplugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// crushStateFiles is every file an install onto a Crush with the lifecycle
// events fetches.
func crushStateFiles() map[string]string {
	files := crushFiles()
	for _, hook := range crushStateHooks {
		files[tagged(hook.script)] = "#!/bin/sh\n# " + hook.name + "\nexit 0\n"
	}
	return files
}

// installCrushOn is installCrush against a Crush reporting crushVersion.
func installCrushOn(t *testing.T, s *Service, crushVersion string) (string, string) {
	t.Helper()
	configDir := t.TempDir()
	lichConfig := lichConfigHome(t)
	crushCLI(t, s, configDir, crushVersion)
	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.Crush}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(configDir, "crushrc"))
	if err != nil {
		t.Fatalf("read crushrc: %v", err)
	}
	return string(data), filepath.Join(lichConfig, "lich", "plugin", "hooks", providers.Crush)
}

// hookLines is the event and command of every `hook add` line in a block, one
// "<event> <script basename> <arg>" each.
func hookLines(block string) []string {
	var out []string
	for line := range strings.SplitSeq(block, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "hook" || fields[1] != "add" {
			continue
		}
		command, ok := commandOf(line, nameOf(line))
		if !ok {
			continue
		}
		words := strings.Fields(strings.ReplaceAll(command, "'", ""))
		entry := fields[2] + " " + filepath.Base(words[0])
		if len(words) > 1 {
			entry += " " + words[1]
		}
		out = append(out, entry)
	}
	return out
}

func nameOf(line string) string {
	_, rest, _ := strings.Cut(line, "--name ")
	name, _, _ := strings.Cut(rest, " ")
	return name
}

// Below the release that has the lifecycle events, the block is the one lich has
// always written: every line on PreToolUse.
func TestCrushrcBelowTheEventsReleaseRegistersOnlyPreToolUse(t *testing.T) {
	for _, crush := range []string{"", "0.88.0", "0.98.2"} {
		block := crushrcBlock(testVersion, crush, "/plugins/lich", "")
		if block != crushrcBlock(testVersion, "", "/plugins/lich", "") {
			t.Errorf("Crush %q: the block differs from the one an unread version gets:\n%s", crush, block)
		}
		for _, line := range hookLines(block) {
			if !strings.HasPrefix(line, crushHookEvent+" ") {
				t.Errorf("Crush %q registers %q, want PreToolUse only", crush, line)
			}
		}
	}
}

// From the events release on, each report rides the event it was measured on
// (docs/hooks/session-state.md, Crush).
func TestCrushrcReportsStateFromTheEventsRelease(t *testing.T) {
	want := []string{
		"SessionStart report-session-start.sh crush",
		"TurnStart report-session-start.sh crush",
		"TurnStart report-state.sh busy",
		"PreToolUse report-tool.sh",
		"PreToolUse report-touched.sh",
		"PostToolUse report-state.sh busy",
		"PostToolUseFailure report-state.sh busy",
		"PermissionRequest report-state.sh waiting",
		"TurnEnd report-state.sh done",
		"StopFailure report-state.sh done",
		"SessionEnd report-state.sh idle",
	}
	got := hookLines(crushrcBlock(testVersion, crushEventsVersion, "/plugins/lich", ""))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("hook lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Every hook name is its own: `hook remove` takes one by name.
func TestCrushStateHookNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, hook := range crushHooksFor(SubagentHookRelease, crushEventsVersion) {
		if seen[hook.name] {
			t.Errorf("hook name %q is used twice", hook.name)
		}
		seen[hook.name] = true
	}
}

// The state reports source the waiting reason's reader beside them, so the
// install fetches it and fetches no script twice.
func TestCrushInstallOnTheEventsReleaseWritesTheStateScripts(t *testing.T) {
	s, asked := fileServer(t, crushStateFiles())
	rc, scriptDir := installCrushOn(t, s, "0.100.0")

	for _, name := range []string{"report-state.sh", "report-tool.sh", "detail.jq", "conversation-id.sh"} {
		if _, err := os.Stat(filepath.Join(scriptDir, name)); err != nil {
			t.Errorf("%s was not installed: %v", name, err)
		}
	}
	if !strings.Contains(rc, "hook add "+crushTurnEndEvent+" ") {
		t.Errorf("crushrc registers no turn end:\n%s", rc)
	}
	fetched := map[string]int{}
	for _, path := range asked() {
		fetched[path]++
	}
	for path, n := range fetched {
		if n > 1 {
			t.Errorf("%s was fetched %d times", path, n)
		}
	}
}

// Installed is the relay's question whether a session's silence means
// something, so for Crush it is whether lich's block reports state, not only
// whether a block is there.
func TestCrushIsInstalledOnlyWithABlockThatReportsState(t *testing.T) {
	s, _ := fileServer(t, crushStateFiles())
	installCrushOn(t, s, "0.98.2")
	if s.Installed(providers.Crush) {
		t.Error("a block written for a Crush without the events reads as reporting state")
	}

	s, _ = fileServer(t, crushStateFiles())
	installCrushOn(t, s, "0.100.0")
	if !s.Installed(providers.Crush) {
		t.Error("a block that registers the state reports reads as not installed")
	}
}

// A turn end the user registered outside lich's block is not lich's report.
func TestCrushTurnEndOutsideTheBlockDoesNotCount(t *testing.T) {
	block := crushrcBlock(testVersion, "", "/plugins/lich", "")
	rc := "hook add TurnEnd --name ding --command 'paplay bell.oga'\n\n" + block
	if blockReportsState(rc) {
		t.Error("the user's own TurnEnd hook read as lich's state report")
	}
	if !blockReportsState(crushrcBlock(testVersion, crushEventsVersion, "/plugins/lich", "")) {
		t.Error("lich's own block with the state reports read as reporting none")
	}
}

// A repair repoints the binary and keeps the registrations: a block with the
// state reports keeps them, and one without gains none whose scripts were never
// fetched.
func TestRepairKeepsWhetherCrushsBlockReportsState(t *testing.T) {
	for _, crush := range []string{"", crushEventsVersion} {
		s, _ := fileServer(t, nil)
		configDir := t.TempDir()
		lichConfigHome(t)
		crushCLI(t, s, configDir, "0.100.0")
		scripts, err := pluginScriptDir(providers.Crush)
		if err != nil {
			t.Fatal(err)
		}
		rc := filepath.Join(configDir, "crushrc")
		writeDoc(t, rc, crushrcBlock(testVersion, crush, scripts, "/opt/old/lich"))

		if err := s.crushRewrite(freshLich); err != nil {
			t.Fatalf("crushRewrite: %v", err)
		}
		if got, want := readFile(t, rc), crushrcBlock(testVersion, crush, scripts, freshLich); got != want {
			t.Errorf("Crush %q: repaired block =\n%s\nwant\n%s", crush, got, want)
		}
	}
}

// A Crush upgraded past the events release under a block written before it has
// the events and reports nothing until the block is written again, so its row
// offers the update even when no newer plugin release exists.
func TestCrushStatusOffersTheUpdateOnceCrushHasTheEvents(t *testing.T) {
	s, _ := fileServer(t, crushStateFiles())
	installCrushOn(t, s, "0.98.2")
	if statusOf(t, s.status(), providers.Crush).UpdateAvailable {
		t.Fatal("a block current for its Crush offered an update")
	}

	t.Setenv(fakeCLIVersion, "0.100.0")
	if st := statusOf(t, s.status(), providers.Crush); !st.UpdateAvailable || !st.Reregister {
		t.Errorf("a block behind the Crush installed: %+v, want an update that re-registers", st)
	}
	if _, err := s.Update(t.Context(), api.PluginProviderOptions{Provider: providers.Crush}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if st := statusOf(t, s.status(), providers.Crush); st.UpdateAvailable || st.Reregister {
		t.Errorf("the update left the block behind: %+v", st)
	}
}
