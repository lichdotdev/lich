//go:build !windows

package chat

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) permissions() api.ChatPermissionList {
	r.t.Helper()
	list, err := r.svc.Permissions(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Permissions: %v", err)
	}
	return list
}

func (r *rig) ruleAct(action, behavior, rule, source string) (api.ChatPermissionActResult, error) {
	return r.svc.PermissionAct(r.t.Context(), api.ChatPermissionActOptions{ID: "s1", Action: action, Behavior: behavior, Rule: rule, Source: source})
}

func (r *rig) hooks() api.ChatHookList {
	r.t.Helper()
	list, err := r.svc.Hooks(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Hooks: %v", err)
	}
	return list
}

func hasRule(l api.ChatPermissionList, behavior, rule, source string) bool {
	return slices.ContainsFunc(l.Rules, func(r api.ChatPermissionRule) bool {
		return r.Behavior == behavior && r.Rule == rule && r.Source == source
	})
}

// Claude Code's list_permission_rules is the list: each rule under lich's
// name of its source with its plain reading, removable only where a settings
// file holds it and it applies; the folders; the broken files; the three
// files a rule is added to, in the TUI's order.
func TestClaudePermissionList(t *testing.T) {
	r := newChat(t, providers.Claude)
	list := r.permissions()
	want := []api.ChatPermissionRule{
		{Behavior: "allow", Rule: "Bash(npm run *)", Source: api.SettingsProject, Description: "Any Bash command starting with npm run", Removable: true},
		{Behavior: "ask", Rule: "WebFetch", Source: api.SettingsUser, Description: "Any use of the WebFetch tool", Removable: true},
		{Behavior: "deny", Rule: "Read(./.env)", Source: api.SettingsManaged},
		{Behavior: "allow", Rule: "Edit", Source: api.SettingsFlag},
		{Behavior: "allow", Rule: "Bash(rm *)", Source: api.SettingsLocal, Ignored: true},
	}
	if !list.Reported || !slices.Equal(list.Rules, want) {
		t.Fatalf("rules = %+v", list.Rules)
	}
	folders := []api.ChatFolder{{Path: "/tmp", Source: api.SettingsLocal}, {Path: "/srv", Source: api.SettingsFlag}}
	if !slices.Equal(list.Folders, folders) || list.Cwd != "/repo" || list.Locked {
		t.Errorf("folders = %+v, cwd %q, locked %v", list.Folders, list.Cwd, list.Locked)
	}
	if !slices.Equal(list.Destinations, []string{api.SettingsLocal, api.SettingsProject, api.SettingsUser}) {
		t.Errorf("destinations = %v", list.Destinations)
	}
	if !slices.Equal(list.Errors, []string{"/repo/.claude/settings.json: Invalid JSON"}) {
		t.Errorf("errors = %v", list.Errors)
	}
}

// An organization that allows only its own rules locks the list: no file
// takes a rule and none is removable.
func TestClaudePermissionsManagedOnly(t *testing.T) {
	list := claudePermissionList(claudeRuleState{ManagedOnly: true, Rules: []claudeRule{
		{Behavior: "allow", Source: "userSettings", Rule: "Edit", Editability: "persistent", NotInEffect: true},
	}})
	if !list.Locked || list.Destinations != nil || list.Rules[0].Removable || !list.Rules[0].Ignored {
		t.Errorf("list = %+v", list)
	}
}

// Adding and removing a rule runs edit-permission-rules in the checkout,
// outside a child session, with the edit its VS Code extension sends; the
// call answers once the running chat lists it, with the warnings the CLI
// gave.
func TestClaudePermissionAct(t *testing.T) {
	t.Setenv(claudeChildEnv, "1")
	t.Setenv(fakeSlowRulesEnv, "1")
	r := newChat(t, providers.Claude)
	res, err := r.ruleAct(api.PermissionAdd, api.PermissionDeny, "Bash(*)", api.SettingsLocal)
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(r.home, "proj")
	if !r.logged(`edit-rules {"behavior":"deny","destination":"localSettings","op":"add","rules":["Bash(*)"]} in ` + proj) {
		t.Errorf("the add did not run the CLI in the checkout: %s", r.fakeLog())
	}
	if len(res.Warnings) != 1 {
		t.Errorf("warnings = %v", res.Warnings)
	}
	if !hasRule(r.permissions(), "deny", "Bash", api.SettingsLocal) {
		t.Fatalf("the chat does not list the stored rule once the add answered: %+v", r.permissions().Rules)
	}
	if _, err := r.ruleAct(api.PermissionRemove, api.PermissionAsk, "WebFetch", api.SettingsUser); err != nil {
		t.Fatal(err)
	}
	if !r.logged(`edit-rules {"behavior":"ask","op":"remove","rule":"WebFetch","source":"userSettings"}`) {
		t.Errorf("the remove did not name the rule verbatim: %s", r.fakeLog())
	}
	if hasRule(r.permissions(), "ask", "WebFetch", api.SettingsUser) {
		t.Error("the chat still lists the removed rule once the remove answered")
	}
}

