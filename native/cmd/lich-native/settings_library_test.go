package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// libraryFixture is a library over the eight providers: a skill on for most of
// them, one off everywhere, a server, one found skill, and Claude Code with a
// second profile.
func libraryFixture() backend.Library {
	ids := []string{"claude", "codex", "antigravity", "opencode", "omp", "crush", "cursor", "kiro"}
	names := []string{"Claude Code", "Codex", "Antigravity", "opencode", "oh-my-pi", "Crush", "Cursor CLI", "Kiro CLI"}
	var list []backend.LibraryProvider
	for i, id := range ids {
		p := backend.LibraryProvider{ID: id, Name: names[i]}
		switch id {
		case "claude":
			p.Profiles = []api.Profile{{Slug: "personal", Name: "Personal", Personal: true}, {Slug: "work", Name: "Work"}}
		case "antigravity", "crush", "kiro":
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
	if !r.showsAll(i18n.T("settings.settings.section.skills"), "codixel", "pdf", "deploy-preview", i18n.T("settings.librarySettings.moveInAll")) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	if r.shows("chrome-devtools") || r.shows("postgres") {
		t.Error("the Skills screen lists an MCP server")
	}
	calledWith(t, f, "library.Seen")
	writeShot(t, r, "settings-skills.png")

	box := labelBox(t, r, "codixel")
	r.hover(f32.Pt(float32(box.Min.X+2), float32(box.Min.Y+2)))
	r.frames(2)
	r.settle(t)
	writeShot(t, r, "settings-skills-hover.png")
	r.clickLabel(t, i18n.T("settings.librarySettings.moveIn"))
	if got := calledWith(t, f, "library.MoveIn"); !strings.Contains(got, `"name":"codixel"`) {
		t.Errorf("move in %s, want codixel", got)
	}
}

// A skill's detail is a switch per provider and per Claude Code profile;
// pressing one stores it.
func TestSkillDetail(t *testing.T) {
	_, r, f := libraryFrames(t, sectionSkills)
	r.clickLabel(t, "pdf")
	r.settle(t)
	if !r.showsAll("Reads, fills and merges PDF files.", "Claude Code", "Personal", "Work", "Kiro CLI") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	writeShot(t, r, "settings-skill-detail.png")
	r.click(t, r.controlOf(t, "Codex"))
	if got := calledWith(t, f, "library.Set"); !strings.Contains(got, `"provider":"codex"`) || !strings.Contains(got, `"on":true`) {
		t.Errorf("set %s, want Codex turned on", got)
	}
	r.clickLabel(t, i18n.T("settings.settings.section.skills"))
	r.settle(t)
	if !r.shows("deploy-preview") {
		t.Error("back did not return to the list")
	}
}

// MCP servers name their transport, and a server's detail puts the
// providers lich does not write servers into last, saying so.
func TestMCPScreenAndDetail(t *testing.T) {
	_, r, _ := libraryFrames(t, sectionMCP)
	if !r.showsAll("chrome-devtools", "postgres", "stdio") {
		t.Fatalf("drawn: %s", r.drawn())
	}
	writeShot(t, r, "settings-mcp.png")
	r.clickLabel(t, "postgres")
	r.settle(t)
	why := i18n.T("settings.librarySettings.reason.notWritten", i18n.Args{"provider": "Crush"})
	if !r.showsAll("npx -y @modelcontextprotocol/server-postgres", why) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	writeShot(t, r, "settings-mcp-detail.png")
}

// Plugins holds lich's own plugin, locked, with what it brings.
func TestPluginsScreen(t *testing.T) {
	_, r, _ := libraryFrames(t, sectionPlugins)
	brings := i18n.T("settings.librarySettings.pluginBrings", i18n.Args{"hooks": 9, "skills": 2, "mcp": 1})
	if !r.showsAll("lich", "v0.23.0", brings) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	writeShot(t, r, "settings-plugins.png")
}
