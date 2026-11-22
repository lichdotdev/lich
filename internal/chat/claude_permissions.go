package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// A Claude Code chat's permission rules, as measured on 2.1.330:
// list_permission_rules gives what its /permissions lists, every rule with its
// source and the folders it may work in. Its stream-json has no request that
// changes a rule; `claude edit-permission-rules --json` does, the writer its
// VS Code extension runs, which checks the rule and refuses one that holds a
// secret or would land in a file git tracks. The running chat watches its
// settings files and takes the change about a second later, and in a git
// worktree its local settings are the main checkout's.

// claudePermissionsCommand is the CLI that adds or removes a rule.
var claudePermissionsCommand = []string{"edit-permission-rules", "--json"}

// claudeRuleFiles are the settings files a rule is added to, in the order
// the TUI offers them.
var claudeRuleFiles = []string{api.SettingsLocal, api.SettingsProject, api.SettingsUser}

// claudeSettleWait bounds how long an edit waits for the running chat to
// list it, and claudeSettleStep how often it looks.
const (
	claudeSettleWait = 4 * time.Second
	claudeSettleStep = 200 * time.Millisecond
)

// claudeSourceNames is lich's name of each of Claude Code's sources; one it
// does not name is api.SettingsOther.
var claudeSourceNames = map[string]string{
	"userSettings":    api.SettingsUser,
	"projectSettings": api.SettingsProject,
	"localSettings":   api.SettingsLocal,
	"policySettings":  api.SettingsManaged,
	"flagSettings":    api.SettingsFlag,
	"cliArg":          api.SettingsFlag,
	"session":         api.SettingsSession,
	"command":         api.SettingsSession,
	"sessionHook":     api.SettingsSession,
	"pluginHook":      api.SettingsPlugin,
}

func claudeSource(s string) string {
	if n, ok := claudeSourceNames[s]; ok {
		return n
	}
	return api.SettingsOther
}

// claudeFileNames is Claude Code's name of each of claudeRuleFiles.
var claudeFileNames = map[string]string{
	api.SettingsLocal: "localSettings", api.SettingsProject: "projectSettings", api.SettingsUser: "userSettings",
}

type claudeRule struct {
	Behavior    string `json:"behavior"`
	Source      string `json:"source"`
	Rule        string `json:"rule"`
	Editability string `json:"editability"`
	NotInEffect bool   `json:"notInEffect"`
	Description *struct {
		Prefix   string `json:"prefix"`
		Emphasis string `json:"emphasis"`
		Suffix   string `json:"suffix"`
	} `json:"description"`
}

type claudeSettingsError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type claudeRuleState struct {
	Rules   []claudeRule `json:"rules"`
	Folders []struct {
		Path   string `json:"path"`
		Source string `json:"source"`
	} `json:"workspaceDirectories"`
	Cwd         string                `json:"originalCwd"`
	ManagedOnly bool                  `json:"managedOnly"`
	Errors      []claudeSettingsError `json:"errors"`
}

func (d *claudeDriver) permissions(ctx context.Context) (api.ChatPermissionList, error) {
	r, err := d.control(ctx, map[string]any{"subtype": "list_permission_rules"})
	if err != nil {
		return api.ChatPermissionList{}, err
	}
	var body struct {
		State *claudeRuleState `json:"state"`
	}
	if json.Unmarshal(r.Body, &body) != nil || body.State == nil {
		return api.ChatPermissionList{}, nil
	}
	return claudePermissionList(*body.State), nil
}

func claudePermissionList(st claudeRuleState) api.ChatPermissionList {
	list := api.ChatPermissionList{Reported: true, Cwd: st.Cwd, Locked: st.ManagedOnly, Errors: claudeErrors(st.Errors)}
	if !st.ManagedOnly {
		list.Destinations = claudeRuleFiles
	}
	for _, r := range st.Rules {
		rule := api.ChatPermissionRule{
			Behavior: r.Behavior, Rule: r.Rule, Source: claudeSource(r.Source), Ignored: r.NotInEffect,
			Removable: r.Editability == "persistent" && !r.NotInEffect && !st.ManagedOnly,
		}
		if r.Description != nil {
			rule.Description = r.Description.Prefix + r.Description.Emphasis + r.Description.Suffix
		}
		list.Rules = append(list.Rules, rule)
	}
	for _, f := range st.Folders {
		list.Folders = append(list.Folders, api.ChatFolder{Path: f.Path, Source: claudeSource(f.Source)})
	}
	return list
}

func claudeErrors(errs []claudeSettingsError) []string {
	var out []string
	for _, e := range errs {
		out = append(out, strings.TrimPrefix(e.Path+": "+e.Message, ": "))
	}
	return out
}

// claudeRuleReply is what edit-permission-rules prints.
type claudeRuleReply struct {
	OK       bool     `json:"ok"`
	Error    string   `json:"error"`
	Warnings []string `json:"warnings"`
	Stored   []string `json:"stored"`
}

// permissionAct writes the edit through the CLI, then waits for the running
// chat to list it: a panel read right after would show the rule as it was.
func (d *claudeDriver) permissionAct(ctx context.Context, opts api.ChatPermissionActOptions) (api.ChatPermissionActResult, error) {
	source := claudeFileNames[opts.Source]
	edit := map[string]any{"op": opts.Action, "behavior": opts.Behavior}
	if opts.Action == api.PermissionAdd {
		edit["rules"], edit["destination"] = []string{opts.Rule}, source
	} else {
		edit["rule"], edit["source"] = opts.Rule, source
	}
	reply, err := d.editRules(ctx, edit)
	if err != nil {
		return api.ChatPermissionActResult{}, err
	}
	rule := opts.Rule
	if len(reply.Stored) > 0 {
		rule = reply.Stored[0]
	}
	d.settle(ctx, opts.Action == api.PermissionAdd, func(r api.ChatPermissionRule) bool {
		return r.Rule == rule && r.Behavior == opts.Behavior && r.Source == opts.Source
	})
	return api.ChatPermissionActResult{Warnings: reply.Warnings}, nil
}

func (d *claudeDriver) editRules(ctx context.Context, edit map[string]any) (claudeRuleReply, error) {
	cmd, err := d.cli(ctx, claudePermissionsCommand...)
	if err != nil {
		return claudeRuleReply{}, err
	}
	in, _ := json.Marshal(edit)
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	var reply claudeRuleReply
	// A refusal prints its reason alone on stderr, measured on 2.1.330.
	if json.Unmarshal(out, &reply) != nil {
		if why := strings.TrimSpace(stderr.String()); why != "" {
			return reply, errors.New(why)
		}
		return reply, fmt.Errorf("claude edit-permission-rules: %w", runErr)
	}
	if !reply.OK {
		return reply, errors.New(reply.Error)
	}
	return reply, nil
}

// settle waits, up to claudeSettleWait, until the chat lists a rule match
// picks when want, or lists none. A chat that is late keeps the edit, which
// is on disk: the next read shows it.
func (d *claudeDriver) settle(ctx context.Context, want bool, match func(api.ChatPermissionRule) bool) {
	ctx, cancel := context.WithTimeout(ctx, claudeSettleWait)
	defer cancel()
	for {
		list, err := d.permissions(ctx)
		if err != nil || slices.ContainsFunc(list.Rules, match) == want {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(claudeSettleStep):
		}
	}
}
