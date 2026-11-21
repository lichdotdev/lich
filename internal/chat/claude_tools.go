package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
)

// A Claude Code chat's MCP servers and skills, as measured on 2.1.330:
// mcp_status lists the servers with their tools, mcp_toggle turns one off or on
// in the project (projects[cwd].disabledMcpServers of the profile's
// .claude.json), mcp_reconnect and mcp_authenticate act on one.
// get_skills_dialog, marked @internal, lists the skills the /skills screen
// shows; turning one off is not a request but `claude edit-skill-overrides`,
// which writes the checkout's .claude/settings.local.json.

// claudeChildEnv makes edit-skill-overrides refuse to run (2.1.330).
const claudeChildEnv = "CLAUDE_CODE_CHILD_SESSION"

// claudeSkillsCommand is the CLI that turns a skill on or off.
var claudeSkillsCommand = []string{"edit-skill-overrides", "--json"}

// claudeAITransport is a claude.ai connector's, which signs in with the
// account.
const claudeAITransport = "claudeai-proxy"

// claudeMCP is one server mcp_status lists.
type claudeMCP struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Error      string `json:"error"`
	Scope      string `json:"scope"`
	Transport  string `json:"transport"`
	ServerInfo *struct {
		Version string `json:"version"`
	} `json:"serverInfo"`
	// Config is read for its url or command only: its env and headers hold
	// secrets the window never sees.
	Config *struct {
		URL     string   `json:"url"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"config"`
	Tools []api.ChatMCPTool `json:"tools"`
}

func (d *claudeDriver) mcpServers(ctx context.Context) (api.ChatMCPList, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "mcp_status"})
	if err != nil {
		return api.ChatMCPList{}, err
	}
	var body struct {
		Servers *[]claudeMCP `json:"mcpServers"`
	}
	if json.Unmarshal(r.Body, &body) != nil || body.Servers == nil {
		return api.ChatMCPList{}, nil
	}
	global, err := claudeprofile.GlobalFile(d.c.proc.ProfileDir)
	if err != nil {
		global = ""
	}
	list := api.ChatMCPList{Reported: true, Actions: mcpActions}
	for _, m := range *body.Servers {
		list.Servers = append(list.Servers, claudeServer(m, global, d.c.cwd))
	}
	return list, nil
}

// claudeServer is m as the window shows it, global being the .claude.json that
// holds the user and local servers.
func claudeServer(m claudeMCP, global, cwd string) api.ChatMCPServer {
	out := api.ChatMCPServer{
		Name: m.Name, Status: m.Status, Scope: m.Scope, Transport: m.Transport,
		Error: m.Error, Fixed: fixedMCP(m.Name), Tools: m.Tools,
	}
	switch m.Scope {
	case "user", "local":
		out.Config = global
	case "project":
		out.Config = filepath.Join(cwd, ".mcp.json")
	}
	if m.ServerInfo != nil {
		out.Version = m.ServerInfo.Version
	}
	if m.Config != nil {
		out.Target = m.Config.URL
		if out.Target == "" && m.Config.Command != "" {
			out.Target = strings.Join(append([]string{m.Config.Command}, m.Config.Args...), " ")
		}
	}
	switch {
	case m.Status == api.MCPNeedsAuth:
		out.Auth = api.MCPNotSignedIn
	case m.Status == api.MCPConnected && (m.Scope == "claudeai" || m.Transport == claudeAITransport):
		out.Auth = api.MCPSignedIn
	}
	return out
}

func (d *claudeDriver) mcpAct(ctx context.Context, server, action string) (api.ChatMCPActResult, error) {
	var err error
	switch action {
	case api.MCPEnable, api.MCPDisable:
		_, err = d.control(ctx, map[string]any{"subtype": "mcp_toggle", "serverName": server, "enabled": action == api.MCPEnable})
	case api.MCPReconnect:
		_, err = d.control(ctx, map[string]any{"subtype": "mcp_reconnect", "serverName": server})
	case api.MCPSignIn:
		return d.mcpSignIn(ctx, server)
	}
	return api.ChatMCPActResult{}, err
}

