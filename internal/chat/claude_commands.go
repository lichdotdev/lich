package chat

import (
	"slices"

	"github.com/lichdotdev/lich/api"
)

// claudeCommand is one command initialize lists (measured on 2.1.329): the
// skills, the plugins' and the user's commands and Claude Code's own, mixed.
type claudeCommand struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ArgumentHint string `json:"argumentHint"`
}

// listCommands takes the commands initialize listed.
func (d *claudeDriver) listCommands(cmds []claudeCommand) {
	d.mu.Lock()
	d.cmds = cmds
	d.mu.Unlock()
	d.publishCommands()
}

// markCommands takes what system/init, written once the first message
// arrives, tells about them: which are skills, and which only the TUI runs,
// which a chat leaves out.
func (d *claudeDriver) markCommands(skills, terminal []string) {
	if skills == nil && terminal == nil {
		return
	}
	d.mu.Lock()
	d.skills, d.terminalOnly = skills, terminal
	d.mu.Unlock()
	d.publishCommands()
}

func (d *claudeDriver) publishCommands() {
	d.mu.Lock()
	cmds, skills, terminal := d.cmds, d.skills, d.terminalOnly
	d.mu.Unlock()
	out := make([]api.ChatCommand, 0, len(cmds))
	for _, c := range cmds {
		if c.Name == "" || slices.Contains(terminal, c.Name) {
			continue
		}
		out = append(out, api.ChatCommand{Text: slashed(c.Name), Description: c.Description, Hint: c.ArgumentHint, Skill: slices.Contains(skills, c.Name)})
	}
	d.c.setCommands(out)
}
