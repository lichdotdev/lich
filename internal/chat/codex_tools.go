package chat

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/codexprofile"
)

// A Codex chat's MCP servers and skills, as measured on 0.170.0:
// mcpServerStatus/list gives each server's tools and sign-in, config/read the
// servers config.toml declares and whether each is on. Turning one off is
// config/value/write of mcp_servers.<name>.enabled then config/mcpServer/reload:
// its tools drop from the list and come back, and runtimeStatus stays null
// either way, with a thread open or not, so config/read is what says a server
// is off. A skill is turned off by skills/config/write of its path.

// codexMCP is one server mcpServerStatus/list gives.
type codexMCP struct {
	Name       string `json:"name"`
	Auth       string `json:"authStatus"`
	Runtime    string `json:"runtimeStatus"`
	ToolsError string `json:"toolsError"`
	PluginID   string `json:"pluginId"`
	ServerInfo *struct {
		Version string `json:"version"`
	} `json:"serverInfo"`
	Tools map[string]api.ChatMCPTool `json:"tools"`
}

// codexMCPConfig is a server as config/read gives it, read for its url or
// command only: its env and headers hold secrets.
type codexMCPConfig struct {
	URL     string   `json:"url"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled *bool    `json:"enabled"`
}

// codexRuntime is the window's status for each runtimeStatus.
var codexRuntime = map[string]string{
	"connected":              api.MCPConnected,
	"starting":               api.MCPPending,
	"notStarted":             api.MCPPending,
	"authenticationRequired": api.MCPNeedsAuth,
	"failed":                 api.MCPFailed,
	"cancelled":              api.MCPFailed,
	"disabled":               api.MCPDisabled,
}

// codexAuth is the window's sign-in state for each authStatus.
var codexAuth = map[string]string{
	"notLoggedIn": api.MCPNotSignedIn,
	"oAuth":       api.MCPSignedIn,
	"bearerToken": api.MCPSignedIn,
}

func (d *codexDriver) mcpServers(ctx context.Context) (api.ChatMCPList, error) {
	var servers []codexMCP
	for cursor := ""; ; {
		params := map[string]any{"detail": "full"}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := d.call(ctx, "mcpServerStatus/list", params)
		if err != nil {
			return api.ChatMCPList{}, err
		}
		var page struct {
			Data []codexMCP `json:"data"`
			Next string     `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return api.ChatMCPList{}, err
		}
		servers = append(servers, page.Data...)
		if cursor = page.Next; cursor == "" {
			break
		}
	}
	configs := d.mcpConfigs(ctx)
	file := filepath.Join(d.codexHome(), "config.toml")
	list := api.ChatMCPList{Reported: true, Actions: mcpActions}
	for _, m := range servers {
		list.Servers = append(list.Servers, codexServer(m, configs, file))
	}
	return list, nil
}

// mcpConfigs is the servers config/read declares, none when it fails: the
// list still shows each server, without its target or whether it is off.
func (d *codexDriver) mcpConfigs(ctx context.Context) map[string]codexMCPConfig {
	raw, err := d.call(ctx, "config/read", map[string]any{"cwd": d.c.cwd})
	if err != nil {
		return nil
	}
	var r struct {
		Config struct {
			Servers map[string]codexMCPConfig `json:"mcp_servers"`
		} `json:"config"`
	}
	_ = json.Unmarshal(raw, &r)
	return r.Config.Servers
}

// codexHome is the CODEX_HOME the chat runs in.
func (d *codexDriver) codexHome() string {
	if dir := d.c.proc.ProfileDir; dir != "" {
		return dir
	}
	dir, _ := codexprofile.PersonalDir()
	return dir
}

// codexServer is m as the window shows it. A server config.toml does not
// declare (codex_apps, a plugin's) has no file to name.
func codexServer(m codexMCP, configs map[string]codexMCPConfig, file string) api.ChatMCPServer {
	out := api.ChatMCPServer{Name: m.Name, Error: m.ToolsError, Auth: codexAuth[m.Auth], Fixed: fixedMCP(m.Name)}
	if m.ServerInfo != nil {
		out.Version = m.ServerInfo.Version
	}
	for name, t := range m.Tools {
		out.Tools = append(out.Tools, api.ChatMCPTool{Name: cmp.Or(t.Name, name), Description: t.Description})
	}
	slices.SortFunc(out.Tools, func(a, b api.ChatMCPTool) int { return strings.Compare(a.Name, b.Name) })
	out.Status = codexRuntime[m.Runtime]
	if out.Status == "" {
		out.Status = api.MCPPending
		if len(out.Tools) > 0 {
			out.Status = api.MCPConnected
		}
	}
	if m.PluginID != "" {
		out.Scope = "plugin"
	}
	cfg, ok := configs[m.Name]
	if !ok {
		return out
	}
	// lich's own comes from the -c flags of its spawn, not from the file.
	if m.PluginID == "" && !out.Fixed {
		out.Scope, out.Config = "user", file
	}
	if cfg.Enabled != nil && !*cfg.Enabled {
		out.Status = api.MCPDisabled
	}
	switch {
	case cfg.URL != "":
		out.Transport, out.Target = "http", cfg.URL
	case cfg.Command != "":
		out.Transport, out.Target = "stdio", strings.Join(append([]string{cfg.Command}, cfg.Args...), " ")
	}
	return out
}

func (d *codexDriver) mcpAct(ctx context.Context, server, action string) (api.ChatMCPActResult, error) {
	switch action {
	case api.MCPEnable, api.MCPDisable:
		if _, err := d.call(ctx, "config/value/write", map[string]any{
			"keyPath": "mcp_servers." + server + ".enabled", "value": action == api.MCPEnable, "mergeStrategy": "upsert",
		}); err != nil {
			return api.ChatMCPActResult{}, err
		}
		_, err := d.call(ctx, "config/mcpServer/reload", nil)
		return api.ChatMCPActResult{}, err
	case api.MCPReconnect:
		_, err := d.call(ctx, "config/mcpServer/reload", nil)
		return api.ChatMCPActResult{}, err
	}
	// Built from the schema of 0.170.0, never called against a real server.
	raw, err := d.call(ctx, "mcpServer/oauth/login", map[string]any{"name": server})
	if err != nil {
		return api.ChatMCPActResult{}, err
	}
	var login struct {
		URL string `json:"authorizationUrl"`
	}
	_ = json.Unmarshal(raw, &login)
	return api.ChatMCPActResult{AuthURL: login.URL}, nil
}

func (d *codexDriver) skillList(ctx context.Context) (api.ChatSkillList, error) {
	skills, err := d.readSkills(ctx)
	if err != nil {
		return api.ChatSkillList{}, err
	}
	list := api.ChatSkillList{Reported: true}
	seen := map[string]bool{}
	for _, s := range skills {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		state := api.SkillOff
		if s.Enabled {
			state = api.SkillOn
		}
		list.Skills = append(list.Skills, api.ChatSkill{Name: s.Name, Description: s.summary(), Source: s.Scope, State: state})
	}
	return list, nil
}

// setSkill writes the skill's path, the one skills/list gives: skills/config/write
// takes a name it does not know and writes it all the same.
func (d *codexDriver) setSkill(ctx context.Context, name string, enabled bool) error {
	skills, err := d.readSkills(ctx)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(skills, func(s codexSkill) bool { return s.Name == name })
	if i < 0 {
		return fmt.Errorf("this chat has no skill %q", name)
	}
	if _, err := d.call(ctx, "skills/config/write", map[string]any{"path": skills[i].Path, "enabled": enabled}); err != nil {
		return err
	}
	d.listSkills(ctx)
	return nil
}
