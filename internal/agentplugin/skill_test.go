package agentplugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// The harnesses that read no skills out of the plugin get the theme skill
// copied into a skills directory of their own, where they look for it.

// assertThemeSkill checks the skill sits whole in skillsDir, renamed.
func assertThemeSkill(t *testing.T, skillsDir string) {
	t.Helper()
	dir := filepath.Join(skillsDir, themeSkillName)
	manifest, err := os.ReadFile(filepath.Join(dir, skillManifest))
	if err != nil {
		t.Fatalf("read the installed skill: %v", err)
	}
	if !strings.HasPrefix(string(manifest), "---\nname: "+themeSkillName+"\n") {
		t.Errorf("SKILL.md is not named %q, the directory it sits in:\n%s", themeSkillName, manifest)
	}
	if !strings.Contains(string(manifest), "# lich themes") {
		t.Errorf("SKILL.md lost the release's body:\n%s", manifest)
	}
	for _, name := range themeSkillFiles[1:] {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the skill's %s is missing: %v", name, err)
		}
	}
}

func TestOpencodeInstallWritesTheThemeSkill(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	s, _ := fileServer(t, map[string]string{tagged(opencodeSource): "export const P = async () => ({})\n"})

	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.OpenCode}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	assertThemeSkill(t, filepath.Join(config, "opencode", "skills"))
}

func TestOMPInstallWritesTheThemeSkill(t *testing.T) {
	agent := ompHome(t)
	s, _ := fileServer(t, map[string]string{tagged(ompSource): "export default function lichPlugin(pi) {}\n"})

	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.OMP}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	assertThemeSkill(t, filepath.Join(agent, "skills"))
}

func TestCrushInstallWritesTheThemeSkill(t *testing.T) {
	tests := []struct {
		name string
		// skillsDir is $CRUSH_SKILLS_DIR, "" for unset.
		skillsDir string
	}{
		{name: "beside the crushrc"},
		{name: "in CRUSH_SKILLS_DIR, the one directory Crush then reads", skillsDir: t.TempDir()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := fileServer(t, crushFiles())
			configDir := t.TempDir()
			lichConfigHome(t)
			crushCLI(t, s, configDir, "0.88.1")
			t.Setenv("CRUSH_SKILLS_DIR", tc.skillsDir)

			if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.Crush}); err != nil {
				t.Fatalf("Install: %v", err)
			}
			want := tc.skillsDir
			if want == "" {
				want = filepath.Join(configDir, "skills")
			}
			assertThemeSkill(t, want)
		})
	}
}

func TestKiroInstallWritesTheThemeSkill(t *testing.T) {
	s, _ := fileServer(t, kiroFiles())
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv(fakeCLILog, filepath.Join(t.TempDir(), "cli.log"))
	s.bins = stubBin(self)
	_, home := kiroTestHome(t)

	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.Kiro}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	assertThemeSkill(t, filepath.Join(providers.KiroHome(home), "skills"))
}

// The skill is written before the version marker, so a release missing part of
// it fails the install without leaving a module that reports it installed.
func TestInstallWithoutTheThemeSkillLeavesNothingInstalled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, _ := fileServer(t, map[string]string{
		tagged(opencodeSource):                          "export const P = async () => ({})\n",
		tagged(themeSkillSource + "/" + "validate.mjs"): "",
	})

	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.OpenCode}); err == nil {
		t.Fatal("Install succeeded on a release without the whole skill")
	}
	if version, ok := s.installedVersion(providers.OpenCode); ok {
		t.Errorf("installedVersion = %q after a failed install, want nothing installed", version)
	}
}