// mcpSignIn starts a server's sign-in. Built from the schema of 2.1.330, never
// called against a real server: a claude.ai connector answers the page alone,
// a plain OAuth server the page and the localhost port Claude Code waits on
// for the callback. One that waits on no port would need its callback relayed,
// which lich does not do.
func (d *claudeDriver) mcpSignIn(ctx context.Context, server string) (api.ChatMCPActResult, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "mcp_authenticate", "serverName": server})
	if err != nil {
		return api.ChatMCPActResult{}, err
	}
	var auth struct {
		URL              string `json:"authUrl"`
		RequiresAction   bool   `json:"requiresUserAction"`
		CallbackExpected bool   `json:"callbackExpected"`
		CallbackPort     int    `json:"callbackPort"`
	}
	_ = json.Unmarshal(r.Body, &auth)
	if !auth.RequiresAction {
		return api.ChatMCPActResult{}, nil
	}
	if auth.CallbackExpected && auth.CallbackPort == 0 {
		return api.ChatMCPActResult{}, errNoMCPPage
	}
	return api.ChatMCPActResult{AuthURL: auth.URL}, nil
}

// claudeSkill is one skill get_skills_dialog lists.
type claudeSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Tokens      int    `json:"tokens"`
	State       string `json:"state"`
	LockedBy    string `json:"locked_by"`
}

// skillList degrades to not reported on any refusal: the request is
// @internal, so a later Claude Code may drop or reshape it.
func (d *claudeDriver) skillList(ctx context.Context) (api.ChatSkillList, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "get_skills_dialog"})
	if r.Subtype == "error" {
		return api.ChatSkillList{}, nil
	}
	if err != nil {
		return api.ChatSkillList{}, err
	}
	var body struct {
		Skills *[]claudeSkill `json:"skills"`
	}
	if json.Unmarshal(r.Body, &body) != nil || body.Skills == nil {
		return api.ChatSkillList{}, nil
	}
	list := api.ChatSkillList{Reported: true}
	for _, s := range *body.Skills {
		list.Skills = append(list.Skills, api.ChatSkill{
			Name: s.Name, Description: s.Description, Source: s.Source, Tokens: s.Tokens, State: s.State, Locked: s.LockedBy,
		})
	}
	return list, nil
}

// setSkill writes the override through the CLI, then has the process reload
// its skills, whose answer is the slash list without the skill turned off.
func (d *claudeDriver) setSkill(ctx context.Context, name string, enabled bool) error {
	state := api.SkillOff
	if enabled {
		state = api.SkillOn
	}
	if err := d.editSkillOverride(ctx, name, state); err != nil {
		return err
	}
	r, err := d.control(ctx, map[string]any{"subtype": "reload_skills"})
	if err != nil {
		return err
	}
	var body struct {
		Commands *[]claudeCommand `json:"skills"`
	}
	if json.Unmarshal(r.Body, &body) == nil && body.Commands != nil {
		d.listCommands(*body.Commands)
	}
	return nil
}

// cli is the claude CLI run with args in the chat's checkout, without the
// variable some of its commands refuse to run under.
func (d *claudeDriver) cli(ctx context.Context, args ...string) (*exec.Cmd, error) {
	cmd, err := d.c.proc.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	cmd.Dir = d.c.cwd
	cmd.Env = slices.DeleteFunc(cmd.Env, func(kv string) bool { return strings.HasPrefix(kv, claudeChildEnv+"=") })
	return cmd, nil
}

func (d *claudeDriver) editSkillOverride(ctx context.Context, name, state string) error {
	cmd, err := d.cli(ctx, claudeSkillsCommand...)
	if err != nil {
		return err
	}
	in, _ := json.Marshal(map[string]string{"name": name, "state": state})
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	var reply struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(out, &reply) != nil {
		return errors.Join(fmt.Errorf("claude edit-skill-overrides: %s", strings.TrimSpace(stderr.String())), runErr)
	}
	if !reply.OK {
		return fmt.Errorf("claude did not change the skill: %s", reply.Error)
	}
	return nil
}
