package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// libraryFixture is a library over the eight providers: a skill on for most of
// them, one off everywhere, a server, one found skill, and Claude Code with a
// second profile.
func libraryFixture() backend.Library {
	ids := []string{"claude", "codex", "antigravity", "opencode", "omp", "crush", "cursor", "kiro"}
	names := []string{"Claude Code", "Codex", "Antigravity", "opencode", "oh-my-pi", "Crush", "Cursor CLI", "Kiro CLI"}
	var list []backend.LibraryProvider
	two := []api.Profile{{Slug: "personal", Name: "Personal", Personal: true}, {Slug: "work", Name: "Work"}}
	for i, id := range ids {
		p := backend.LibraryProvider{ID: id, Name: names[i]}
		switch id {
		case "claude", "opencode":
			p.Profiles = two
		case "codex":
			p.Profiles, p.OneSkills = two, api.LibrarySharedFile
		case "omp", "cursor":
			p.Profiles, p.OneMCP = two, api.LibrarySharedFile
		}
		if id == "antigravity" || id == "crush" || id == "kiro" {
			p.NoMCP = api.LibraryNotWritten
		}
		list = append(list, p)
	}
	switches := func(on ...string) []api.LibrarySwitch {
		var out []api.LibrarySwitch
		for _, p := range list {
			out = append(out, api.LibrarySwitch{Provider: p.ID, On: slices.Contains(on, p.ID)})
			for _, prof := range p.Profiles {
				out = append(out, api.LibrarySwitch{Provider: p.ID, Profile: prof.Slug, On: prof.Slug != "work"})
			}
		}
		return out
	}
	return backend.Library{
		Skills: []backend.LibraryItem{
			{Kind: api.LibrarySkill, Name: "pdf", Description: "Reads, fills and merges PDF files.", Switches: switches("claude", "antigravity", "opencode", "omp", "crush", "cursor", "kiro")},
			{Kind: api.LibrarySkill, Name: "deploy-preview", Description: "Builds a preview of the branch.", Switches: switches()},
		},
		MCP: []backend.LibraryItem{
			{Kind: api.LibraryMCP, Name: "postgres", Command: "npx -y @modelcontextprotocol/server-postgres", Transport: "stdio", Switches: switches("claude", "codex", "opencode")},
		},
		Found: []backend.LibraryFound{
			{Kind: api.LibrarySkill, Name: "codixel", Description: "Pixel art.", Providers: []string{"claude", "crush"}, New: true},
			{Kind: api.LibraryMCP, Name: "chrome-devtools", Transport: "stdio", Providers: []string{"antigravity"}},
		},
		Plugin:    api.LibraryPlugin{Version: "0.23.0", Hooks: 9, Skills: 2, MCP: 1},
		Providers: list,
	}
}

// libraryFixtureIn is libraryFixture in project's scope: the providers'
// plugins of a project like sop_logistica and, in a project, the servers its
// .mcp.json and .codex/config.toml declare.
func libraryFixtureIn(project string) backend.Library {
	lib := libraryFixture()
	claude := []api.LibrarySwitch{{Provider: "claude", Profile: "personal", On: true}, {Provider: "claude", Profile: "work"}}
	if project != "" {
		claude = []api.LibrarySwitch{{Provider: "claude", On: true}}
	}
	codex := []api.LibrarySwitch{{Provider: "codex", Profile: "personal", On: true}, {Provider: "codex", Profile: "work", On: true}}
	lib.Plugins = []backend.LibraryItem{
		{Kind: api.LibraryPluginKind, Name: "@dietrichgebert/ponytail", Switches: []api.LibrarySwitch{{Provider: "opencode", On: true, Reason: api.LibraryNoSwitch}}},
		{Kind: api.LibraryPluginKind, Name: "addon-studio@snk-devcenter", Description: "3.5.5", Switches: append(slices.Clone(claude), codex...)},
		{Kind: api.LibraryPluginKind, Name: "claude-code-wakatime@wakatime", Description: "4.1.2", Switches: claude},
	}
	if project != "" {
		lib.Project = []backend.LibraryFound{
			{Kind: api.LibraryMCP, Name: "sankhya-schema", Transport: "stdio", Providers: []string{"claude"}, Paths: []string{".mcp.json"}},
			{Kind: api.LibraryMCP, Name: "youtrack-snk", Transport: "http", Providers: []string{"claude", "codex"}, Paths: []string{".mcp.json", ".codex/config.toml"}},
		}
	}
	return lib
}

