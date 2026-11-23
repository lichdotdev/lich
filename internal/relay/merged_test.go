package relay

import (
	"errors"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/store"
)

// merging is a project with sessions spread over its own directory and two
// worktrees, one of them the merged branch's.
func merging() fakeSessions {
	return fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "root", Label: "main", Kind: "claude"},
			{ID: "agent", Label: "feat", Kind: "codex", Path: "/wt/feat"},
			{ID: "modded", Label: "feat-claude", Kind: "claude", Path: "/wt/feat"},
			{ID: "shell", Label: "feat-shell", Kind: "shell", Path: "/wt/feat"},
			{ID: "parked", Label: "feat-parked", Kind: "claude", Path: "/wt/feat"},
			{ID: "other", Label: "fix", Kind: "claude", Path: "/wt/fix"},
		}},
	}}
}

func TestAnnounceMergeTellsTheAgentsInTheMergedCheckout(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("root", "agent", "modded", "shell", "other")
	term.openChat("modded", acked)
	svc := newRelay(merging(), term, nil)

	if _, err := svc.AnnounceMerge(t.Context(), api.AnnounceMergeOptions{Checkout: "/wt/feat", Number: 42, Title: "Add merge notice", Branch: "feat/merge", Base: "main"}); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}

	if !awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want the merge notice", term.written("agent"))
	}
	if !awaitPrompts(term, "modded", 1) {
		t.Fatal("modded's chat was handed nothing, want the merge notice")
	}
	if got := term.promptsTo("modded")[0]; !strings.Contains(got, "#42") {
		t.Fatalf("modded's chat was handed %q, want the merge notice", got)
	}
	if got := term.written("modded"); got != "" {
		t.Fatalf("typed at modded = %q, want nothing: its chat took the notice", got)
	}
	// Who hears it is decided before AnnounceMerge returns; only the delivery
	// itself runs behind it.
	for _, id := range []string{"root", "shell", "parked", "other"} {
		if got := term.written(id); got != "" {
			t.Errorf("typed at %s = %q, want nothing", id, got)
		}
	}
}

func TestAnnounceMergeReachesSessionsInTheProjectsOwnDirectory(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("root", "agent")
	svc := newRelay(merging(), term, nil)

	if _, err := svc.AnnounceMerge(t.Context(), api.AnnounceMergeOptions{Checkout: "/src/lich", Number: 7, Title: "Fix", Branch: "fix/x", Base: "main"}); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}

	if !awaitWritten(term, "root", "#7") {
		t.Fatalf("typed at root = %q, want the merge notice", term.written("root"))
	}
	if got := term.written("agent"); got != "" {
		t.Fatalf("typed at agent = %q, want nothing", got)
	}
}

func TestAnnounceMergeTellsTheSessionThatOpenedOneThere(t *testing.T) {
	t.Parallel()
	sessions := fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "lead", Label: "main", Kind: "claude"},
			{ID: "bystander", Label: "other", Kind: "codex", Path: "/wt/other"},
		}},
		{ID: "p2", Name: "site", Path: "/src/site", Sessions: []store.Session{
			{ID: "worker", Label: "feat", Kind: "codex", Path: "/wt/feat", OriginSessionID: "lead"},
			{ID: "parked", Label: "feat-2", Kind: "codex", Path: "/wt/feat", OriginSessionID: "lead"},
			{ID: "nested", Label: "feat-3", Kind: "codex", Path: "/wt/feat", OriginSessionID: "worker"},
		}},
	}}
	term := newFakeTerminal("lead", "bystander", "worker", "nested")
	svc := newRelay(sessions, term, nil)

	if _, err := svc.AnnounceMerge(t.Context(), api.AnnounceMergeOptions{Checkout: "/wt/feat", Number: 42, Title: "Add merge notice", Branch: "feat/merge", Base: "main"}); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}

	for _, id := range []string{"lead", "worker", "nested"} {
		if !awaitWritten(term, id, "#42") {
			t.Fatalf("typed at %s = %q, want the merge notice", id, term.written(id))
		}
	}
	if got := strings.Count(term.written("lead"), "#42"); got != 1 {
		t.Fatalf("lead heard the notice %d times, want once for its two sessions there", got)
	}
	if got := strings.Count(term.written("worker"), "#42"); got != 1 {
		t.Fatalf("worker heard the notice %d times, want once: it runs there and opened nested", got)
	}
	if got := term.written("bystander"); got != "" {
		t.Fatalf("typed at bystander = %q, want nothing", got)
	}
}

