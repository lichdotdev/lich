package chat

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// A Claude Code chat's plugins, as measured on 2.1.330: its stream-json has
// no request that lists or changes them, so lich runs the CLI. `claude plugin
// list --json --available` lists the installed ones, of every project, and
// those the marketplaces offer that the checkout has not installed; `plugin
// details` is the only one that says what a plugin brings and costs, in text
// alone, for an installed one only. Every change answers one --json line, and
// reload_plugins then has the running chat take it, answering its new slash
// list. An update says it applies on a restart.

// claudePluginScopes are where Claude Code installs a plugin, its default
// first.
var claudePluginScopes = []string{api.PluginScopeUser, api.PluginScopeProject, api.PluginScopeLocal}

var claudePluginActions = []string{
	api.PluginInstall, api.PluginUninstall, api.PluginEnable, api.PluginDisable, api.PluginUpdate,
	api.MarketplaceAdd, api.MarketplaceUpdate, api.MarketplaceRemove,
}

// claudeDetailsAtOnce bounds the `plugin details` run together: each is a
// whole Claude Code start, about a second.
const claudeDetailsAtOnce = 4

// claudeInGoalState is the failure code of a change already made, which
// lich takes as done.
const claudeInGoalState = "already_in_goal_state"

type claudeInstalled struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	Scope         string `json:"scope"`
	Enabled       bool   `json:"enabled"`
	InstallPath   string `json:"installPath"`
	ProjectPath   string `json:"projectPath"`
	FolderVersion string `json:"folderVersion"`
}