// libraryFrames is Settings open on a library section, its store loaded.
func libraryFrames(t *testing.T, section string) (*settingsScreen, *frameRig, *sidebarBackend) {
	t.Helper()
	s, r, f := settingsFramesOn(t, section)
	s.library.store = new(libraryStore)
	if err := s.library.store.load(t.Context(), s.client); err != nil {
		t.Fatal(err)
	}
	r.settle(t)
	return s, r, f
}

// calledWith waits for the fake backend to have taken method.
func calledWith(t *testing.T, f *sidebarBackend, method string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		f.mu.Lock()
		calls := slices.Clone(f.calls)
		f.mu.Unlock()
		for _, c := range calls {
			if strings.HasPrefix(c, method+"(") {
				return c
			}
		}
	}
	t.Fatalf("no %s call", method)
	return ""
}

// Skills lists what the providers hold that lich does not, then the library,
// and a visit marks what was found as seen.
func TestSkillsScreen(t *testing.T) {
	s, r, f := libraryFrames(t, sectionSkills)
	if s.libraryBadge(sectionSkills) != 1 || s.libraryBadge(sectionMCP) != 0 {
		t.Errorf("badges skills %d mcp %d, want the one new skill counted", s.libraryBadge(sectionSkills), s.libraryBadge(sectionMCP))
	}
	if !r.showsAll(i18n.T("settings.settings.section.skills"), "codixel", "pdf", "deploy-preview", i18n.T("settings.librarySettings.moveInAll"), strings.ToUpper(i18n.T("settings.librarySettings.library"))) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	if r.shows("chrome-devtools") || r.shows("postgres") {
		t.Error("the Skills screen lists an MCP server")
	}
	calledWith(t, f, "library.Seen")
	shootBothThemes(t, r, "settings-skills")

	box := labelBox(t, r, "codixel")
	r.hover(f32.Pt(float32(box.Min.X+2), float32(box.Min.Y+2)))
	r.frames(2)
	r.settle(t)
	shootBothThemes(t, r, "settings-skills-hover")
	r.clickLabel(t, i18n.T("settings.librarySettings.moveIn"))
	if got := calledWith(t, f, "library.MoveIn"); !strings.Contains(got, `"name":"codixel"`) {
		t.Errorf("move in %s, want codixel", got)
	}
}

// switchBeside is whether a switch is drawn on label's line: the line of the
// nth thing drawn as label (0 the first).
func switchBeside(t *testing.T, r *frameRig, label string, nth int) bool {
	t.Helper()
	var top []int
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			top = append(top, n.Desc.Bounds.Min.Y)
		}
	}
	if nth >= len(top) {
		t.Fatalf("%q drawn %d times, want a %dth:\n%s", label, len(top), nth+1, r.drawn())
	}
	for _, n := range r.nodes() {
		b := n.Desc.Bounds
		if n.Desc.Label == "" && n.Desc.Gestures&input.ClickGesture != 0 && b.Dx() < 80 && b.Min.X > r.size.X/2 &&
			b.Min.Y > top[nth]-rowReach && b.Min.Y < top[nth]+rowReach/2 {
			return true
		}
	}
	return false
}

// detailRow is what a provider's part of an item's detail must be.
type detailRow struct {
	provider string
	// heading is a provider whose profiles each have a switch: its name with
	// none, then Personal and Work each with one.
	heading bool
	// shared is the reason one switch serves a provider with profiles.
	shared bool
	// cannot is a provider that cannot take the item at all.
	cannot bool
}

