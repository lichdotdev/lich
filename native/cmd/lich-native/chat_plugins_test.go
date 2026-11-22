package main

import (
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
)

var toolsPlugins = api.ChatPluginList{
	Reported: true,
	Actions: []string{api.PluginInstall, api.PluginUninstall, api.PluginEnable, api.PluginDisable, api.PluginUpdate,
		api.MarketplaceAdd, api.MarketplaceUpdate, api.MarketplaceRemove},
	Scopes: []string{api.PluginScopeUser, api.PluginScopeProject, api.PluginScopeLocal},
	Plugins: []api.ChatPlugin{
		{ID: "ponytail@dietrich", Name: "ponytail", Marketplace: "dietrich", Description: "Lazy senior developer", Version: "1.8.0", Latest: "1.9.0",
			Installed: true, Enabled: true, Scope: "user", Parts: &api.ChatPluginParts{Skills: 7, Hooks: 2, Tokens: 640}},
		{ID: "skill-creator@official", Name: "skill-creator", Marketplace: "official", Description: "Create skills", Version: "2.0.1",
			Installed: true, Scope: "project", Parts: &api.ChatPluginParts{Skills: 1, Tokens: 120}},
		{ID: "lich@lich-plugin", Name: "lich", Marketplace: "lich-plugin", Version: "0.21.1", Installed: true, Enabled: true, Scope: "user", Fixed: true},
		{ID: "frontend-design@official", Name: "frontend-design", Marketplace: "official", Description: "Product-grade interfaces", Installs: 48000},
		{ID: "pr-review@official", Name: "pr-review", Marketplace: "official", Description: "Review pull requests", Installs: 21000},
	},
	Marketplaces: []api.ChatMarketplace{
		{Name: "official", Source: "anthropics/claude-plugins-official", Plugins: 3},
		{Name: "lich-plugin", Source: "lichdotdev/lich-plugin", Plugins: 1, Fixed: true},
	},
}

// pluginFrames is /plugin open on a Claude Code chat whose backend answers
// toolsPlugins.
func pluginFrames(t *testing.T) (*frameRig, *chatBackend, *chatPane) {
	t.Helper()
	u, r, f, p := toolsFrames(t, "/plugin")
	f.mu.Lock()
	f.answers[api.ChatPlugins.Name] = toolsPlugins
	f.mu.Unlock()
	p.tools.open(chatPluginCommand)
	u.fetchTools(p, "s1")
	toolsSettle(r)
	return r, f, p
}

// /plugin opens on the installed plugins, each marketplace's under its
// name, with what each costs and whether an update is out.
func TestPluginOpensOnTheInstalledOnes(t *testing.T) {
	u, r, f, p := toolsFrames(t, "/plugin")
	if u.model.chat.isRunning("s1") || f.callsOf(api.ChatPrompt.Name) != 0 || p.editor.Len() != 0 {
		t.Fatal("/plugin went to the agent")
	}
	if f.callsOf(api.ChatPlugins.Name) == 0 {
		t.Fatal("/plugin read no plugins")
	}
	r, _, _ = pluginFrames(t)
	if !r.showsAll("Plugins", "3 installed", "Discover", "Installed", "Marketplaces", "ponytail", "Lazy senior developer",
		"update", "user · ~640 tokens", "official", "project · ~120 tokens") {
		t.Errorf("the installed plugins are not drawn:\n%s", r.drawn())
	}
	if r.shows("frontend-design") {
		t.Error("a plugin on offer is among the installed")
	}
}

// A plugin opens on a click with what it brings and what can be done to it;
// Turn off reaches the backend in its scope, Escape goes back and shuts.
func TestAnInstalledPluginOpensAndTurnsOff(t *testing.T) {
	r, f, p := pluginFrames(t)
	r.click(t, r.at(t, "skill-creator"))
	if !r.showsAll("Create skills", "2.0.1", "off", "project", "1 skill", "~120 tokens", "Update", "Turn on", "Uninstall") {
		t.Fatalf("the plugin's detail is not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Turn on"))
	if body := f.waitCall(t, api.ChatPluginAct.Name); !strings.Contains(body, `"action":"enable"`) ||
		!strings.Contains(body, `"target":"skill-creator@official"`) || !strings.Contains(body, `"scope":"project"`) {
		t.Errorf("Turn on asked %s", body)
	}
	r.press(key.NameEscape)
	if !r.shows("3 installed") {
		t.Errorf("Escape did not go back to the list:\n%s", r.drawn())
	}
	r.press(key.NameEscape)
	if p.tools.opened() {
		t.Errorf("a second Escape left the panel open:\n%s", r.drawn())
	}
}

