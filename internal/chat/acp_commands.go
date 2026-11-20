package chat

import (
	"encoding/json"

	"github.com/lichdotdev/lich/api"
)

// kiroCommands is the notification Kiro CLI lists its commands in, in place
// of ACP's available_commands_update (measured on 2.21.0). Its names carry
// the slash; a command it marks local runs only in its TUI.
const kiroCommands = "_kiro.dev/commands/available"

// commandsAvailable reads ACP's available_commands_update (opencode, oh-my-pi
// and Antigravity send it once a session opens).
func (d *acpDriver) commandsAvailable(update json.RawMessage) {
	var u struct {
		Commands []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Input       *struct {
				Hint string `json:"hint"`
			} `json:"input"`
		} `json:"availableCommands"`
	}
	if json.Unmarshal(update, &u) != nil {
		return
	}
	cmds := make([]api.ChatCommand, 0, len(u.Commands))
	for _, c := range u.Commands {
		if c.Name == "" {
			continue
		}
		cmd := api.ChatCommand{Text: slashed(c.Name), Description: c.Description}
		if c.Input != nil {
			cmd.Hint = c.Input.Hint
		}
		cmds = append(cmds, cmd)
	}
	d.c.setCommands(cmds)
}

// kiroCommandsAvailable reads Kiro's own list.
func (d *acpDriver) kiroCommandsAvailable(params json.RawMessage) {
	var p struct {
		Commands []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Meta        struct {
				Hint  string `json:"hint"`
				Local bool   `json:"local"`
			} `json:"meta"`
		} `json:"commands"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	cmds := make([]api.ChatCommand, 0, len(p.Commands))
	for _, c := range p.Commands {
		if c.Name == "" || c.Meta.Local {
			continue
		}
		cmds = append(cmds, api.ChatCommand{Text: slashed(c.Name), Description: c.Description, Hint: c.Meta.Hint})
	}
	d.c.setCommands(cmds)
}
