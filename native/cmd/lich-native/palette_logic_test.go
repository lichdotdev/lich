package main

import (
	"slices"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
)

var paletteOpen = []paletteProject{
	{ID: "p1", Name: "lich", Path: "/home/u/try/skipo"},
	{ID: "p2", Name: "revu", Path: "/home/u/try/revu"},
}

var paletteClosed = []paletteProject{
	{ID: "c1", Name: "lich-plugin", Path: "/home/u/try/lich-plugin"},
	{ID: "c2", Name: "controle-de-licitacao", Path: "/home/u/snk/controle-de-licitacao"},
	{ID: "c3", Name: "gh-turnkey", Path: "/home/u/snk/gh-turnkey"},
	{ID: "c4", Name: "policy-engine", Path: "/home/u/hycastle/policy-engine"},
}

func paletteWorkspace() []lichclient.Project {
	return []lichclient.Project{
		{ID: "p1", Name: "lich", Path: "/home/u/try/skipo", Sessions: []lichclient.Session{
			{ID: "s1", Label: "Fix flaky test", Kind: "claude"},
			{ID: "s2", Label: "build watch", Kind: "shell", Path: "/home/u/try/skipo/frontend"},
		}},
		{ID: "p2", Name: "revu", Path: "/home/u/try/revu", Sessions: []lichclient.Session{
			{ID: "s3", Label: "Wire revu auth middleware", Kind: "claude"},
		}},
	}
}

func sessionIDsOf(rows []paletteSession) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.sessionID
	}
	return ids
}

func rowKeys(groups []paletteGroup) []string {
	var keys []string
	for _, g := range groups {
		for _, r := range g.rows {
			keys = append(keys, rowKey(r))
		}
	}
	return keys
}

func groupLabels(groups []paletteGroup) []string {
	labels := make([]string, len(groups))
	for i, g := range groups {
		labels[i] = g.label
	}
	return labels
}