type claudeAvailable struct {
	ID          string `json:"pluginId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Marketplace string `json:"marketplaceName"`
	Version     string `json:"version"`
	Installs    int    `json:"installCount"`
}

type claudeMarketplace struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
	URL  string `json:"url"`
	Path string `json:"path"`
}

func (d *claudeDriver) plugins(ctx context.Context) (api.ChatPluginList, error) {
	var listed struct {
		Installed []claudeInstalled `json:"installed"`
		Available []claudeAvailable `json:"available"`
	}
	if err := d.cliJSON(ctx, &listed, "plugin", "list", "--json", "--available"); err != nil {
		return api.ChatPluginList{}, err
	}
	var markets []claudeMarketplace
	if err := d.cliJSON(ctx, &markets, "plugin", "marketplace", "list", "--json"); err != nil {
		return api.ChatPluginList{}, err
	}
	list := api.ChatPluginList{Reported: true, Actions: claudePluginActions, Scopes: claudePluginScopes}
	var paths []string
	for _, in := range listed.Installed {
		if in.ProjectPath != "" && in.ProjectPath != d.c.cwd {
			continue
		}
		list.Plugins = append(list.Plugins, claudePlugin(in))
		paths = append(paths, in.InstallPath)
	}
	details := d.pluginDetailsOf(ctx, list.Plugins, paths)
	for i, det := range details {
		list.Plugins[i].Parts, list.Plugins[i].Description = det.parts, det.description
	}
	for _, a := range listed.Available {
		list.Plugins = append(list.Plugins, api.ChatPlugin{
			ID: a.ID, Name: a.Name, Marketplace: a.Marketplace, Description: a.Description, Version: a.Version,
			Installs: a.Installs, Fixed: fixedPlugin(a.ID),
		})
	}
	counts := pluginMarketplaceCounts(list.Plugins)
	for _, m := range markets {
		list.Marketplaces = append(list.Marketplaces, api.ChatMarketplace{
			Name: m.Name, Source: cmp.Or(m.Repo, m.URL, m.Path), Plugins: counts[m.Name], Fixed: m.Name == lichMarketplace,
		})
	}
	return list, nil
}

// claudePlugin is an installed plugin. A newer version is known only for one
// read from a folder, whose version the list gives beside the installed one.
func claudePlugin(in claudeInstalled) api.ChatPlugin {
	name, market, _ := strings.Cut(in.ID, "@")
	p := api.ChatPlugin{
		ID: in.ID, Name: name, Marketplace: market, Version: in.Version, Installed: true, Enabled: in.Enabled,
		Scope: in.Scope, Fixed: fixedPlugin(in.ID),
	}
	if in.FolderVersion != "" && in.FolderVersion != in.Version {
		p.Latest = in.FolderVersion
	}
	return p
}

// claudeDetails is what `claude plugin details` says of an installed plugin:
// the list gives neither its description nor what it brings.
type claudeDetails struct {
	description string
	parts       *api.ChatPluginParts
}

// pluginDetailsOf is the details of each of plugins, read once per install
// path.
func (d *claudeDriver) pluginDetailsOf(ctx context.Context, plugins []api.ChatPlugin, paths []string) []claudeDetails {
	out := make([]claudeDetails, len(plugins))
	slots := make(chan struct{}, claudeDetailsAtOnce)
	var wg sync.WaitGroup
	for i, p := range plugins {
		d.mu.Lock()
		known, ok := d.details[paths[i]]
		d.mu.Unlock()
		if ok {
			out[i] = known
			continue
		}
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			det := d.pluginDetails(ctx, p.ID)
			out[i] = det
			d.mu.Lock()
			if d.details == nil {
				d.details = map[string]claudeDetails{}
			}
			d.details[paths[i]] = det
			d.mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

var (
	claudeDescLine   = regexp.MustCompile(`(?m)^\s*Description: (.+)$`)
	claudePartCount  = regexp.MustCompile(`(?m)^\s*(Skills|Agents|Hooks|MCP servers) \((\d+)\)`)
	claudeAlwaysCost = regexp.MustCompile(`Always-on:\s*(?:~|<\s*)?([\d.]+)(k?)\s*tok`)
)

// pluginDetails reads what `claude plugin details` prints, nothing when it
// prints nothing lich can read: it has no --json, so a later Claude Code may
// word it otherwise.
func (d *claudeDriver) pluginDetails(ctx context.Context, id string) claudeDetails {
	cmd, err := d.cli(ctx, "plugin", "details", id)
	if err != nil {
		return claudeDetails{}
	}
	out, err := cmd.Output()
	if err != nil {
		return claudeDetails{}
	}
	det := claudeDetails{parts: parseClaudeParts(string(out))}
	if m := claudeDescLine.FindStringSubmatch(string(out)); m != nil {
		det.description = strings.TrimSpace(m[1])
	}
	return det
}

func parseClaudeParts(text string) *api.ChatPluginParts {
	counts := claudePartCount.FindAllStringSubmatch(text, -1)
	if len(counts) == 0 {
		return nil
	}
	var parts api.ChatPluginParts
	for _, m := range counts {
		n, _ := strconv.Atoi(m[2])
		switch m[1] {
		case "Skills":
			parts.Skills = n
		case "Agents":
			parts.Agents = n
		case "Hooks":
			parts.Hooks = n
		default:
			parts.MCP = n
		}
	}
	if m := claudeAlwaysCost.FindStringSubmatch(text); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		if m[2] == "k" {
			f *= 1000
		}
		parts.Tokens = int(f)
	}
	return &parts
}

// cliJSON runs the CLI and decodes what it prints into v.
func (d *claudeDriver) cliJSON(ctx context.Context, v any, args ...string) error {
	cmd, err := d.cli(ctx, args...)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		return fmt.Errorf("claude %s: %w: %s", strings.Join(args, " "), runErr, strings.TrimSpace(stderr.String()))
	}
	return json.Unmarshal(out, v)
}

// claudePluginArgs is the CLI line of each action. A scope is passed where
// the plugin has one: uninstall refuses a plugin installed elsewhere than
// the one it is given. Never -y: it would run a command a marketplace
// declares without the person seeing it, so such an install is refused.
func claudePluginArgs(opts api.ChatPluginActOptions) []string {
	args := map[string][]string{
		api.PluginInstall:     {"plugin", "install"},
		api.PluginUninstall:   {"plugin", "uninstall"},
		api.PluginEnable:      {"plugin", "enable"},
		api.PluginDisable:     {"plugin", "disable"},
		api.PluginUpdate:      {"plugin", "update"},
		api.MarketplaceAdd:    {"plugin", "marketplace", "add"},
		api.MarketplaceUpdate: {"plugin", "marketplace", "update"},
		api.MarketplaceRemove: {"plugin", "marketplace", "remove"},
	}[opts.Action]
	args = append(slices.Clone(args), opts.Target, "--json")
	if opts.Scope != "" && !strings.HasPrefix(opts.Action, "marketplace-") {
		args = append(args, "--scope", opts.Scope)
	}
	return args
}

// claudePluginReply is the line every change prints last.
type claudePluginReply struct {
	Outcome       string `json:"outcome"`
	Message       string `json:"message"`
	FailureCode   string `json:"failureCode"`
	UpdateOutcome string `json:"updateOutcome"`
}

func (d *claudeDriver) pluginAct(ctx context.Context, opts api.ChatPluginActOptions) (api.ChatPluginActResult, error) {
	reply, err := d.pluginChange(ctx, claudePluginArgs(opts))
	if err != nil {
		return api.ChatPluginActResult{}, err
	}
	d.reloadPlugins(ctx)
	return api.ChatPluginActResult{Restart: reply.UpdateOutcome == "updated", Current: reply.UpdateOutcome == "up_to_date"}, nil
}

func (d *claudeDriver) pluginChange(ctx context.Context, args []string) (claudePluginReply, error) {
	cmd, err := d.cli(ctx, args...)
	if err != nil {
		return claudePluginReply{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var reply claudePluginReply
	if json.Unmarshal([]byte(lines[len(lines)-1]), &reply) != nil {
		return reply, errors.Join(fmt.Errorf("claude %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String())), runErr)
	}
	if reply.Outcome != "ok" && reply.FailureCode != claudeInGoalState {
		return reply, errors.New(reply.Message)
	}
	return reply, nil
}

// reloadPlugins has the running chat take the change and republishes its
// slash list. The change is on disk by then: a chat that refuses, busy with a
// turn, says so in the conversation instead of failing it.
func (d *claudeDriver) reloadPlugins(ctx context.Context) {
	r, err := d.control(ctx, map[string]any{"subtype": "reload_plugins"})
	if err != nil {
		d.c.conv.notice("Claude Code did not reload its plugins, a restart of the chat applies the change: " + err.Error())
		return
	}
	var body struct {
		Commands *[]claudeCommand `json:"commands"`
	}
	if json.Unmarshal(r.Body, &body) == nil && body.Commands != nil {
		d.listCommands(*body.Commands)
	}
}
