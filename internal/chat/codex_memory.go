package chat

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// A Codex chat's memory and subagents, as measured on 0.170.0: its TUI has
// neither screen. thread/start and thread/resume answer instructionSources,
// the AGENTS.md files the thread loaded (the one in CODEX_HOME, then one per
// folder from the project's root down to the checkout, AGENTS.override.md
// before AGENTS.md); config/read's agents table holds the roles
// config.toml declares, each with its description and the file it runs on.
// Codex has no memory setting lich can change, its built-in roles are not
// listed, and a role is declared in config.toml, which lich does not write.

// codexAgentKeys are the agents table's own settings, not roles.
var codexAgentKeys = []string{"max_threads", "max_depth", "job_max_runtime_seconds", "interrupt_message"}

func (d *codexDriver) memory(context.Context) (api.ChatMemoryList, error) {
	d.mu.Lock()
	sources := slices.Clone(d.sources)
	d.mu.Unlock()
	home := d.codexHome()
	list := api.ChatMemoryList{Reported: true}
	for _, path := range sources {
		kind := api.MemoryOther
		if home != "" && filepath.Dir(path) == filepath.Clean(home) {
			kind = api.MemoryUser
		}
		list.Files = append(list.Files, api.ChatMemoryFile{Kind: kind, Path: path, Exists: true})
	}
	return list, nil
}

func (d *codexDriver) setMemory(context.Context, string, bool) error { return errNoMemorySetting }

func (d *codexDriver) agents(ctx context.Context) (api.ChatAgentList, error) {
	raw, err := d.call(ctx, "config/read", map[string]any{})
	if err != nil {
		return api.ChatAgentList{}, err
	}
	var r struct {
		Config struct {
			Agents map[string]json.RawMessage `json:"agents"`
		} `json:"config"`
	}
	_ = json.Unmarshal(raw, &r)
	list := api.ChatAgentList{Reported: true}
	for name, v := range r.Config.Agents {
		var role struct {
			Description string `json:"description"`
			ConfigFile  string `json:"config_file"`
		}
		if slices.Contains(codexAgentKeys, name) || json.Unmarshal(v, &role) != nil {
			continue
		}
		list.Agents = append(list.Agents, api.ChatAgent{Name: name, Description: role.Description, Source: api.AgentOther, Path: role.ConfigFile})
	}
	slices.SortFunc(list.Agents, func(a, b api.ChatAgent) int { return strings.Compare(a.Name, b.Name) })
	return list, nil
}

func (d *codexDriver) agentDir(string) (string, error) { return "", errAgentScope }