func equal(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestPaletteSessionsFlattenOpenProjectsWithTheirPaths(t *testing.T) {
	rows := paletteSessionsOf(paletteWorkspace())
	equal(t, "ids", sessionIDsOf(rows), []string{"s1", "s2", "s3"})
	if rows[0].projectID != "p1" || rows[0].projectName != "lich" || rows[0].label != "Fix flaky test" {
		t.Errorf("first row = %+v", rows[0])
	}
	if rows[0].path != "/home/u/try/skipo" || rows[1].path != "/home/u/try/skipo/frontend" {
		t.Errorf("paths = %q, %q; a worktree's own, else the project's", rows[0].path, rows[1].path)
	}
}

func TestFilterPaletteNarrowsEachKind(t *testing.T) {
	all := paletteSessionsOf(paletteWorkspace())
	ids := func(q string) []string {
		return sessionIDsOf(filterPalette(q, all, paletteOpen, nil, nil, 0, nil).sessions)
	}
	equal(t, "flaky", ids("flaky"), []string{"s1"})
	equal(t, "revu", ids("revu"), []string{"s3"})
	equal(t, "frontend", ids("frontend"), []string{"s2"})
	if r := filterPalette("", all, paletteOpen, paletteClosed, nil, 0, nil); len(r.sessions) != 3 || len(r.projects) != 2 {
		t.Errorf("an empty query keeps everything, got %d sessions and %d projects", len(r.sessions), len(r.projects))
	}
	closed := func(q string) []string {
		var out []string
		for _, p := range filterPalette(q, all, paletteOpen, paletteClosed, nil, 0, nil).closed {
			out = append(out, p.ID)
		}
		return out
	}
	equal(t, "plugin", closed("plugin"), []string{"c1"})
	equal(t, "snk", closed("snk"), []string{"c2", "c3"})
	if got := filterPalette("plugin", all, paletteOpen, nil, nil, 0, nil).closed; len(got) != 0 {
		t.Errorf("no closed projects were passed, got %v", got)
	}
}

func TestPaletteGroupsByTab(t *testing.T) {
	all := paletteSessionsOf(paletteWorkspace())
	results := filterPalette("", all, paletteOpen, paletteClosed, nil, 0, nil)
	messages := paletteMessagesOf([]lichclient.TranscriptMatch{{ID: "s1", Snippet: "flaky again", Count: 1}}, all)

	groups := paletteGroupsOf(tabAll, results, messages, 0, 0)
	equal(t, "All", groupLabels(groups), []string{"Sessions", "Projects", "Messages"})
	if g := groups[1]; len(g.rows) != 2 || g.total != 2 {
		t.Errorf("Projects = %d rows of %d, want 2 of 2", len(g.rows), g.total)
	}

	many := make([]paletteSession, 7)
	for i := range many {
		many[i] = all[0]
		many[i].sessionID = "x" + string(rune('0'+i))
	}
	cut := paletteGroupsOf(tabAll, paletteResults{sessions: many}, nil, 0, 0)
	if g := cut[0]; len(g.rows) != 5 || g.total != 7 {
		t.Errorf("a group is cut to five rows and says what it left out, got %d of %d", len(g.rows), g.total)
	}

	equal(t, "Projects", groupLabels(paletteGroupsOf(tabProjects, results, messages, 0, 0)), []string{"Open", "Closed"})
	if g := paletteGroupsOf(tabProjects, results, messages, 0, 0)[1]; len(g.rows) != 4 {
		t.Errorf("Closed lists its four whole, got %d", len(g.rows))
	}
	if g := paletteGroupsOf(tabSessions, results, messages, 0, 0)[0]; len(g.rows) != 3 {
		t.Errorf("Sessions lists its three whole, got %d", len(g.rows))
	}
}

func TestPaletteGroupReportsTheStoresCutAndNeverLessThanItsRows(t *testing.T) {
	all := paletteSessionsOf(paletteWorkspace())
	cut := filterPalette("", all, paletteOpen, paletteClosed, nil, 63, nil)
	closed := paletteGroupsOf(tabProjects, cut, nil, 0, 0)[1]
	if len(closed.rows) != 4 || closed.total != 63 {
		t.Errorf("Closed = %d of %d, want 4 of 63", len(closed.rows), closed.total)
	}
	stale := filterPalette("", all, paletteOpen, paletteClosed, nil, 1, nil)
	if g := paletteGroupsOf(tabProjects, stale, nil, 0, 0)[1]; g.total != 4 {
		t.Errorf("a stale total of 1 under four rows reads %d, want 4", g.total)
	}
}

func TestPaletteDropsEmptyGroups(t *testing.T) {
	empty := filterPalette("nothing-matches-this", paletteSessionsOf(paletteWorkspace()), paletteOpen, paletteClosed, nil, 0, nil)
	for _, tab := range []paletteTab{tabAll, tabProjects} {
		if got := paletteGroupsOf(tab, empty, nil, 0, 0); len(got) != 0 {
			t.Errorf("%s drew %v for a query that matched nothing", tab, groupLabels(got))
		}
	}
}

func TestRankSessionsPutsWaitingThenTurnsFirstAndShellsLast(t *testing.T) {
	all := paletteSessionsOf(paletteWorkspace())
	cases := []struct {
		name  string
		turns map[string]string
		want  []string
	}{
		{"a running agent leads, the shell is last", map[string]string{"s3": statusBusy}, []string{"s3", "s1", "s2"}},
		{"a waiting session leads a running one", map[string]string{"s1": statusBusy, "s3": statusWaiting}, []string{"s3", "s1", "s2"}},
		{"a waiting shell still leads", map[string]string{"s2": statusWaiting, "s3": statusBusy}, []string{"s2", "s3", "s1"}},
		{"equal rank keeps the tab order", nil, []string{"s1", "s3", "s2"}},
		{"a running shell ranks with the running", map[string]string{"s2": statusBusy}, []string{"s2", "s1", "s3"}},
	}
	for _, c := range cases {
		equal(t, c.name, sessionIDsOf(rankSessions(all, c.turns)), c.want)
	}
	before := sessionIDsOf(all)
	rankSessions(all, map[string]string{"s3": statusBusy})
	equal(t, "the list it was given", sessionIDsOf(all), before)
}

func TestRowKeysNameWhatARowOpens(t *testing.T) {
	all := paletteSessionsOf(paletteWorkspace())
	results := filterPalette("", all, paletteOpen, paletteClosed, nil, 0, nil)
	messages := paletteMessagesOf([]lichclient.TranscriptMatch{{ID: "s1", Snippet: "x", Count: 1}}, all)
	equal(t, "keys", rowKeys(paletteGroupsOf(tabAll, results, messages, 0, 0)), []string{"s1", "s2", "s3", "p1", "p2", "s1"})
}

func TestPaletteTabCountsAndWraps(t *testing.T) {
	results := filterPalette("", paletteSessionsOf(paletteWorkspace()), paletteOpen, paletteClosed, nil, 0, nil)
	for _, c := range []struct {
		tab  paletteTab
		want int
		ok   bool
	}{{tabProjects, 6, true}, {tabSessions, 3, true}, {tabMessages, 0, true}, {tabAll, 0, false}} {
		if got, ok := paletteTabCount(c.tab, results, nil); got != c.want || ok != c.ok {
			t.Errorf("%s count = %d, %v; want %d, %v", c.tab, got, ok, c.want, c.ok)
		}
	}
	for _, c := range []struct {
		from paletteTab
		step int
		want paletteTab
	}{{tabAll, 1, tabSessions}, {tabMessages, 1, tabHistory}, {tabHistory, 1, tabAll}, {tabAll, -1, tabHistory}} {
		if got := nextTab(c.from, c.step); got != c.want {
			t.Errorf("nextTab(%s, %d) = %s, want %s", c.from, c.step, got, c.want)
		}
	}
}

func TestPaletteMessagesCarryTheirSessionAndSurviveAClose(t *testing.T) {
	all := paletteSessionsOf(paletteWorkspace())
	rows := paletteMessagesOf([]lichclient.TranscriptMatch{{ID: "s3", Snippet: "wire the auth middleware", Count: 2}}, all)
	want := paletteMessage{paletteSession: paletteSession{sessionID: "s3", projectID: "p2", projectName: "revu",
		label: "Wire revu auth middleware", kind: "claude", path: "/home/u/try/revu"}, snippet: "wire the auth middleware", count: 2}
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
	ordered := paletteMessagesOf([]lichclient.TranscriptMatch{{ID: "s3"}, {ID: "s1"}}, all)
	if ordered[0].sessionID != "s3" || ordered[1].sessionID != "s1" {
		t.Errorf("the order the search returned is kept, got %+v", ordered)
	}
	dropped := paletteMessagesOf([]lichclient.TranscriptMatch{{ID: "s9"}, {ID: "s1"}}, all)
	if len(dropped) != 1 || dropped[0].sessionID != "s1" {
		t.Errorf("a match whose session is gone is dropped, got %+v", dropped)
	}
	if got := paletteMessagesOf(nil, all); got != nil {
		t.Errorf("no matches is no rows, got %+v", got)
	}
}

var paletteParked = []lichclient.ClosedSession{
	{ID: "h1", ProjectID: "p1", ProjectName: "lich", ProjectPath: "/home/u/try/skipo", Label: "Wire the relay inbox", Kind: "claude",
		Path: "/home/u/wt/lich/relay-inbox", ParkedBranch: "feat/relay-inbox", ClosedAt: 1_700_000_200},
	{ID: "h2", ProjectID: "p1", ProjectName: "lich", ProjectPath: "/home/u/try/skipo", Label: "Conpty handle recycling", Kind: "claude",
		Path: "/home/u/wt/lich/conpty", ParkedBranch: "fix/conpty-first-try", ClosedAt: 1_700_000_100},
	{ID: "h3", ProjectID: "p2", ProjectName: "revu", ProjectPath: "/home/u/try/revu", Label: "vitest --ui", Kind: "shell",
		Path: "/home/u/wt/revu/gone", ParkedBranch: "chore/vitest-ui", ClosedAt: 1_700_000_000},
}

var paletteBranches = map[string]string{
	"/home/u/wt/lich/relay-inbox": "feat/relay-inbox",
	"/home/u/wt/lich/conpty":      "fix/conpty-handle",
}

func historyIDs(rows []paletteHistory) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func TestHistoryRowsHangTheLiveBranchAndMarkGoneCheckouts(t *testing.T) {
	rows := historyRows(paletteParked, paletteBranches, map[string]bool{"/home/u/wt/revu/gone": true})
	for i, want := range []struct {
		branch string
		gone   bool
	}{{"feat/relay-inbox", false}, {"fix/conpty-handle", false}, {"", true}} {
		if rows[i].branch != want.branch || rows[i].gone != want.gone {
			t.Errorf("%s = %q gone=%v, want %q gone=%v", rows[i].ID, rows[i].branch, rows[i].gone, want.branch, want.gone)
		}
	}
	bare := historyRows(paletteParked[:1], nil, nil)
	if bare[0].branch != "" || bare[0].gone {
		t.Errorf("a checkout that is present but nameless is not gone: %+v", bare[0])
	}
	if got := historyRows(nil, nil, nil); len(got) != 0 {
		t.Errorf("a store that answered before git did: %+v", got)
	}
}

func TestHistorySearchKeepsWhatTheStoreMatched(t *testing.T) {
	rows := historyRows(paletteParked, paletteBranches, nil)
	by := func(q string, in []paletteHistory) []string {
		return historyIDs(filterPalette(q, nil, nil, nil, in, 0, nil).history)
	}
	equal(t, "live branch", by("conpty-handle", rows), []string{"h2"})
	equal(t, "parked branch", by("first-try", rows), []string{"h2"})
	equal(t, "parked branch of a gone checkout", by("vitest-ui", rows), []string{"h3"})
	equal(t, "label", by("relay", rows), []string{"h1"})
	equal(t, "project", by("revu", rows), []string{"h3"})
	equal(t, "path", by("wt/lich", rows), []string{"h1", "h2"})
	equal(t, "two tokens narrow further", by("lich relay", rows), []string{"h1"})
	equal(t, "a token that is absent", by("lich nothing", rows), nil)
	equal(t, "no history loaded", by("relay", nil), nil)

	said := paletteParked[1]
	said.MatchedConversation, said.Snippet = true, "…the ConPTY handle was recycled…"
	kept := historyRows([]lichclient.ClosedSession{said}, paletteBranches, nil)
	equal(t, "matched on its conversation", by("recycled zombie", kept), []string{"h2"})
	said.Snippet = ""
	folded := historyRows([]lichclient.ClosedSession{said}, paletteBranches, nil)
	equal(t, "matched with no snippet to show", by("decision", folded), []string{"h2"})
}

func TestHistoryTabLists(t *testing.T) {
	rows := historyRows(paletteParked, paletteBranches, nil)
	results := filterPalette("", nil, paletteOpen, paletteClosed, rows, 0, nil)
	groups := paletteGroupsOf(tabHistory, results, nil, 0, 0)
	equal(t, "labels", groupLabels(groups), []string{"Closed sessions"})
	equal(t, "rows", rowKeys(groups), []string{"h1", "h2", "h3"})
	for _, g := range paletteGroupsOf(tabAll, results, nil, 0, 0) {
		for _, r := range g.rows {
			if r.kind == rowHistory {
				t.Errorf("All lists a history row, the mid-work posture's tab")
			}
		}
	}
	if n, _ := paletteTabCount(tabHistory, results, nil); n != 3 {
		t.Errorf("History counts %d, want 3", n)
	}
	if got := paletteGroupsOf(tabHistory, filterPalette("", nil, paletteOpen, paletteClosed, nil, 0, nil), nil, 0, 0); len(got) != 0 {
		t.Errorf("nothing closed draws no group, got %v", groupLabels(got))
	}
	if g := paletteGroupsOf(tabHistory, results, nil, 143, 0)[0]; len(g.rows) != 3 || g.total != 143 {
		t.Errorf("a cut page reads %d of %d, want 3 of 143", len(g.rows), g.total)
	}
	for _, c := range []struct {
		total, indexing int
		note            string
		wantTotal       int
	}{{143, 12, "indexing 12 sessions", 143}, {143, 1, "indexing 1 session", 143}, {143, 0, "", 143}, {3, 0, "", 3}, {0, 0, "", 3}} {
		g := paletteGroupsOf(tabHistory, results, nil, c.total, c.indexing)[0]
		if g.note != c.note || g.total != c.wantTotal {
			t.Errorf("total=%d indexing=%d: note %q total %d, want %q %d", c.total, c.indexing, g.note, g.total, c.note, c.wantTotal)
		}
	}
}

func TestHistoryIndexNoteAndAction(t *testing.T) {
	rows := historyRows(paletteParked, paletteBranches, map[string]bool{"/home/u/wt/revu/gone": true})
	if got := historyIndexNote(rows[0]); got != "" {
		t.Errorf("a session indexed whole says nothing, got %q", got)
	}
	cut := rows[0]
	cut.Truncated = true
	if got := historyIndexNote(cut); got != "indexed: newest 8 MB" {
		t.Errorf("note = %q", got)
	}
	if historyAction(rows[0]) != historyResume || historyAction(rows[2]) != historyForget {
		t.Error("a checkout that is there resumes and one that is gone forgets")
	}
}

var paletteOutside = []lichclient.ExternalSession{
	{Kind: "codex", ProviderSessionID: "c1", Title: "Port the theme loader", Path: "/wt/theme-port", ProjectID: "p1", ProjectName: "lich", ProjectPath: "/src/lich", UpdatedAt: 1788532932},
	{Kind: "kiro", ProviderSessionID: "k1", ProjectID: "p2", ProjectName: "sop", ProjectPath: "/src/sop", UpdatedAt: 1788000000},
}

func TestConversationsStartedOutsideLich(t *testing.T) {
	rows := historyRows(paletteParked, paletteBranches, nil)
	results := filterPalette("", nil, paletteOpen, paletteClosed, rows, 0, paletteOutside)
	groups := paletteGroupsOf(tabHistory, results, nil, 0, 0)
	equal(t, "labels", groupLabels(groups), []string{"Closed sessions", "Outside lich"})
	equal(t, "outside rows", rowKeys(groups[1:]), []string{"external:c1", "external:k1"})
	if n, _ := paletteTabCount(tabHistory, results, nil); n != len(rows)+len(paletteOutside) {
		t.Errorf("History counts %d, want the closed sessions and the outside ones together", n)
	}
	by := func(q string) []string {
		var out []string
		for _, e := range filterPalette(q, nil, nil, nil, nil, 0, paletteOutside).external {
			out = append(out, e.ProviderSessionID)
		}
		return out
	}
	equal(t, "title", by("theme"), []string{"c1"})
	equal(t, "project", by("sop"), []string{"k1"})
	equal(t, "the project's own directory", by("/src/sop"), []string{"k1"})
	equal(t, "directory tokens", by("wt theme-port"), []string{"c1"})
	equal(t, "nothing", by("nothing"), nil)

	if externalLabel(paletteOutside[0]) != "Port the theme loader" || externalLabel(paletteOutside[1]) != "Untitled conversation" {
		t.Error("an untitled conversation is named rather than drawn empty")
	}
	blank := paletteOutside[0]
	blank.Title = "   "
	if externalLabel(blank) != "Untitled conversation" {
		t.Error("a blank title is untitled")
	}
	want := lichclient.ClosedSession{ID: "s1", ProjectID: "p1", ProjectName: "lich", ProjectPath: "/src/lich",
		Label: "Port the theme loader", Kind: "codex", Path: "/wt/theme-port"}
	if got := adoptedSession(paletteOutside[0], "s1"); got != want {
		t.Errorf("adopted = %+v, want %+v", got, want)
	}
}

func TestActionTermReadsAfterALeadingPrefix(t *testing.T) {
	for _, c := range []struct {
		query, term string
		ok          bool
	}{{">", "", true}, {"> pin", " pin", true}, {"", "", false}, {"fix > later", "", false}, {" >pin", "", false}} {
		if term, ok := actionTerm(c.query); ok != c.ok || (ok && term != c.term) {
			t.Errorf("actionTerm(%q) = %q, %v; want %q, %v", c.query, term, ok, c.term, c.ok)
		}
	}
}

func TestActionGroupsListChordsAsBound(t *testing.T) {
	all := func(hotkeyID) bool { return true }
	lines := func(groups []paletteGroup) []string {
		var out []string
		for _, g := range groups {
			for _, r := range g.rows {
				out = append(out, g.label+": "+string(r.action)+" "+r.keys)
			}
		}
		return out
	}
	equal(t, "pin", lines(actionGroups(" pin", defaultHotkeys(), false, all)), []string{"Sessions: togglePin Ctrl+Shift+K"})

	rebound := defaultHotkeys()
	rebound[hotkeySettings] = combo{Mod: true, Shift: true, Key: "o"}
	equal(t, "mac", lines(actionGroups("settings", rebound, true, all)), []string{"App: settings ⌘⇧O"})

	unbound := defaultHotkeys()
	unbound[hotkeyToggleDock] = comboUnassigned
	equal(t, "unassigned", lines(actionGroups("dock", unbound, false, all)), []string{"View: toggleDock Unassigned"})

	var ids []string
	for _, g := range actionGroups("", defaultHotkeys(), false, all) {
		for _, r := range g.rows {
			ids = append(ids, string(r.action))
		}
	}
	if slices.Contains(ids, "commandPalette") || !slices.Contains(ids, "searchInFiles") {
		t.Errorf("the palette's own action is left out and the rest listed, got %v", ids)
	}
	if got := actionGroups("zzz", defaultHotkeys(), false, all); len(got) != 0 {
		t.Errorf("nothing matches, got %v", groupLabels(got))
	}
	equal(t, "key", rowKeys(actionGroups(" pin", defaultHotkeys(), false, all)), []string{"action:togglePin"})
}

func TestActionGroupsLeaveOutWhatThisWindowCannotRun(t *testing.T) {
	only := func(id hotkeyID) bool { return id == hotkeyTogglePin }
	got := actionGroups("", defaultHotkeys(), false, only)
	equal(t, "keys", rowKeys(got), []string{"action:togglePin"})
}

func TestAgoLabelPhrasesTheUnit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	at := func(d time.Duration) int64 { return now.Add(-d).Unix() }
	for _, c := range []struct {
		at   int64
		want string
	}{
		{at(2 * time.Hour), "2h ago"},
		{at(3 * 24 * time.Hour), "3d ago"},
		{at(21 * 24 * time.Hour), "3w ago"},
		{at(59*time.Minute + 59*time.Second), "59m ago"},
		{at(5 * time.Second), "just now"},
		{0, ""},
		{-1, ""},
	} {
		if got := agoLabel(c.at, now); got != c.want {
			t.Errorf("agoLabel(%d) = %q, want %q", c.at, got, c.want)
		}
	}
}

func TestQuerySegmentsMarksEveryToken(t *testing.T) {
	hits := func(segs []segment) []string {
		var out []string
		for _, s := range segs {
			if s.hit {
				out = append(out, s.text)
			}
		}
		return out
	}
	equal(t, "a token that is absent marks nothing of its own", hits(querySegments("…the ConPTY handle was recycled…", "recycled zombie")), []string{"recycled"})
	equal(t, "two tokens, either case", hits(querySegments("…the ConPTY handle was recycled…", "conpty HANDLE")), []string{"ConPTY", "handle"})
	equal(t, "no query", hits(querySegments("plain", "")), nil)
}
