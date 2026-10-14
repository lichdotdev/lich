package main

import (
	"context"
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

func TestSkipLadderRoundTripsAndOnlyClimbingAsks(t *testing.T) {
	for _, rung := range skipRungs {
		here, worktrees := skipLevelPair(rung.value)
		if got := skipLevelOf(here, worktrees); got != rung.value {
			t.Errorf("%s writes (%v, %v), which reads as %s", rung.value, here, worktrees, got)
		}
	}
	if got := skipLevelOf(true, false); got != skipEverywhere {
		t.Errorf("skip here, ask in worktrees reads as %s, want everywhere", got)
	}
	climbs := map[[2]string]bool{
		{skipNever, skipWorktrees}:      true,
		{skipNever, skipEverywhere}:     true,
		{skipWorktrees, skipEverywhere}: true,
		{skipEverywhere, skipWorktrees}: false,
		{skipWorktrees, skipNever}:      false,
		{skipEverywhere, skipNever}:     false,
		{skipWorktrees, skipWorktrees}:  false,
	}
	for move, want := range climbs {
		if got := skipClimbs(move[0], move[1]); got != want {
			t.Errorf("%s → %s asks = %v, want %v", move[0], move[1], got, want)
		}
	}
}

func TestEveryProviderHasASkipFlagAndResumes(t *testing.T) {
	for _, kind := range providerKinds() {
		if !hasSkipFlag(kind) {
			t.Errorf("%s has no skip flag", kind)
		}
		if !canResume(kind) {
			t.Errorf("%s cannot resume", kind)
		}
	}
	if len(skipFlags) != len(providerKinds()) {
		t.Errorf("%d flags for %d providers", len(skipFlags), len(providerKinds()))
	}
}

func TestOnlyClaudeHasUltracodeAndSubagentCards(t *testing.T) {
	for _, kind := range providerKinds() {
		want := kind == "claude"
		if supportsUltracode(kind) != want || supportsSubagentCards(kind) != want {
			t.Errorf("%s: ultracode %v, subagent cards %v", kind, supportsUltracode(kind), supportsSubagentCards(kind))
		}
	}
}

func TestPasteUnfoldTimingPerProvider(t *testing.T) {
	want := map[string]uint8{
		"claude": pasteNextPaste, "kiro": pasteNextPaste, "opencode": pasteNextSession,
		"codex": pasteNone, "antigravity": pasteNone, "omp": pasteNone, "crush": pasteNone, "cursor": pasteNone,
	}
	for kind, timing := range want {
		if got := pasteUnfoldTiming(kind); got != timing {
			t.Errorf("%s = %d, want %d", kind, got, timing)
		}
	}
}

func TestAnUnsetOrParkedLayerNeverWins(t *testing.T) {
	set := binLayer{bin: "/opt/claude"}
	parked := binLayer{bin: "/opt/claude", off: true}
	spaces := binLayer{bin: "   "}
	cases := []struct {
		name            string
		project, global binLayer
		want            string
	}{
		{"nothing set", binLayer{}, binLayer{}, binScopePath},
		{"project wins", set, binLayer{bin: "/g"}, binScopeProject},
		{"global when the project is empty", binLayer{}, set, binScopeGlobal},
		{"a parked project falls to global", parked, binLayer{bin: "/g"}, binScopeGlobal},
		{"parked both fall to $PATH", parked, parked, binScopePath},
		{"spaces are no path", spaces, spaces, binScopePath},
	}
	for _, c := range cases {
		if got := winningBinScope(c.project, c.global); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	if got := configuredBin(binScopePath, parked, parked); got != "" {
		t.Errorf("a parked path is what $PATH wins over, got %q", got)
	}
	if got := configuredBin(binScopeGlobal, binLayer{}, binLayer{bin: " /g "}); got != "/g" {
		t.Errorf("configured = %q, want the trimmed path", got)
	}
}

func TestParkedOverridesAreNamedOnlyWhenSwitchingThemOnWouldChangeTheSpawn(t *testing.T) {
	parked := binLayer{bin: "/x", off: true}
	set := binLayer{bin: "/x"}
	cases := []struct {
		name            string
		scope           string
		project, global binLayer
		want            string
	}{
		{"nothing parked", binScopePath, binLayer{}, binLayer{}, ""},
		{"a parked global under a winning project is shadowed", binScopeProject, set, parked, ""},
		{"a parked project under global is news", binScopeGlobal, parked, set, " · lich-playground override off"},
		{"both parked under $PATH are both named", binScopePath, parked, parked, " · lich-playground override off · global override off"},
		{"an empty off layer holds nothing to put back", binScopePath, binLayer{off: true}, binLayer{}, ""},
	}
	for _, c := range cases {
		if got := parkedLabel(c.scope, c.project, c.global, "lich-playground"); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if got := parkedLabel(binScopeGlobal, parked, set, ""); got != " · project override off" {
		t.Errorf("unnamed project: %q", got)
	}
}

func TestBinaryVerdictWordsFollowWhatWasAskedFor(t *testing.T) {
	cases := []struct {
		status, bin, label, detail string
	}{
		{lichclient.CheckOK, "claude", "executable", ""},
		{"not-found", "claude", "not on $PATH", binDetailBroken},
		{"not-found", "/opt/claude", "no such file", binDetailBroken},
		{"not-found", `C:\bin\claude.exe`, "no such file", binDetailBroken},
		{"not-executable", "/opt/claude", "not executable", binDetailBroken},
		{"home-shortcut", "~/claude", "~ is not expanded", binDetailHome},
		{"relative", "./claude", "relative path", binDetailRel},
		{lichclient.CheckEmpty, "", "", ""},
	}
	for _, c := range cases {
		check := lichclient.BinaryCheck{Status: c.status}
		if got := binCheckLabel(check, c.bin); got != c.label {
			t.Errorf("%s %q label = %q, want %q", c.status, c.bin, got, c.label)
		}
		if got := binCheckDetail(check); got != c.detail {
			t.Errorf("%s detail = %q, want %q", c.status, got, c.detail)
		}
	}
}

func TestPlanSummaryFitsAListRow(t *testing.T) {
	ok := lichclient.QuotaPlan{Status: "ok", Windows: []lichclient.QuotaWindow{
		{Label: "Weekly", Seconds: 7 * day, Percent: 20},
		{Label: "Session", Seconds: 5 * hour, Percent: 62},
	}}
	cases := []struct {
		name  string
		plan  lichclient.QuotaPlan
		found bool
		want  string
	}{
		{"no reading", lichclient.QuotaPlan{}, false, ""},
		{"the hottest window", ok, true, "62% of the 5h window"},
		{"a window with no length", lichclient.QuotaPlan{Status: "ok", Windows: []lichclient.QuotaWindow{{Percent: 7}}}, true, "7% used"},
		{"signed out is words", lichclient.QuotaPlan{Status: "signed-out"}, true, "Signed out"},
		{"an error says nothing", lichclient.QuotaPlan{Status: "error"}, true, ""},
		{"no windows says nothing", lichclient.QuotaPlan{Status: "ok"}, true, ""},
	}
	for _, c := range cases {
		if got := planSummary(c.plan, c.found); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestListRowsKeepOnlyDefaultHereAndNoPrompts(t *testing.T) {
	cases := []struct {
		name, projectDefault, skip string
		want                       []string
	}{
		{"nothing to say", "", skipNever, nil},
		{"worktrees only is not a tag", "", skipWorktrees, nil},
		{"another provider is the project's default", "codex", skipNever, nil},
		{"the project's default", "claude", skipNever, []string{"default here"}},
		{"nothing asks", "", skipEverywhere, []string{"no prompts"}},
		{"both", "claude", skipEverywhere, []string{"default here", "no prompts"}},
	}
	for _, c := range cases {
		if got := providerTags("claude", c.projectDefault, c.skip); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestInstallSummaryNamesAConfiguredPath(t *testing.T) {
	if got := installSummary(providerInfo{source: "path"}); got != "Installed" {
		t.Errorf("on $PATH: %q", got)
	}
	if got := installSummary(providerInfo{source: "setting"}); got != "Installed · custom path" {
		t.Errorf("configured: %q", got)
	}
}

// What Settings writes for a restore choice is what the spawn gate reads and
// what the resume prompt's "always" writes: one key, one set of values.
func TestRestoreChoiceIsWhatTheGateUsesNext(t *testing.T) {
	want := map[string]spawnVerdict{restoreAsk: spawnAsk, restoreResume: spawnResume, restoreFresh: spawnPlain}
	if len(restoreRungs) != len(want) {
		t.Fatalf("%d rungs for %d answers the gate has", len(restoreRungs), len(want))
	}
	session := lichclient.Session{ID: "s", Kind: "codex", ProviderSessionID: "thread"}
	for _, rung := range restoreRungs {
		if restoreChoice(rung.value) != rung.value {
			t.Errorf("%q is not a value the gate reads back as itself", rung.value)
		}
		probe := &storeProbe{fakeProbe: fakeProbe{available: true}, store: map[string]string{restoreKey("codex"): rung.value}}
		if got := spawnDecision(context.Background(), probe, "/repo", session); got != want[rung.value] {
			t.Errorf("%s decides %d, want %d", rung.value, got, want[rung.value])
		}
	}
}

func TestThePromptsAlwaysIsASettingsChoice(t *testing.T) {
	probe := &storeProbe{store: map[string]string{}}
	g := &spawnGate{probe: probe, invalidate: func() {}, notify: func(ui.Toast) {}}
	g.asking = &lichclient.Session{ID: "s", Kind: "codex", ProviderSessionID: "thread"}
	g.answer(context.Background(), restoreResume, true)
	stored := probe.store[restoreKey("codex")]
	if !slices.ContainsFunc(restoreRungs, func(r struct{ value, label, consequence string }) bool { return r.value == stored }) {
		t.Errorf("the prompt stored %q, which no Settings rung is", stored)
	}
}

// storeProbe is a probe whose settings are a map, the way the backend's are.
type storeProbe struct {
	fakeProbe
	store map[string]string
}

func (p *storeProbe) GetSetting(_ context.Context, key, _ string) (string, error) {
	return p.store[key], nil
}

func (p *storeProbe) SetSetting(_ context.Context, key, _, value string) error {
	p.store[key] = value
	return nil
}

func TestPerProviderBlocksAreSearchedOncePerEnabledProvider(t *testing.T) {
	claude := launchProvider{id: "claude", name: "Claude Code", enabled: true}
	codex := launchProvider{id: "codex", name: "Codex", enabled: true}
	hits := func(query string, providers ...launchProvider) []settingHit {
		return searchSettings(query, providers...)
	}

	if got := hits("ultracode"); len(got) != 0 {
		t.Errorf("with no provider on: %+v", got)
	}
	got := hits("ultracode", claude, codex)
	if len(got) != 1 || got[0].providerID != "claude" || got[0].path() != "Providers › Claude Code" {
		t.Errorf("ultracode is Claude Code's alone: %+v", got)
	}
	got = hits("binary", claude, codex)
	if len(got) != 2 || got[0].providerID != "claude" || got[1].providerID != "codex" {
		t.Errorf("a block every provider has is one result each: %+v", got)
	}
	if got := hits("default provider"); len(got) != 1 || got[0].providerID != "" {
		t.Errorf("the default is no provider's block: %+v", got)
	}
	if got := hits("codex", claude, codex); len(got) != len(providerBlockTitlesFor("codex")) {
		t.Errorf("a provider's name finds its blocks: %+v", got)
	}
	if got := hits("unfold", claude, codex); len(got) != 1 {
		t.Errorf("paste unfold is not Codex's: %+v", got)
	}
}

func providerBlockTitlesFor(kind string) []string {
	var titles []string
	for _, e := range providerEntries() {
		if e.perProvider && (len(e.onlyFor) == 0 || slices.Contains(e.onlyFor, kind)) {
			titles = append(titles, e.title)
		}
	}
	return titles
}

func TestProviderKeysAreScopedAndCountedPerProject(t *testing.T) {
	keys := scopedKeys()
	for _, kind := range providerKinds() {
		if !slices.Contains(keys, providerBinKey(kind)) {
			t.Errorf("%s's binary is not read per project", kind)
		}
	}
	if providerBinKey("claude") != "claude.bin" || providerBinKey("codex") != "provider.codex.bin" {
		t.Errorf("keys: %q %q", providerBinKey("claude"), providerBinKey("codex"))
	}
	if !slices.Contains(keys, providerDefaultKey) {
		t.Error("the default provider is not read per project")
	}
	values := scopedValues{providerDefaultKey: {"pg": "codex"}, "provider.codex.bin": {"pg": "/opt/codex"}}
	if got := sectionBadge(sectionByID(sectionProviders), "pg", openTabs, values); got != 2 {
		t.Errorf("Providers badge = %d, want the default and the binary", got)
	}
	if got := sectionBadge(sectionByID(sectionProviders), "", openTabs, values); got != 2 {
		t.Errorf("Providers from Global = %d, want 2 rows overridden somewhere", got)
	}
}

func TestProvidersIsWhereSettingsFirstOpens(t *testing.T) {
	if got := parseSettingsSection("no-such-section"); got != sectionProviders {
		t.Errorf("default section %q", got)
	}
}

func TestTypingAPathWritesItAndAMovedStoreMovesTheField(t *testing.T) {
	var b binInput
	if typed, edited := b.sync("claude", "", "/opt/claude"); edited || typed != "" {
		t.Fatalf("first draw reported an edit: %q", typed)
	}
	if got := b.editor.Text(); got != "/opt/claude" {
		t.Fatalf("field = %q, want the stored path", got)
	}
	b.editor.SetText("/opt/claude2")
	if typed, edited := b.sync("claude", "", "/opt/claude"); !edited || typed != "/opt/claude2" {
		t.Errorf("typed = %q, %v", typed, edited)
	}
	if _, edited := b.sync("claude", "", "/opt/claude2"); edited {
		t.Error("the stored echo of a keystroke reads as another edit")
	}
	if _, edited := b.sync("claude", "", ""); edited || b.editor.Text() != "" {
		t.Errorf("Use Global emptied the store and the field holds %q", b.editor.Text())
	}
	if _, edited := b.sync("codex", "", "/opt/codex"); edited || b.editor.Text() != "/opt/codex" {
		t.Errorf("another provider: %q", b.editor.Text())
	}
}

func TestOnlyTheLastOfABurstOfWritesIsSent(t *testing.T) {
	var p providerSettings
	var sent []string
	done := make(chan struct{})
	p.send.Lock()
	p.enqueue("k", func() { sent = append(sent, "a") })
	p.enqueue("k", func() { sent = append(sent, "ab") })
	p.enqueue("k", func() { sent = append(sent, "abc"); close(done) })
	p.send.Unlock()
	<-done
	p.send.Lock()
	defer p.send.Unlock()
	if !slices.Equal(sent, []string{"abc"}) {
		t.Errorf("sent %v, want only the last", sent)
	}
}

// The machine this was built for: three codex installs on $PATH, the first of
// them the one a session spawns.
func TestOthersNamesTheVersionsASessionDoesNotRun(t *testing.T) {
	v := binVersions{version: "0.162.1", onPath: []lichclient.ProviderBinary{
		{Path: "/home/u/.local/bin/codex", Version: "0.162.1"},
		{Path: "/home/u/.local/share/mise/shims/codex", Version: "0.144.5"},
		{Path: "/opt/codex-copy", Version: "0.162.1"},
		{Path: "/usr/local/bin/codex", Version: ""},
		{Path: "/usr/bin/codex", Version: "0.147.0"},
	}}

	got := alsoOnPathLine(v.others("/home/u/.local/bin/codex"))

	want := "Also on PATH: /home/u/.local/share/mise/shims/codex 0.144.5, /usr/bin/codex 0.147.0. Pin one above to choose."
	if got != want {
		t.Errorf("line = %q, want %q", got, want)
	}
}

func TestOneBinaryOnPathSaysNothing(t *testing.T) {
	v := binVersions{version: "2.1.299", onPath: []lichclient.ProviderBinary{{Path: "/usr/bin/claude", Version: "2.1.299"}}}
	if got := alsoOnPathLine(v.others("/usr/bin/claude")); got != "" {
		t.Errorf("line = %q, want none", got)
	}
}

// A configured binary off $PATH still differs from every version on it.
func TestAConfiguredBinaryIsComparedWithEveryoneOnPath(t *testing.T) {
	scope, configured, asked := spawnedBin(binLayer{}, binLayer{bin: " /opt/codex "}, "codex")
	if scope != binScopeGlobal || configured != "/opt/codex" || asked != "/opt/codex" {
		t.Fatalf("spawnedBin = %q %q %q", scope, configured, asked)
	}
	v := binVersions{version: "0.150.0", onPath: []lichclient.ProviderBinary{{Path: "/usr/bin/codex", Version: "0.147.0"}}}
	if got := v.others("/opt/codex"); len(got) != 1 {
		t.Errorf("others = %+v, want /usr/bin/codex", got)
	}
}