// A newer version shows in the detail, and an update that installed one
// says the chat runs it once restarted.
func TestAnUpdateSaysItNeedsARestart(t *testing.T) {
	r, f, _ := pluginFrames(t)
	f.mu.Lock()
	f.answers[api.ChatPluginAct.Name] = api.ChatPluginActResult{Restart: true}
	f.mu.Unlock()
	r.click(t, r.at(t, "ponytail"))
	if !r.showsAll("1.8.0, 1.9.0 is out", "7 skills · 2 hooks") {
		t.Fatalf("the detail does not say a newer version is out:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Update"))
	f.waitCall(t, api.ChatPluginAct.Name)
	toolsSettle(r)
	if !r.shows("Updated. The chat runs the new version once restarted.") {
		t.Errorf("the update's restart is not said:\n%s", r.drawn())
	}
}

// lich's own plugin offers nothing to do, and says why.
func TestLichsOwnPluginTakesNoAction(t *testing.T) {
	r, _, _ := pluginFrames(t)
	r.click(t, r.at(t, "lich"))
	if !r.shows("lich installs and updates this plugin itself.") || r.shows("Uninstall") || r.shows("Turn off") {
		t.Errorf("lich's plugin offers an action:\n%s", r.drawn())
	}
}

// Discover lists what is on offer, the most installed first, filtered by
// the search; one installs in the scope its button names.
func TestDiscoverSearchesAndInstalls(t *testing.T) {
	r, f, p := pluginFrames(t)
	r.click(t, r.at(t, "Discover"))
	if !r.showsAll("frontend-design", "48K installs", "pr-review") || r.shows("skill-creator") {
		t.Fatalf("Discover does not list the plugins on offer:\n%s", r.drawn())
	}
	p.tools.query.SetText("review")
	r.frames(2)
	if r.shows("frontend-design") || !r.shows("pr-review") {
		t.Errorf("the search did not filter:\n%s", r.drawn())
	}
	if got := p.tools.discovered(toolsPlugins.Plugins, p.tools.gen); len(got) != 1 {
		t.Errorf("review found %d plugins, want pr-review alone", len(got))
	}
	p.tools.query.SetText("interfaces")
	if got := p.tools.discovered(toolsPlugins.Plugins, p.tools.gen); len(got) != 1 || got[0].Name != "frontend-design" {
		t.Errorf("a search by description found %+v", got)
	}
	p.tools.query.SetText("review")
	r.frames(2)
	r.click(t, r.at(t, "pr-review"))
	if !r.showsAll("Install for you", "Install in the project", "Install here only") {
		t.Fatalf("the install buttons are not drawn:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Install here only"))
	if body := f.waitCall(t, api.ChatPluginAct.Name); !strings.Contains(body, `"action":"install"`) || !strings.Contains(body, `"scope":"local"`) {
		t.Errorf("install asked %s", body)
	}
}

// Marketplaces lists each with its source; a typed source and Enter adds
// one, and a marketplace opens with Update and Remove.
func TestMarketplacesAddAndOpen(t *testing.T) {
	r, f, p := pluginFrames(t)
	r.click(t, r.at(t, "Marketplaces"))
	if !r.showsAll("official", "anthropics/claude-plugins-official", "3 plugins", "Add marketplace") {
		t.Fatalf("the marketplaces are not drawn:\n%s", r.drawn())
	}
	p.tools.plug.source.SetText("owner/repo")
	r.frames(2)
	r.press(key.NameReturn)
	if body := f.waitCall(t, api.ChatPluginAct.Name); !strings.Contains(body, `"action":"marketplace-add"`) || !strings.Contains(body, `"target":"owner/repo"`) {
		t.Errorf("Enter asked %s", body)
	}
	time.Sleep(20 * time.Millisecond)
	r.click(t, r.at(t, "official"))
	if !r.showsAll("Update", "Remove") {
		t.Errorf("the marketplace's detail is not drawn:\n%s", r.drawn())
	}
}

// A plugin the search names comes before one it only describes, however
// many installed that one.
func TestDiscoverPutsNamesFirst(t *testing.T) {
	var tl chatTools
	tl.plug.reset()
	tl.query.SetText("review")
	got := tl.discovered([]api.ChatPlugin{{Name: "linter", Description: "Lints, then asks for a review", Installs: 900}, {Name: "pr-review"}}, 1)
	if len(got) != 2 || got[0].Name != "pr-review" {
		t.Errorf("found %+v, want pr-review first", got)
	}
}
