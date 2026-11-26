package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
)

const codexLichOff = "[plugins.\"lich@lich-plugin\"]\nenabled = false\n"

func writeCodexConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Codex takes no plugin per session, so its spawn turns lich's back on in the
// home it runs in: Personal's, or the profile's.
func TestReenableTurnsCodexsPluginBackOn(t *testing.T) {
	personal := t.TempDir()
	t.Setenv("CODEX_HOME", personal)
	profile := t.TempDir()
	for name, dir := range map[string]string{"Personal": "", "a profile": profile} {
		t.Run(name, func(t *testing.T) {
			home := dir
			if home == "" {
				home = personal
			}
			path := writeCodexConfig(t, home, codexLichOff)
			changed, err := Reenable(providers.Codex, dir, "")
			if err != nil || !changed {
				t.Fatalf("Reenable = (%v, %v), want it turned back on", changed, err)
			}
			if data, _ := os.ReadFile(path); string(data) != "[plugins.\"lich@lich-plugin\"]\nenabled = true\n" {
				t.Errorf("config.toml = %q", data)
			}
		})
	}
}

// Antigravity's switch is its CLI's: lich runs `plugin enable` only when its
// config has the plugin turned off.
func TestReenableAntigravity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config := filepath.Join(home, ".gemini", "config", "config.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	_, calls := fakeCLI(t, "")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{`{}`, `{"plugins":{"lich":{"enabled":true}}}`} {
		if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if changed, err := Reenable(providers.Antigravity, "", self); changed || err != nil {
			t.Errorf("with %s: Reenable = (%v, %v), want nothing to do", body, changed, err)
		}
	}
	if got := calls(); got != nil {
		t.Fatalf("calls %v while the plugin was on, want none", got)
	}

	if err := os.WriteFile(config, []byte(`{"plugins":{"lich":{"enabled":false}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := Reenable(providers.Antigravity, "", self); !changed || err != nil {
		t.Fatalf("Reenable = (%v, %v), want it turned back on", changed, err)
	}
	if got := calls(); !slices.Equal(got, []string{"plugin enable " + pluginName}) {
		t.Errorf("calls %v, want the CLI's enable", got)
	}
}

func TestReenableLeavesTheOtherProvidersAlone(t *testing.T) {
	for _, p := range providers.Registry {
		if p.ID == providers.Codex || p.ID == providers.Antigravity {
			continue
		}
		if changed, err := Reenable(p.ID, t.TempDir(), "/nonexistent"); changed || err != nil {
			t.Errorf("Reenable(%s) = (%v, %v), want nothing to do", p.ID, changed, err)
		}
	}
}

// The install of the past comes out of opencode's and omp's own directories
// only once a release is there for their sessions to load instead.
func TestEnsureBundleUnpacksThenTakesTheInstallsOut(t *testing.T) {
	config := opencodeConfig(t)
	agent := ompHome(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	module := filepath.Join(config, "opencode", "plugins", opencodeFile)
	extension := filepath.Join(agent, ompExtensionsDir, ompFile)
	s, _ := fileServer(t, nil)
	s.lookPath = func(string) (string, error) { return "", os.ErrNotExist }

	s.tarballBase += "/missing"
	for _, path := range []string{module, extension} {
		writeModule(t, path, markedModule("0.20.0", "export default 1\n"))
	}
	s.EnsureBundle()
	for _, path := range []string{module, extension} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("with no release to load instead, %s was taken out: %v", path, err)
		}
	}

	s.tarballBase = s.tarballBase[:len(s.tarballBase)-len("/missing")]
	s.EnsureBundle()
	if _, version, ok := BundleDir(); !ok || version != testVersion {
		t.Fatalf("BundleDir version = (%q,%v), want the release unpacked", version, ok)
	}
	for _, path := range []string{module, extension} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s is still there beside the release (stat err = %v)", path, err)
		}
	}
}
