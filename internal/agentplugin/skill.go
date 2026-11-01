package agentplugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// The plugin's theme skill, for the harnesses that do not load it from the
// plugin itself. Claude Code, Codex and Antigravity read skills/ out of the
// installed plugin, and Cursor loads Claude Code's installed plugins whole;
// opencode, oh-my-pi, Crush and Kiro only read a skills directory of their own,
// so their install copies the skill there.

const (
	themeSkillSource = "skills/theme"
	// themeSkillName is the skill's name and directory once copied out of the
	// plugin. It loses the plugin's namespace there, and a bare `theme` would sit
	// beside the user's own skills under a name one of them may already hold.
	themeSkillName = "lich-theme"
	skillManifest  = "SKILL.md"
)

// themeSkillFiles is every file the skill ships: its SKILL.md tells the agent
// to copy the template and run the validator from the skill's own directory.
var themeSkillFiles = []string{skillManifest, "template.json", "validate.mjs"}

// installThemeSkill writes the release's theme skill into skillsDir, renamed to
// themeSkillName in its frontmatter too: the Agent Skills format wants the name
// to match the directory, and opencode makes that name the slash command.
func (s *Service) installThemeSkill(version, skillsDir string) error {
	files := make(map[string][]byte, len(themeSkillFiles))
	for _, name := range themeSkillFiles {
		data, err := s.fetchFile(version, themeSkillSource+"/"+name)
		if err != nil {
			return err
		}
		files[name] = data
	}
	files[skillManifest] = bytes.Replace(files[skillManifest],
		[]byte("\nname: theme\n"), []byte("\nname: "+themeSkillName+"\n"), 1)
	dir := filepath.Join(skillsDir, themeSkillName)
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
