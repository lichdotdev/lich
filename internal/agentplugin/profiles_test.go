package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// oldVersion is a release a profile was left on before Personal moved past it.
const oldVersion = "0.12.0"

// codexPluginListFile is what the fake Codex CLI prints for `plugin list
// --json`, read from the CODEX_HOME it runs with (Personal's when unset). Codex
// reads its plugin set from config.toml, so a home whose config.toml links to
// another's lists the other's.
const codexPluginListFile = "fake-plugin-list.json"

func printCodexPluginList() {
	dir := os.Getenv("CODEX_HOME")
	if dir == "" {
		dir, _ = codexprofile.PersonalDir()
	}
	if config, err := filepath.EvalSymlinks(filepath.Join(dir, "config.toml")); err == nil {
		dir = filepath.Dir(config)
	}
	if data, err := os.ReadFile(filepath.Join(dir, codexPluginListFile)); err == nil {
		fmt.Print(string(data))
	}
}

func writeClaudePlugin(t *testing.T, dir, version string) {
	t.Helper()
	state := `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"` + version + `"}]}}`
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCodexPlugin(t *testing.T, dir, version string) {
	t.Helper()
	list := `{"installed":[{"pluginId":"lich@lich-plugin","version":"` + version + `","installed":true}]}`
	if err := os.WriteFile(filepath.Join(dir, codexPluginListFile), []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
}

func createClaudeProfile(t *testing.T, name string) claudeprofile.Profile {
	t.Helper()
	p, err := claudeprofile.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// envLines reads the per-call environment log the fake CLI keeps.
func envLines(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func ranAs(lines []string, dir string) int {
	n := 0
	for _, line := range lines {
		if strings.HasPrefix(line, dir+"|") {
			n++
		}
	}
	return n
}

// recordEvents wires the service's emit into a channel of what it reported.
func recordEvents(svc *Service) <-chan ProfilesEvent {
	events := make(chan ProfilesEvent, 4)
	svc.SetEmit(func(name string, data any) {
		if name == api.PluginProfilesEvent.Name {
			events <- data.(ProfilesEvent)
		}
	})
	return events
}

func nextEvent(t *testing.T, events <-chan ProfilesEvent) ProfilesEvent {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(30 * time.Second):
		t.Fatal("no profiles event after the update")
		return ProfilesEvent{}
	}
}

// TestPersonalUpdateReachesEveryClaudeProfile: updating the plugin in Personal
// used to leave a profile on the release it was logged in with. Now the same
// pinned install runs as the profile, after Personal's.
func TestPersonalUpdateReachesEveryClaudeProfile(t *testing.T) {
	dir := workProfile(t)
	home, _ := os.UserHomeDir()
	writeClaudePlugin(t, filepath.Join(home, ".claude"), testVersion)
	writeClaudePlugin(t, dir, oldVersion)
	svc, calls := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLIEnvLog, envLog)
	events := recordEvents(svc)

	if err := svc.Update(providers.Claude); err != nil {
		t.Fatal(err)
	}
	ev := nextEvent(t, events)
	if ev.Provider != providers.Claude || ev.Name != "Claude Code" || ev.Version != testVersion || len(ev.Failures) != 0 {
		t.Errorf("event %+v, want claude at %s with no failures", ev, testVersion)
	}
	want := append(slices.Clone(pinnedCalls[providers.Claude]), pinnedCalls[providers.Claude]...)
	if got := calls(); !slices.Equal(got, want) {
		t.Errorf("calls %v, want Personal's install then the profile's: %v", got, want)
	}
	if n := ranAs(envLines(t, envLog), dir); n != len(pinnedCalls[providers.Claude]) {
		t.Errorf("%d calls ran as the profile, want %d", n, len(pinnedCalls[providers.Claude]))
	}
}

// TestSyncProfilesInstallsWhereMissingAndNamesFailures is the backend-start
// sweep: a profile logged in before profiles got the plugin has none and gets
// it, one already at the release is left alone, and one whose install fails is
// reported by name.
func TestSyncProfilesInstallsWhereMissingAndNamesFailures(t *testing.T) {
	workProfile(t)
	home, _ := os.UserHomeDir()
	writeClaudePlugin(t, filepath.Join(home, ".claude"), testVersion)
	done := createClaudeProfile(t, "Done")
	writeClaudePlugin(t, done.Dir, testVersion)
	svc, calls := fakeCLI(t, "plugin install")
	events := recordEvents(svc)

	svc.SyncProfiles()

	ev := nextEvent(t, events)
	if len(ev.Failures) != 1 || ev.Failures[0].Profile != "work" || ev.Failures[0].Name != "Work" ||
		!strings.Contains(ev.Failures[0].Error, "refusing") {
		t.Errorf("failures %+v, want the work profile's install refused", ev.Failures)
	}
	if got := calls(); !slices.Equal(got, pinnedCalls[providers.Claude]) {
		t.Errorf("calls %v, want one install, into the profile that has none", got)
	}
}

// TestSyncProfilesWaitsForPersonal: while Personal itself is behind the
// release, a profile that has the plugin keeps its version, so the two never
// drift further apart; one with no plugin at all still gets it. Personal with
// no plugin leads nowhere.
func TestSyncProfilesWaitsForPersonal(t *testing.T) {
	work := workProfile(t)
	home, _ := os.UserHomeDir()
	personal := filepath.Join(home, ".claude")
	writeClaudePlugin(t, work, oldVersion)
	bare := createClaudeProfile(t, "Bare")
	svc, calls := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLIEnvLog, envLog)
	events := recordEvents(svc)

	svc.SyncProfiles()
	if got := calls(); got != nil {
		t.Fatalf("calls %v with no plugin in Personal, want none", got)
	}

	writeClaudePlugin(t, personal, oldVersion)
	svc.SyncProfiles()
	if ev := nextEvent(t, events); len(ev.Failures) != 0 {
		t.Errorf("failures %+v", ev.Failures)
	}
	lines := envLines(t, envLog)
	if ranAs(lines, work) != 0 || ranAs(lines, bare.Dir) != len(pinnedCalls[providers.Claude]) {
		t.Errorf("calls ran as %v, want only the bare profile's install", lines)
	}
}

// TestPersonalUpdateReachesEveryCodexProfile: the same for Codex, whose
// installed version is asked of the CLI run as the profile.
func TestPersonalUpdateReachesEveryCodexProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("LICH_DEV", "")
	personal := filepath.Join(home, ".codex")
	if err := os.MkdirAll(personal, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCodexPlugin(t, personal, testVersion)
	current, err := codexprofile.Create("Current")
	if err != nil {
		t.Fatal(err)
	}
	writeCodexPlugin(t, current.Dir, testVersion)
	stale, err := codexprofile.Create("Stale")
	if err != nil {
		t.Fatal(err)
	}
	svc, calls := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLICodexEnvLog, envLog)
	events := recordEvents(svc)

	if err := svc.Update(providers.Codex); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, events); ev.Provider != providers.Codex || len(ev.Failures) != 0 {
		t.Errorf("event %+v", ev)
	}
	installs := slices.DeleteFunc(calls(), func(c string) bool { return c == "plugin list --json" })
	want := append(slices.Clone(pinnedCalls[providers.Codex]), pinnedCalls[providers.Codex]...)
	if !slices.Equal(installs, want) {
		t.Errorf("installs %v, want Personal's then the stale profile's: %v", installs, want)
	}
	lines := envLines(t, envLog)
	if got := ranAs(lines, stale.Dir); got != len(pinnedCalls[providers.Codex])+1 {
		t.Errorf("%d calls ran as the stale profile, want its version read and its install", got)
	}
	if got := ranAs(lines, current.Dir); got != 1 {
		t.Errorf("%d calls ran as the current profile, want only its version read", got)
	}
}

