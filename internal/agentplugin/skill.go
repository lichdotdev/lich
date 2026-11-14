package agentplugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/internal/semver"
)

// The plugin's skills, for the harnesses that do not load them from the
// plugin itself. Claude Code, Codex and Antigravity read skills/ out of the
// installed plugin, and Cursor loads Claude Code's installed plugins whole;
// opencode, oh-my-pi, Crush and Kiro only read a skills directory of their own,
// so their install copies the skills there.

const skillManifest = "SKILL.md"

// pluginSkill is one skill as a plugin release ships it. name is its name and
// directory once copied out of the plugin: it loses the plugin's namespace
// there, and a bare `theme` would sit beside the user's own skills under a
// name one of them may already hold. files is every file it ships.
type pluginSkill struct {
	source, plain, name string
	files               []string
}

// themeSkill's SKILL.md tells the agent to copy the template and run the
// validator from the skill's own directory.
var themeSkill = pluginSkill{
	source: "skills/theme", plain: "theme", name: "lich-theme",
	files: []string{skillManifest, "template.json", "validate.mjs"},
}

// extensionSkill is manual only where the harness can say so: its
// frontmatter keeps it out of the agent's context on oh-my-pi, Crush and
// opencode 2, and opencode 1 gets it without a description, the one thing
// that version reads (installOpencodeSkills). Kiro has no such setting, so
// Kiro's install leaves it out rather than make every Kiro user pay for it
// (docs/ceilings.md).
var extensionSkill = pluginSkill{
	source: "skills/extension", plain: "extension", name: "lich-extension",
	files: []string{skillManifest},
}

// ExtensionSkillRelease is the first plugin release that ships extensionSkill.
const ExtensionSkillRelease = "0.21.1"

// installSkills writes every skill release ships into skillsDir, for a
// harness that can keep extensionSkill manual only.
func (s *Service) installSkills(version, skillsDir string) error {
	if err := s.installSkill(version, skillsDir, themeSkill); err != nil {
		return err
	}
	if semver.Less(version, ExtensionSkillRelease) {
		return nil
	}
	return s.installSkill(version, skillsDir, extensionSkill)
}

// installOpencodeSkills is installSkills for opencode. opencode 1 lists every
// skill that has a description to the agent and ignores the frontmatter that
// hides one, so the extension skill's copy goes without its description:
// opencode 1 then leaves it out, and opencode 2, which honours the
// frontmatter, behaves the same either way.
func (s *Service) installOpencodeSkills(version, skillsDir string) error {
	if err := s.installSkills(version, skillsDir); err != nil {
		return err
	}
	if semver.Less(version, ExtensionSkillRelease) {
		return nil
	}
	return dropDescription(filepath.Join(skillsDir, extensionSkill.name, skillManifest))
}

// dropDescription removes the description line from the frontmatter of the
// SKILL.md at path.
func dropDescription(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	for i := 1; i < len(lines) && lines[0] == "---\n" && lines[i] != "---\n"; i++ {
		if strings.HasPrefix(lines[i], "description:") {
			lines = slices.Delete(lines, i, i+1)
			break
		}
	}
	return writeFile(path, []byte(strings.Join(lines, "")), 0o644)
}

// installSkill writes the release's skill into skillsDir, renamed to its name
// in its frontmatter too: the Agent Skills format wants the name to match the
// directory, and opencode makes that name the slash command.
func (s *Service) installSkill(version, skillsDir string, skill pluginSkill) error {
	files := make(map[string][]byte, len(skill.files))
	for _, name := range skill.files {
		data, err := s.fetchFile(version, skill.source+"/"+name)
		if err != nil {
			return err
		}
		files[name] = data
	}
	files[skillManifest] = bytes.Replace(files[skillManifest],
		[]byte("\nname: "+skill.plain+"\n"), []byte("\nname: "+skill.name+"\n"), 1)
	dir := filepath.Join(skillsDir, skill.name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for name, data := range files {
		if err := writeFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