// checkDetail holds the drawn detail to rows, provider by provider.
func checkDetail(t *testing.T, r *frameRig, kind string, rows []detailRow) {
	t.Helper()
	lib := libraryFixture()
	profiles := 0
	for _, row := range rows {
		var p backend.LibraryProvider
		for _, q := range lib.Providers {
			if q.ID == row.provider {
				p = q
			}
		}
		why := ""
		switch {
		case row.cannot:
			why = i18n.T("settings.librarySettings.reason.notWritten", i18n.Args{"provider": p.Name})
		case row.shared:
			why = i18n.T("settings.librarySettings.reason.sharedFile", i18n.Args{"provider": p.Name})
		}
		if why != "" && !r.shows(why) {
			t.Errorf("%s %s: %q not drawn", kind, p.Name, why)
		}
		if got := switchBeside(t, r, p.Name, 0); got != (!row.heading && !row.cannot) {
			t.Errorf("%s %s: a switch beside its name = %v, want %v", kind, p.Name, got, !row.heading && !row.cannot)
		}
		if row.heading {
			for _, name := range []string{"Personal", "Work"} {
				if !switchBeside(t, r, name, profiles) {
					t.Errorf("%s %s: no switch beside %s", kind, p.Name, name)
				}
			}
			profiles++
		}
	}
}

// A provider with profiles each keeping their own switch is a heading with
// one switch per profile; one whose profiles read one file is one switch,
// saying so; one with no profiles is one switch. Pressing one stores it.
func TestSkillDetail(t *testing.T) {
	_, r, f := libraryFrames(t, sectionSkills)
	r.clickLabel(t, "pdf")
	r.settle(t)
	if !r.shows("Reads, fills and merges PDF files.") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	checkDetail(t, r, "skill", []detailRow{
		{provider: "claude", heading: true},
		{provider: "codex", shared: true},
		{provider: "antigravity"},
		{provider: "opencode", heading: true},
		{provider: "omp", heading: true},
		{provider: "crush"},
		{provider: "cursor", heading: true},
		{provider: "kiro"},
	})
	shootBothThemes(t, r, "settings-skill-detail")
	r.click(t, r.controlOf(t, "Codex"))
	if got := calledWith(t, f, "library.Set"); !strings.Contains(got, `"provider":"codex"`) || !strings.Contains(got, `"profile":""`) {
		t.Errorf("set %s, want Codex's one switch", got)
	}
	r.clickLabel(t, i18n.T("settings.settings.section.skills"))
	r.settle(t)
	if !r.shows("deploy-preview") {
		t.Error("back did not return to the list")
	}
}

// MCP servers name their transport; a server's detail splits by profile
// where the provider takes servers per spawn, keeps one switch where its
// profiles read one mcp.json, and puts the providers lich does not write
// servers into last, saying so.
func TestMCPScreenAndDetail(t *testing.T) {
	_, r, _ := libraryFrames(t, sectionMCP)
	if !r.showsAll("chrome-devtools", "postgres", "stdio", strings.ToUpper(i18n.T("settings.librarySettings.library"))) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	shootBothThemes(t, r, "settings-mcp")
	r.clickLabel(t, "postgres")
	r.settle(t)
	if !r.shows("npx -y @modelcontextprotocol/server-postgres") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	checkDetail(t, r, "mcp", []detailRow{
		{provider: "claude", heading: true},
		{provider: "codex", heading: true},
		{provider: "opencode", heading: true},
		{provider: "omp", shared: true},
		{provider: "cursor", shared: true},
		{provider: "antigravity", cannot: true},
		{provider: "crush", cannot: true},
		{provider: "kiro", cannot: true},
	})
	shootBothThemes(t, r, "settings-mcp-detail")
}

// Plugins holds lich's own plugin first, locked, with what it brings, then
// the providers' plugins, each with the marks of the providers it is on in.
func TestPluginsScreen(t *testing.T) {
	_, r, _ := libraryFrames(t, sectionPlugins)
	brings := i18n.T("settings.librarySettings.pluginBrings", i18n.Args{"hooks": 9, "skills": 2, "mcp": 1})
	if !r.showsAll("lich", "v0.23.0", brings, "addon-studio@snk-devcenter", "3.5.5", "@dietrichgebert/ponytail", "claude-code-wakatime@wakatime") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	if lich, first := labelBox(t, r, "lich"), labelBox(t, r, "@dietrichgebert/ponytail"); lich.Min.Y >= first.Min.Y {
		t.Error("lich's own plugin is not the first row")
	}
	shootBothThemes(t, r, "settings-plugins")
}

