package agentplugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

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

// extensionSkill is manual only: its frontmatter keeps it out of the agent's
// context on opencode, oh-my-pi and Crush. Kiro has no such setting, so
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
