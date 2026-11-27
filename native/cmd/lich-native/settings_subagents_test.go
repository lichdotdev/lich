package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// subagentCardsFixture is one provider in each state the screen draws:
// Claude Code bringing a server and a plugin, opencode bringing nothing,
// oh-my-pi with its cards off, and Codex, which cannot start with less.
func subagentCardsFixture() []api.SubagentCardProvider {
	return []api.SubagentCardProvider{
		{Provider: "claude", Cards: true, Lean: true, Items: []api.SubagentCardItem{
			{ID: "mcp:ai-memory", Kind: api.SubagentBringMCP, Name: "ai-memory", Origin: ".claude.json", On: true},
			{ID: "mcp:postgres", Kind: api.SubagentBringMCP, Name: "postgres", Origin: api.SubagentOriginLibrary},
			{ID: "plugin:ponytail@ponytail", Kind: api.SubagentBringPlugin, Name: "ponytail", Origin: "ponytail", On: true},
			{ID: "plugin:hyperframes@heygen", Kind: api.SubagentBringPlugin, Name: "hyperframes", Origin: "heygen"},
			{ID: api.SubagentAlsoMemory, Kind: api.SubagentBringAlso, Name: api.SubagentAlsoMemory},
		}},
		{Provider: "codex", Cards: true},
		{Provider: "opencode", Cards: true, Lean: true, Items: []api.SubagentCardItem{
			{ID: "mcp:github", Kind: api.SubagentBringMCP, Name: "github", Origin: "opencode.json"},
			{ID: api.SubagentAlsoClaudeSetup, Kind: api.SubagentBringAlso, Name: api.SubagentAlsoClaudeSetup},
		}},
		{Provider: "omp", Cards: false, Lean: true, Items: []api.SubagentCardItem{
			{ID: api.SubagentAlsoExtensions, Kind: api.SubagentBringAlso, Name: api.SubagentAlsoExtensions},
			{ID: api.SubagentAlsoSkills, Kind: api.SubagentBringAlso, Name: api.SubagentAlsoSkills},
			{ID: api.SubagentAlsoRules, Kind: api.SubagentBringAlso, Name: api.SubagentAlsoRules},
		}},
	}
}

func subagentFrames(t *testing.T) (*settingsScreen, *frameRig, *sidebarBackend) {
	t.Helper()
	s, r, f := settingsFramesOn(t, sectionSubagents)
	setDetected(f, []backend.DetectedProvider{
		{ID: "claude", Name: "Claude Code", Installed: true},
		{ID: "codex", Name: "Codex", Installed: true},
		{ID: "opencode", Name: "opencode", Installed: true},
		{ID: "omp", Name: "oh-my-pi", Installed: true},
	})
	f.mu.Lock()
	f.settings["provider.opencode.enabled@"], f.settings["provider.omp.enabled@"] = "1", "1"
	f.mu.Unlock()
	if err := s.providers.book.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.subagents.store = new(subagentCardsStore)
	if err := s.subagents.store.load(t.Context(), s.client); err != nil {
		t.Fatal(err)
	}
	r.settle(t)
	return s, r, f
}

// Each provider's row says in a line what one of its cards brings.
func TestSubagentsListSaysWhatACardBrings(t *testing.T) {
	_, r, _ := subagentFrames(t)
	want := []string{
		i18n.T("settings.subagentsSettings.subtitle"),
		i18n.T("settings.subagentsSettings.detailBrings", i18n.Args{"items": "ai-memory, ponytail"}),
		i18n.T("settings.subagentsSettings.detailNothing"),
		i18n.T("settings.subagentsSettings.detailOff"),
		i18n.T("settings.subagentsSettings.detailEverything", i18n.Args{"provider": "Codex"}),
	}
	if !r.showsAll(want...) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	shootBothThemes(t, r, "settings-subagents")
}