// What the CLI refuses fails the call with its own words, and what no file
// takes never reaches it.
func TestClaudePermissionActRefused(t *testing.T) {
	r := newChat(t, providers.Claude)
	if _, err := r.ruleAct(api.PermissionAdd, api.PermissionAllow, "Bash(curl -H token:abc)", api.SettingsProject); err == nil || err.Error() != "the rule looks like it contains a secret, so it was not saved to settings" {
		t.Errorf("secret err = %v", err)
	}
	for _, opts := range []api.ChatPermissionActOptions{
		{Action: "edit", Behavior: "allow", Rule: "Edit", Source: api.SettingsUser},
		{Action: api.PermissionAdd, Behavior: "maybe", Rule: "Edit", Source: api.SettingsUser},
		{Action: api.PermissionAdd, Behavior: "allow", Rule: "Edit", Source: api.SettingsManaged},
		{Action: api.PermissionAdd, Behavior: "allow", Rule: "Edit\nBash", Source: api.SettingsUser},
		{Action: api.PermissionAdd, Behavior: "allow", Rule: " ", Source: api.SettingsUser},
	} {
		opts.ID = "s1"
		if _, err := r.svc.PermissionAct(t.Context(), opts); err == nil {
			t.Errorf("%+v was taken", opts)
		}
	}
	if r.logged("edit-rules", "Edit") {
		t.Errorf("a refused edit reached the CLI: %s", r.fakeLog())
	}
}

// Claude Code's get_hooks_listing is the list, read only: each hook with its
// event's summary, lich's name of its source and the label its row shows,
// and why none runs.
func TestClaudeHookList(t *testing.T) {
	r := newChat(t, providers.Claude)
	list := r.hooks()
	want := []api.ChatHook{
		{Event: "PreToolUse", Summary: "Before tool execution", Matcher: "Bash", Source: api.SettingsProject, Type: "command", Command: "echo pre", Label: "echo pre", Timeout: 5},
		{Event: "PreToolUse", Summary: "Before tool execution", Source: api.SettingsPlugin, Plugin: "lich", Type: "command", Command: "node hook.js", Label: "Checking", Off: true},
		{Event: "Stop", Summary: "Right before Claude concludes its response", Source: api.SettingsUser, Type: "prompt", Command: "Is it done?", Label: "Is it done?", Condition: "Bash(git *)"},
	}
	if !list.Reported || !slices.Equal(list.Hooks, want) || list.Actions != nil || list.Blocked != api.HooksOff {
		t.Fatalf("list = %+v", list)
	}
	if _, err := r.svc.HookAct(t.Context(), api.ChatHookActOptions{ID: "s1", Key: "k", Action: api.HookDisable}); !errors.Is(err, errClaudeHooksReadOnly) {
		t.Errorf("act err = %v", err)
	}
}

// Codex's hooks/list is the list: its events named as the other providers
// name them, an untrusted or turned off hook off, an organization's managed
// and never asked to be trusted. Trusting writes the hash the person saw,
// turning off enabled false, each into its own hooks.state entry.
func TestCodexHooks(t *testing.T) {
	r := newChat(t, providers.Codex)
	list := r.hooks()
	if !list.Reported || !slices.Equal(list.Actions, codexHookActions) || len(list.Hooks) != 3 || !slices.Equal(list.Errors, []string{"/x/.codex/hooks.json: bad matcher"}) {
		t.Fatalf("list = %+v", list)
	}
	pre, stop, mdm := list.Hooks[0], list.Hooks[1], list.Hooks[2]
	if pre.Event != "PreToolUse" || pre.Matcher != "Bash" || pre.Trust != api.HookUntrusted || !pre.Off || pre.Source != api.SettingsUser || pre.Hash != "sha256:pre" {
		t.Errorf("pre = %+v", pre)
	}
	if !mdm.Managed || mdm.Trust != "" || mdm.Off || mdm.Source != api.SettingsManaged || stop.Source != api.SettingsProject {
		t.Errorf("mdm = %+v, stop = %+v", mdm, stop)
	}
	act := func(key, action, hash string) {
		t.Helper()
		if _, err := r.svc.HookAct(t.Context(), api.ChatHookActOptions{ID: "s1", Key: key, Action: action, Hash: hash}); err != nil {
			t.Fatal(err)
		}
	}
	act(pre.Key, api.HookTrust, pre.Hash)
	act(stop.Key, api.HookDisable, "")
	if !r.logged(`"method":"config/batchWrite"`, `"keyPath":"hooks.state"`, `"mergeStrategy":"upsert"`, `"trusted_hash":"sha256:pre"`, `"reloadUserConfig":true`) {
		t.Errorf("the trust wrote no hooks.state upsert: %s", r.fakeLog())
	}
	after := r.hooks()
	if h := after.Hooks[0]; h.Trust != api.HookTrusted || h.Off {
		t.Errorf("trusted hook = %+v", h)
	}
	if h := after.Hooks[1]; !h.Off || h.Trust != api.HookUntrusted {
		t.Errorf("turned off hook = %+v", h)
	}
	if _, err := r.svc.HookAct(t.Context(), api.ChatHookActOptions{ID: "s1", Key: pre.Key, Action: api.HookTrust}); !errors.Is(err, errHookHash) {
		t.Errorf("a trust without the hash = %v", err)
	}
	if _, err := r.svc.HookAct(t.Context(), api.ChatHookActOptions{ID: "s1", Key: pre.Key, Action: "delete"}); err == nil {
		t.Error("an unknown action was taken")
	}
}

// Codex reports no permission rules: its /permissions is the mode. An ACP
// agent reports neither, and acting on either fails.
func TestPermissionsNotReported(t *testing.T) {
	r := newChat(t, providers.Codex)
	if l := r.permissions(); l.Reported {
		t.Errorf("codex permissions = %+v", l)
	}
	if _, err := r.ruleAct(api.PermissionAdd, api.PermissionAllow, "Edit", api.SettingsUser); !errors.Is(err, errNoPermissions) {
		t.Errorf("codex act = %v", err)
	}
	a := newChat(t, providers.OMP)
	if l := a.hooks(); l.Reported {
		t.Errorf("acp hooks = %+v", l)
	}
	if _, err := a.svc.HookAct(t.Context(), api.ChatHookActOptions{ID: "s1", Key: "k", Action: api.HookTrust}); !errors.Is(err, errNoHooks) {
		t.Errorf("acp act = %v", err)
	}
}