func TestAnnounceMergeReportsAStoreFailure(t *testing.T) {
	t.Parallel()
	svc := newRelay(fakeSessions{err: errors.New("disk gone")}, newFakeTerminal(), nil)

	if _, err := svc.AnnounceMerge(t.Context(), api.AnnounceMergeOptions{Checkout: "/wt/feat", Number: 1, Title: "x", Branch: "b", Base: "main"}); err == nil {
		t.Fatal("AnnounceMerge = nil, want the store failure")
	}
}

// A merge is a merged envelope, which the briefing says is news and not a task,
// and carries no prompt.
func TestMergeNoticeNamesThePullRequestAndAsksForNoWork(t *testing.T) {
	t.Parallel()
	got := mergedEnvelope(prompt.English, api.ForgeGitHub, 42, "Title \x1b[201~with escape", "feat/merge", "main").String()

	for _, want := range []string{`kind="merged"`, `pr="42"`, `branch="feat/merge"`, "#42", "merged into main"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "\x1b") || strings.Contains(got, "<prompt>") {
		t.Errorf("notice %q carries an escape out of the title, or a task", got)
	}
}

// The Pulls screen announces its own merge and the pull request watch reads it
// merged a moment later: the agents hear it once.
func TestAnnounceMergeTellsAMergeOnce(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("agent")
	svc := newRelay(merging(), term, nil)
	opts := api.AnnounceMergeOptions{Checkout: "/wt/feat", Number: 42, Title: "Add merge notice", Branch: "feat/merge", Base: "main"}

	for range 2 {
		if _, err := svc.AnnounceMerge(t.Context(), opts); err != nil {
			t.Fatalf("AnnounceMerge: %v", err)
		}
	}

	if !awaitWritten(term, "agent", "#42") {
		t.Fatalf("typed at agent = %q, want the merge notice", term.written("agent"))
	}
	opts.Number = 43
	if _, err := svc.AnnounceMerge(t.Context(), opts); err != nil {
		t.Fatalf("AnnounceMerge: %v", err)
	}
	if !awaitWritten(term, "agent", "#43") {
		t.Fatalf("typed at agent = %q, want the second pull request's notice", term.written("agent"))
	}
	if got := strings.Count(term.written("agent"), "#42"); got != 1 {
		t.Fatalf("agent heard #42 %d times, want once", got)
	}
}

func TestAGitLabNoticeNamesTheMergeRequest(t *testing.T) {
	t.Parallel()
	failedOn := api.SessionWatch{On: api.WatchChecksFailed}
	passedOn := api.SessionWatch{On: api.WatchChecksPassed}
	gitlab := api.PRRead{Number: 7, Head: "abc", Forge: api.ForgeGitLab}
	en := map[string]string{
		mergedEnvelope(prompt.English, api.ForgeGitLab, 42, "Title", "feat/merge", "main").String(): "Merge request !42",
		watchEnvelope(prompt.English, failedOn, gitlab, "feat/merge").String():                      "merge request !7",
		watchEnvelope(prompt.English, passedOn, gitlab, "feat/merge").String():                      "merge request !7",
	}
	for notice, want := range en {
		if !strings.Contains(notice, want) || strings.Contains(notice, "pull request") {
			t.Errorf("notice %q wants %q and no pull request", notice, want)
		}
	}
	for _, lang := range prompt.Langs {
		for name, notice := range map[string]string{
			"merge":  mergedEnvelope(lang, api.ForgeGitLab, 42, "Title", "feat/merge", "main").String(),
			"failed": watchEnvelope(lang, failedOn, gitlab, "feat/merge").String(),
			"passed": watchEnvelope(lang, passedOn, gitlab, "feat/merge").String(),
		} {
			ref := map[string]string{"merge": "42", "failed": "7", "passed": "7"}[name]
			if !strings.Contains(notice, "!"+ref) || strings.Contains(notice, "#"+ref) {
				t.Errorf("%s %s: want !%s and no #%s:\n%s", lang, name, ref, ref, notice)
			}
		}
		github := watchEnvelope(lang, failedOn, api.PRRead{Number: 7, Head: "abc"}, "feat/merge").String()
		if !strings.Contains(github, "#7") {
			t.Errorf("%s: a GitHub watch notice wants #7:\n%s", lang, github)
		}
	}
}