// TestSyncProfilesMigratesALinkedCodexProfileFirst: a Codex profile made when
// config.toml was shared still links Personal's, so its plugin list read
// Personal's and the pass left it alone; the profile's next spawn then rebuilt
// its own config.toml without the lich plugin or its marketplace. The pass now
// brings the profile to its own config.toml before asking what it has.
func TestSyncProfilesMigratesALinkedCodexProfileFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("LICH_DEV", "")
	personal := filepath.Join(home, ".codex")
	if err := os.MkdirAll(personal, 0o755); err != nil {
		t.Fatal(err)
	}
	config := "model = \"gpt-5\"\n\n[plugins.\"lich@lich-plugin\"]\nenabled = true\n"
	if err := os.WriteFile(filepath.Join(personal, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCodexPlugin(t, personal, testVersion)
	old, err := codexprofile.Create("Old")
	if err != nil {
		t.Fatal(err)
	}
	ownConfig := filepath.Join(old.Dir, "config.toml")
	if err := os.Remove(ownConfig); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(personal, "config.toml"), ownConfig); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	svc, _ := fakeCLI(t, "")
	envLog := filepath.Join(t.TempDir(), "env.log")
	t.Setenv(fakeCLICodexEnvLog, envLog)
	events := recordEvents(svc)

	svc.SyncProfiles()

	if ev := nextEvent(t, events); ev.Provider != providers.Codex || len(ev.Failures) != 0 {
		t.Errorf("event %+v, want codex with no failures", ev)
	}
	if info, err := os.Lstat(ownConfig); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("profile config.toml still a link (%v), want its own file", err)
	}
	if got := ranAs(envLines(t, envLog), old.Dir); got != len(pinnedCalls[providers.Codex])+1 {
		t.Errorf("%d calls ran as the profile, want its version read and its install", got)
	}
}