// Claude Code's page lists its servers and plugins with how many are on, and
// Memory alone under Also; pressing one sends it.
func TestClaudeCodesSubagentPage(t *testing.T) {
	_, r, f := subagentFrames(t)
	r.clickLabel(t, "Claude Code")
	r.settle(t)
	onOf := func(on, count int) string {
		return i18n.T("settings.subagentsSettings.onOf", i18n.Args{"on": on, "count": count})
	}
	if !r.showsAll(i18n.T("settings.subagentsSettings.subtitleLean"), strings.ToUpper(i18n.T("settings.settings.section.mcp")),
		strings.ToUpper(i18n.T("settings.settings.section.plugins")), strings.ToUpper(i18n.T("settings.subagentsSettings.also")),
		onOf(1, 2), "ai-memory", ".claude.json", i18n.T("settings.subagentsSettings.originLibrary"), "hyperframes", "heygen",
		i18n.T("settings.subagentsSettings.memory")) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	if r.shows(i18n.T("settings.subagentsSettings.skillsDetail")) {
		t.Error("Claude Code's page offers a Skills switch, which saves a card nothing")
	}
	r.hover(f32.Pt(1, 1)) // off the row the click left a tooltip on
	r.frames(2)
	shootBothThemes(t, r, "settings-subagents-claude")
	r.click(t, r.controlOf(t, "hyperframes"))
	if got := calledWith(t, f, "subagent.SetBring"); !strings.Contains(got, `"item":"plugin:hyperframes@heygen"`) || !strings.Contains(got, `"on":true`) {
		t.Errorf("sent %s, want hyperframes on", got)
	}
}

// opencode's page has servers and Claude Code's setup but no Plugins group,
// and says its plugins always come.
func TestOpencodesSubagentPage(t *testing.T) {
	s, r, _ := subagentFrames(t)
	s.subagents.open = "opencode"
	r.settle(t)
	if !r.showsAll("github", i18n.T("settings.subagentsSettings.claudeSetup"), i18n.T("settings.subagentsSettings.opencodePlugins")) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	if r.shows(strings.ToUpper(i18n.T("settings.settings.section.plugins"))) {
		t.Error("opencode's page lists plugins it cannot leave out")
	}
}

// Cards off hides what a card would bring; a provider that cannot start with
// less says so and lists nothing.
func TestSubagentPagesWithNothingToChoose(t *testing.T) {
	s, r, f := subagentFrames(t)
	s.subagents.open = "omp"
	r.settle(t)
	if !r.shows(i18n.T("settings.subagentsSettings.subtitleOff", i18n.Args{"provider": "oh-my-pi"})) || r.shows(i18n.T("settings.subagentsSettings.extensions")) {
		t.Fatalf("cards off, drawn: %s", r.drawn())
	}
	s.subagents.open = "codex"
	r.settle(t)
	if !r.shows(i18n.T("settings.subagentsSettings.subtitleEverything", i18n.Args{"provider": "Codex"})) || r.shows(strings.ToUpper(i18n.T("settings.subagentsSettings.also"))) {
		t.Fatalf("codex, drawn: %s", r.drawn())
	}
	r.click(t, r.controlOf(t, "Codex"))
	if got := calledWith(t, f, "subagent.SetCards"); !strings.Contains(got, `"provider":"codex"`) || !strings.Contains(got, `"on":false`) {
		t.Errorf("sent %s, want Codex's cards off", got)
	}
}

// A project's scope leaves the page to Global.
func TestSubagentsInAProjectsScope(t *testing.T) {
	s, r, _ := subagentFrames(t)
	s.scope = "pg"
	r.settle(t)
	if !r.shows(i18n.T("settings.settingsRow.allGlobal")) || r.shows("Claude Code") {
		t.Fatalf("drawn: %s", r.drawn())
	}
}

// A card's tooltip names what it brought, and says nothing when it brought
// nothing.
func TestTheTooltipNamesWhatACardBrought(t *testing.T) {
	line := i18n.T("sidebar.sessionTooltip.brought", i18n.Args{"items": "ai-memory, ponytail, " + i18n.T("settings.subagentsSettings.memory")})
	for _, c := range []struct {
		brought []string
		shows   bool
	}{{nil, false}, {[]string{"mcp:ai-memory", "plugin:ponytail@ponytail", api.SubagentAlsoMemory}, true}} {
		sv := sessionView{Session: backend.Session{ID: "s1", Label: "Worker", Brought: c.brought}, shown: "/repo"}
		r := newFrameRig(image.Pt(400, 160), func(gtx C, th *ui.Theme) { sessionTooltip(th, &sv)(gtx) })
		drawn, _ := r.frame()
		if r.shows(line) != c.shows {
			t.Errorf("brought %v: line drawn %v, want %v:\n%s", c.brought, !c.shows, c.shows, drawn)
		}
	}
}
