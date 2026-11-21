package chat

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// codexSkillSigil starts a skill typed at Codex's prompt, "$skill-name": its
// TUI's spelling (0.170.0). Codex lists no commands over app-server, its
// slash commands being the TUI's own.
const codexSkillSigil = "$"

// codexMention is a skill named in a message, its name the submatch.
var codexMention = regexp.MustCompile(`\$([\w.:-]+)`)

// codexSkill is one skill skills/list gives (schema of 0.170.0).
type codexSkill struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	ShortDescription string `json:"shortDescription"`
	Interface        *struct {
		ShortDescription string `json:"shortDescription"`
	} `json:"interface"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// summary is the skill's short description where it has one.
func (s codexSkill) summary() string {
	switch {
	case s.Interface != nil && s.Interface.ShortDescription != "":
		return s.Interface.ShortDescription
	case s.ShortDescription != "":
		return s.ShortDescription
	}
	return s.Description
}

// listSkills reads the skills the chat's checkout has into its commands. A
// failed read leaves the list as it was: the prompt takes text either way.
func (d *codexDriver) listSkills(ctx context.Context) {
	raw, err := d.call(ctx, "skills/list", map[string]any{"cwds": []string{d.c.cwd}})
	if err != nil {
		return
	}
	var r struct {
		Data []struct {
			Skills []codexSkill `json:"skills"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return
	}
	paths := map[string]string{}
	var cmds []api.ChatCommand
	for _, entry := range r.Data {
		for _, s := range entry.Skills {
			if !s.Enabled || s.Name == "" || paths[s.Name] != "" {
				continue
			}
			paths[s.Name] = s.Path
			cmds = append(cmds, api.ChatCommand{Text: codexSkillSigil + s.Name, Description: s.summary(), Skill: true})
		}
	}
	d.mu.Lock()
	d.skills = paths
	d.mu.Unlock()
	d.c.setCommands(cmds)
}

// input is a message as turn/start and turn/steer take it: its text, an
// image item for each image, and a skill item for each skill it names, which
// is how app-server's schema hands one to the turn.
func (d *codexDriver) input(m message) []map[string]any {
	text := m.text
	var in []map[string]any
	if text != "" || len(m.images) == 0 {
		in = textInput(text)
	}
	// app-server reads the file itself: turn/start's and turn/steer's
	// LocalImageUserInput (0.170.0 schema).
	for _, img := range m.images {
		in = append(in, map[string]any{"type": "localImage", "path": img.path})
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	seen := map[string]bool{}
	for _, m := range codexMention.FindAllStringSubmatch(text, -1) {
		// A mention ending a sentence keeps its full stop out of the name.
		name := strings.TrimRight(m[1], ".:-")
		if path, ok := d.skills[name]; ok && !seen[name] {
			seen[name] = true
			in = append(in, map[string]any{"type": "skill", "name": name, "path": path})
		}
	}
	return in
}
