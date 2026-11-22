package main

import (
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

var toolsRules = api.ChatPermissionList{
	Reported: true,
	Cwd:      "/repo",
	Rules: []api.ChatPermissionRule{
		{Behavior: api.PermissionAllow, Rule: "Bash(npm run *)", Source: api.SettingsProject, Description: "Any Bash command starting with npm run", Removable: true},
		{Behavior: api.PermissionAllow, Rule: "Edit", Source: api.SettingsFlag},
		{Behavior: api.PermissionDeny, Rule: "Read(./.env)", Source: api.SettingsManaged},
	},
	Folders:      []api.ChatFolder{{Path: "/srv/data", Source: api.SettingsLocal}},
	Destinations: []string{api.SettingsLocal, api.SettingsProject, api.SettingsUser},
}

var toolsHooks = api.ChatHookList{
	Reported: true,
	Actions:  []string{api.HookTrust, api.HookEnable, api.HookDisable},
	Hooks: []api.ChatHook{
		{Event: "PreToolUse", Matcher: "Bash", Source: api.SettingsUser, Type: "command", Command: "echo pre", Key: "k1", Hash: "sha256:a", Trust: api.HookUntrusted, Off: true},
		{Event: "Stop", Source: api.SettingsProject, Type: "command", Command: "echo stop", Label: "Stopping", Key: "k2", Trust: api.HookTrusted},
	},
}

// permFrames is command open on a Claude Code chat whose backend answers
// toolsRules and toolsHooks.
func permFrames(t *testing.T, command string) (*window, *frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f, p := toolsFrames(t, "/mcp")
	f.mu.Lock()
	f.answers[api.ChatPermissions.Name] = toolsRules
	f.answers[api.ChatHooks.Name] = toolsHooks
	f.mu.Unlock()
	p.tools.open(command)
	u.fetchTools(p, "s1")
	toolsSettle(r)
	return u, r, f, p
}

// /permissions opens on the allow rules, each with its plain reading and its
// source, and Add a rule first; it never reaches the agent.
func TestPermissionsOpenOnTheAllowRules(t *testing.T) {
	u, r, f, p := toolsFrames(t, "/permissions")
	if u.model.chat.isRunning("s1") || f.callsOf(api.ChatPrompt.Name) != 0 || p.editor.Len() != 0 {
		t.Fatal("/permissions went to the agent")
	}
	if f.callsOf(api.ChatPermissions.Name) == 0 {
		t.Fatal("/permissions read no rules")
	}
	_, r, _, _ = permFrames(t, chatPermissionsCommand)
	if !r.showsAll("Permissions", "3 rules", "Allow", "Ask", "Deny", "Folders", "Add a rule…", "Bash(npm run *)",
		"Any Bash command starting with npm run", "Project", "Launch flag") {
		t.Errorf("the allow rules are not drawn:\n%s", r.drawn())
	}
	if r.shows("Read(./.env)") {
		t.Error("a deny rule is on the allow tab")
	}
}

// Add a rule takes the rule typed and saves it in the file of the row
// picked, Enter on the second row the project's.
func TestARuleIsAddedToTheFilePicked(t *testing.T) {
	_, r, f, p := permFrames(t, chatPermissionsCommand)
	r.press(key.NameReturn)
	if !r.showsAll("New allow rule", "Save in", "This checkout only, not shared", ".claude/settings.local.json", "Everyone in the project, checked in") {
		t.Fatalf("the add form is not drawn:\n%s", r.drawn())
	}
	p.tools.set.rule.SetText("WebFetch(domain:go.dev)")
	r.press(key.NameDownArrow)
	r.press(key.NameReturn)
	body := f.waitCall(t, api.ChatPermissionAct.Name)
	for _, want := range []string{`"action":"add"`, `"behavior":"allow"`, `"rule":"WebFetch(domain:go.dev)"`, `"source":"project"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the add asked %s, want %s", body, want)
		}
	}
	toolsSettle(r)
	if p.tools.detail != "" || p.tools.set.rule.Text() != "" {
		t.Errorf("a saved rule left the form open (%q) or its text (%q)", p.tools.detail, p.tools.set.rule.Text())
	}
}

// A rule opens with what it means and Remove where a settings file holds it;
// one that does not says so and offers nothing.
func TestARuleOpensAndIsRemoved(t *testing.T) {
	_, r, f, _ := permFrames(t, chatPermissionsCommand)
	r.click(t, r.at(t, "Edit"))
	if !r.shows("This rule does not come from a settings file lich can change.") || r.shows("Remove") {
		t.Fatalf("a flag's rule offers Remove:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	r.click(t, r.at(t, "Bash(npm run *)"))
	if !r.showsAll("Means", "From", "Remove") {
		t.Fatalf("the rule's detail is not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Remove"))
	if body := f.waitCall(t, api.ChatPermissionAct.Name); !strings.Contains(body, `"action":"remove"`) || !strings.Contains(body, `"rule":"Bash(npm run *)"`) {
		t.Errorf("Remove asked %s", body)
	}
}

// Folders lists where the chat started and the folders it may also work in.
func TestFoldersTab(t *testing.T) {
	_, r, _, p := permFrames(t, chatPermissionsCommand)
	p.tools.set.tabs.Selected = permTabFolders
	r.frames(2)
	if !r.showsAll("Started in", "/repo", "/srv/data", "Local") || r.shows("Add a rule…") {
		t.Errorf("the folders are not drawn:\n%s", r.drawn())
	}
}

// Codex reports no rules: /permissions lists its modes, the one in force
// checked, and a pick sets it and shuts the panel.
func TestCodexPermissionsAreItsModes(t *testing.T) {
	u, r, f, p := permFrames(t, chatPermissionsCommand)
	f.mu.Lock()
	f.answers[api.ChatPermissions.Name] = api.ChatPermissionList{}
	f.mu.Unlock()
	u.model.chat.setOptions("s1", []backend.ChatOption{{ID: "permissions", Category: api.OptionMode, Current: ":workspace",
		Choices: []backend.ChatChoice{{ID: ":read-only", Label: "Read Only"}, {ID: ":workspace", Label: "Workspace"}}}})
	u.fetchTools(p, "s1")
	toolsSettle(r)
	if !r.showsAll("Permissions", "Read Only", "Workspace") {
		t.Fatalf("the modes are not drawn:\n%s", r.drawn())
	}
	r.press(key.NameReturn)
	if body := f.waitCall(t, api.ChatSetOption.Name); !strings.Contains(body, `":read-only"`) {
		t.Errorf("the pick asked %s", body)
	}
	if p.tools.opened() {
		t.Error("a pick left the panel open")
	}
}

// /hooks lists the hooks by event, one that needs trust marked; its detail
// trusts it with the hash it showed.
func TestHooksListAndTrust(t *testing.T) {
	_, r, f, _ := permFrames(t, chatHooksCommand)
	if !r.showsAll("Hooks", "2 hooks", "PreToolUse", "echo pre", "Bash", "User", "Stop", "Stopping", "Project") {
		t.Fatalf("the hooks are not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "echo pre"))
	if !r.showsAll("needs your trust", "For tools", "Runs", "Trust") || r.shows("Turn on") {
		t.Fatalf("the hook's detail is not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Trust"))
	if body := f.waitCall(t, api.ChatHookAct.Name); !strings.Contains(body, `"action":"trust"`) || !strings.Contains(body, `"hash":"sha256:a"`) || !strings.Contains(body, `"key":"k1"`) {
		t.Errorf("Trust asked %s", body)
	}
}

// A provider whose hooks are read only says where they change, and offers
// nothing.
func TestReadOnlyHooks(t *testing.T) {
	u, r, f, p := permFrames(t, chatHooksCommand)
	f.mu.Lock()
	f.answers[api.ChatHooks.Name] = api.ChatHookList{Reported: true, Blocked: api.HooksOff, Hooks: []api.ChatHook{{Event: "Stop", Source: api.SettingsUser, Type: "command", Command: "echo stop"}}}
	f.mu.Unlock()
	u.fetchTools(p, "s1")
	toolsSettle(r)
	if !r.shows("Hooks are turned off (disableAllHooks), so none of these run.") {
		t.Errorf("why the hooks do not run is not said:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "echo stop"))
	if !r.shows("Hooks are changed in the settings files, or by asking the agent.") || r.shows("Turn off") {
		t.Errorf("a read-only hook offers an action:\n%s", r.drawn())
	}
}

// While a turn runs, Codex's modes are listed but a pick waits, as the mode
// chip does, and the panel says why.
func TestCodexModesWaitForTheTurn(t *testing.T) {
	u, r, f, p := permFrames(t, chatPermissionsCommand)
	f.mu.Lock()
	f.answers[api.ChatPermissions.Name] = api.ChatPermissionList{}
	f.mu.Unlock()
	u.model.chat.setOptions("s1", []backend.ChatOption{{ID: "permissions", Category: api.OptionMode, Current: ":workspace",
		Choices: []backend.ChatChoice{{ID: ":read-only", Label: "Read Only"}, {ID: ":workspace", Label: "Workspace"}}}})
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	u.fetchTools(p, "s1")
	toolsSettle(r)
	if !r.showsAll("Read Only", "Wait for the turn to end to change this.") {
		t.Fatalf("the busy modes are not drawn:\n%s", r.drawn())
	}
	r.press(key.NameReturn)
	toolsSettle(r)
	if f.callsOf(api.ChatSetOption.Name) != 0 || !p.tools.opened() {
		t.Error("a pick changed the mode while the turn runs")
	}
}
