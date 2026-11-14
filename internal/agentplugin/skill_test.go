package agentplugin

import (
	"errors"
	"io/fs"
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
	dir := filepath.Join(skillsDir, themeSkill.name)
	manifest, err := os.ReadFile(filepath.Join(dir, skillManifest))
	if err != nil {
		t.Fatalf("read the installed skill: %v", err)
	}
	if !strings.HasPrefix(string(manifest), "---\nname: "+themeSkill.name+"\n") {
		t.Errorf("SKILL.md is not named %q, the directory it sits in:\n%s", themeSkill.name, manifest)
	}
	if !strings.Contains(string(manifest), "# lich themes") {
		t.Errorf("SKILL.md lost the release's body:\n%s", manifest)
	}
	for _, name := range themeSkill.files[1:] {
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
		tagged(opencodeSource):                           "export const P = async () => ({})\n",
		tagged(themeSkill.source + "/" + "validate.mjs"): "",
	})

	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.OpenCode}); err == nil {
		t.Fatal("Install succeeded on a release without the whole skill")
	}
	if version, ok := s.installedVersion(providers.OpenCode); ok {
		t.Errorf("installedVersion = %q after a failed install, want nothing installed", version)
	}
}

// retag moves the files a test serves at testVersion to version.
func retag(files map[string]string, version string) map[string]string {
	out := make(map[string]string, len(files))
	for path, body := range files {
		out[taggedAt(version, strings.TrimPrefix(path, tagged("")))] = body
	}
	return out
}

func TestInstallSkillsAddsTheManualExtensionSkillFromItsRelease(t *testing.T) {
	s, _ := fileServerAt(t, ExtensionSkillRelease, nil)
	dir := t.TempDir()

	if err := s.installSkills(ExtensionSkillRelease, dir); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	assertThemeSkill(t, dir)
	manifest, err := os.ReadFile(filepath.Join(dir, extensionSkill.name, skillManifest))
	if err != nil {
		t.Fatalf("read the extension skill: %v", err)
	}
	if !strings.HasPrefix(string(manifest), "---\nname: "+extensionSkill.name+"\n") {
		t.Errorf("SKILL.md is not named %q, the directory it sits in:\n%s", extensionSkill.name, manifest)
	}
	if !strings.Contains(string(manifest), "disable-model-invocation: true\nuser-invocable: true\n") {
		t.Errorf("SKILL.md lost what keeps it manual only:\n%s", manifest)
	}
}

func TestInstallSkillsFromAnOlderReleaseAsksForNoExtensionSkill(t *testing.T) {
	s, asked := fileServer(t, nil)

	if err := s.installSkills(testVersion, t.TempDir()); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	for _, path := range asked() {
		if strings.Contains(path, extensionSkill.source) {
			t.Errorf("asked %s of release %s, which predates the extension skill", path, testVersion)
		}
	}
}

// Kiro cannot keep a skill out of the agent's context, so the extension skill
// would cost every Kiro user its description on every turn.
func TestKiroInstallLeavesTheExtensionSkillOut(t *testing.T) {
	s, _ := fileServerAt(t, ExtensionSkillRelease, retag(kiroFiles(), ExtensionSkillRelease))
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
	skills := filepath.Join(providers.KiroHome(home), "skills")
	assertThemeSkill(t, skills)
	if _, err := os.Stat(filepath.Join(skills, extensionSkill.name)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Kiro got the extension skill (stat: %v), which it cannot keep manual only", err)
	}
}

// opencode 1 lists every skill with a description to the agent, whatever its
// frontmatter says, so its copy of the extension skill has none.
func TestOpencodeInstallDropsTheExtensionSkillsDescription(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	s, _ := fileServerAt(t, ExtensionSkillRelease, map[string]string{
		taggedAt(ExtensionSkillRelease, opencodeSource): "export const P = async () => ({})\n",
	})

	if _, err := s.Install(t.Context(), api.PluginProviderOptions{Provider: providers.OpenCode}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	skills := filepath.Join(config, "opencode", "skills")
	manifest, err := os.ReadFile(filepath.Join(skills, extensionSkill.name, skillManifest))
	if err != nil {
		t.Fatalf("read the extension skill: %v", err)
	}
	if strings.Contains(string(manifest), "\ndescription:") {
		t.Errorf("opencode's copy keeps its description, which opencode 1 lists to the agent:\n%s", manifest)
	}
	if !strings.HasPrefix(string(manifest), "---\nname: "+extensionSkill.name+"\ndisable-model-invocation: true\n") {
		t.Errorf("opencode's copy lost its name or what hides it from opencode 2:\n%s", manifest)
	}
	theme, err := os.ReadFile(filepath.Join(skills, themeSkill.name, skillManifest))
	if err != nil || !strings.Contains(string(theme), "\ndescription:") {
		t.Errorf("the theme skill lost its description (err %v):\n%s", err, theme)
	}
}

func TestDropDescriptionTouchesOnlyTheFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), skillManifest)
	body := "---\nname: x\ndescription: Hide me.\nuser-invocable: true\n---\n\ndescription: stays in the body\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := dropDescription(path); err != nil {
		t.Fatalf("dropDescription: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "---\nname: x\nuser-invocable: true\n---\n\ndescription: stays in the body\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