// A plugin's page is a switch per provider that loads it: Claude Code and
// Codex one per profile, each profile having its own config; opencode none,
// saying why. Pressing one asks the backend to flip it.
func TestPluginDetail(t *testing.T) {
	_, r, f := libraryFrames(t, sectionPlugins)
	r.clickLabel(t, "addon-studio@snk-devcenter")
	r.settle(t)
	for _, name := range []string{"Claude Code", "Codex"} {
		if switchBeside(t, r, name, 0) {
			t.Errorf("%s: a switch beside its name, want a heading over its profiles", name)
		}
	}
	for nth := range 2 {
		for _, profile := range []string{"Personal", "Work"} {
			if !switchBeside(t, r, profile, nth) {
				t.Errorf("no switch beside %s under provider %d", profile, nth)
			}
		}
	}
	shootBothThemes(t, r, "settings-plugin-detail")
	r.click(t, r.controlOf(t, "Work"))
	if got := calledWith(t, f, "library.Set"); !strings.Contains(got, `"kind":"plugin"`) || !strings.Contains(got, `"profile":"work"`) || !strings.Contains(got, `"on":true`) {
		t.Errorf("set %s, want Claude Code's Work profile turned on", got)
	}
	r.clickLabel(t, i18n.T("settings.settings.section.plugins"))
	r.settle(t)
	r.clickLabel(t, "@dietrichgebert/ponytail")
	r.settle(t)
	if !r.shows(i18n.T("settings.librarySettings.reason.noSwitch", i18n.Args{"provider": "opencode"})) || switchBeside(t, r, "opencode", 0) {
		t.Errorf("opencode's plugin: drawn %s, want its reason and no switch", r.drawn())
	}
}

// In a project's scope, MCP servers adds the ones its own files declare,
// with the files and no Move in, and Plugins gives Claude Code one switch,
// which flips it in that project.
func TestLibraryInAProjectsScope(t *testing.T) {
	s, r, f := libraryFrames(t, sectionMCP)
	s.scope = "pg"
	r.frames(2)
	eventually(t, "the project's scope loaded", func() bool { r.frames(1); return r.shows("sankhya-schema") })
	r.settle(t)
	if !r.showsAll(strings.ToUpper(i18n.T("settings.librarySettings.project")), "youtrack-snk", ".mcp.json, .codex/config.toml", "chrome-devtools") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	box := labelBox(t, r, "sankhya-schema")
	r.hover(f32.Pt(float32(box.Min.X+2), float32(box.Min.Y+2)))
	r.frames(2)
	if r.shows(i18n.T("settings.librarySettings.moveIn")) {
		t.Error("a project's server offers Move in")
	}
	shootBothThemes(t, r, "settings-mcp-project")

	s.section, s.page = parseSettingsSection(sectionPlugins), settingsPageOf(sectionPlugins)
	r.hover(f32.Pt(1, 1))
	r.frames(2)
	r.settle(t)
	shootBothThemes(t, r, "settings-plugins-project")
	r.clickLabel(t, "claude-code-wakatime@wakatime")
	r.settle(t)
	if !switchBeside(t, r, "Claude Code", 0) {
		t.Fatalf("no switch beside Claude Code in the project: %s", r.drawn())
	}
	r.click(t, r.controlOf(t, "Claude Code"))
	if got := calledWith(t, f, "library.Set"); !strings.Contains(got, `"project":"/work/lich-playground"`) || !strings.Contains(got, `"on":false`) {
		t.Errorf("set %s, want it off in the project", got)
	}
}

// shootBothThemes writes r's frame in the light theme and the dark one, back
// on the dark one after.
func shootBothThemes(t *testing.T, r *frameRig, name string) {
	t.Helper()
	for _, theme := range []struct {
		suffix  string
		palette ui.Palette
	}{{"light", ui.Light}, {"dark", ui.Dark}} {
		r.theme.Palette, r.theme.Surface = theme.palette, theme.palette.Background
		r.frames(2)
		writeShot(t, r, name+"-"+theme.suffix+".png")
	}
}
