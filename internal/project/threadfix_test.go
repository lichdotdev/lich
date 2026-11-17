package project

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// memFixes stands in for the workspace store, the database boundary.
type memFixes struct{ list []api.ThreadFix }

func (m *memFixes) ThreadFixes() ([]api.ThreadFix, error) { return slices.Clone(m.list), nil }

func (m *memFixes) SaveThreadFix(f api.ThreadFix) error {
	m.list = slices.DeleteFunc(m.list, func(o api.ThreadFix) bool { return o.ThreadID == f.ThreadID })
	m.list = append(m.list, f)
	return nil
}

func (m *memFixes) DeleteThreadFix(id string) error {
	m.list = slices.DeleteFunc(m.list, func(o api.ThreadFix) bool { return o.ThreadID == id })
	return nil
}

type emitted struct{ names []string }

func (e *emitted) emit(name string, _ any) { e.names = append(e.names, name) }

func withFixes(gh *fakeGH, fixes *memFixes, ev *emitted) *Service {
	s := withGH(gh)
	s.SetThreadFixes(fixes)
	s.SetEmit(ev.emit)
	return s
}

// fixedRepo is a checkout whose history holds a fix and a head past it, and a
// commit off to the side that no head here contains.
func fixedRepo(t *testing.T) (dir, fix, head, aside string) {
	t.Helper()
	dir, git := initRepo(t)
	commitIn(t, dir, git, "fix")
	fix = strings.TrimSpace(git("rev-parse", "HEAD"))
	commitIn(t, dir, git, "later")
	head = strings.TrimSpace(git("rev-parse", "HEAD"))
	git("checkout", "-q", "-b", "aside", "HEAD~2")
	commitIn(t, dir, git, "aside")
	aside = strings.TrimSpace(git("rev-parse", "HEAD"))
	git("checkout", "-q", "main")
	return dir, fix, head, aside
}

func openPR(head string) []byte {
	return []byte(`{"headRefOid":"` + head + `","state":"OPEN"}`)
}

func TestAwaitThreadFixWaitsWithoutAWord(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{out: openPR("abc1234567")}
	fixes, ev := &memFixes{}, &emitted{}
	opts := api.ThreadFixOptions{Path: "/repo", Number: 42, ThreadID: "PRRT_1", CommentID: 900001}
	if _, err := withFixes(gh, fixes, ev).AwaitThreadFix(t.Context(), opts); err != nil {
		t.Fatalf("AwaitThreadFix: %v", err)
	}
	if gh.calls != 1 || !slices.Contains(gh.history[0], "view") {
		t.Errorf("gh calls = %v, want the pull request read and no reply: the thread may be a question", gh.history)
	}
	want := []api.ThreadFix{{ThreadID: "PRRT_1", Path: "/repo", Number: 42, CommentID: 900001}}
	if !slices.Equal(fixes.list, want) {
		t.Errorf("waiting = %+v, want %+v", fixes.list, want)
	}
	if !slices.Contains(ev.names, api.ThreadFixesEvent.Name) {
		t.Errorf("events = %v, want %s", ev.names, api.ThreadFixesEvent.Name)
	}
}

func TestAwaitThreadFixRefusals(t *testing.T) {
	t.Parallel()
	open := api.ThreadFixOptions{Path: "/repo", Number: 42, ThreadID: "PRRT_1", CommentID: 1}
	t.Run("a closed pull request", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"headRefOid":"abc","state":"MERGED"}`)}
		fixes := &memFixes{}
		if _, err := withFixes(gh, fixes, &emitted{}).AwaitThreadFix(t.Context(), open); err == nil {
			t.Error("expected an error for a merged pull request")
		}
		if gh.calls != 1 || len(fixes.list) != 0 {
			t.Errorf("gh calls = %d, waiting = %v; want only the read and nothing waiting", gh.calls, fixes.list)
		}
	})
	t.Run("no thread never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		bad := open
		bad.ThreadID = ""
		if _, err := withFixes(gh, &memFixes{}, &emitted{}).AwaitThreadFix(t.Context(), bad); err == nil || gh.calls != 0 {
			t.Errorf("err = %v, gh calls = %d; want an error and no call", err, gh.calls)
		}
	})
}

func TestMarkThreadFixedNamesTheFullCommit(t *testing.T) {
	t.Parallel()
	dir, fix, _, _ := fixedRepo(t)
	fixes := &memFixes{list: []api.ThreadFix{{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11}}}
	ev := &emitted{}
	if _, err := withFixes(&fakeGH{}, fixes, ev).MarkThreadFixed(t.Context(), api.MarkThreadFixedOptions{ThreadID: "PRRT_1", Commit: fix[:shortSHA]}); err != nil {
		t.Fatalf("MarkThreadFixed: %v", err)
	}
	if got := fixes.list[0].Commit; got != fix {
		t.Errorf("commit = %q, want the full %q", got, fix)
	}
	if !slices.Contains(ev.names, api.ThreadFixesEvent.Name) {
		t.Errorf("events = %v, want %s", ev.names, api.ThreadFixesEvent.Name)
	}
}

func TestMarkThreadFixedRefusals(t *testing.T) {
	t.Parallel()
	dir, fix, _, _ := fixedRepo(t)
	sent := api.ThreadFix{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11}
	for name, opts := range map[string]api.MarkThreadFixedOptions{
		"a thread never sent from the Pulls screen": {ThreadID: "PRRT_9", Commit: fix},
		"a commit the checkout does not hold":       {ThreadID: "PRRT_1", Commit: "0123456789abcdef"},
		"an option in place of a commit":            {ThreadID: "PRRT_1", Commit: "--all"},
		"a name instead of a sha":                   {ThreadID: "PRRT_1", Commit: "HEAD"},
		"no commit":                                 {ThreadID: "PRRT_1"},
	} {
		t.Run(name, func(t *testing.T) {
			fixes := &memFixes{list: []api.ThreadFix{sent}}
			if _, err := withFixes(&fakeGH{}, fixes, &emitted{}).MarkThreadFixed(t.Context(), opts); err == nil {
				t.Error("expected an error")
			}
			if fixes.list[0].Commit != "" {
				t.Errorf("commit = %q, want the thread left unmarked", fixes.list[0].Commit)
			}
		})
	}
}

func TestCheckThreadFixesResolvesOnceTheHeadHoldsTheFix(t *testing.T) {
	t.Parallel()
	dir, fix, head, _ := fixedRepo(t)
	gh := &fakeGH{out: openPR(head)}
	fixes := &memFixes{list: []api.ThreadFix{
		{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11, Commit: fix},
		{ThreadID: "PRRT_2", Path: dir, Number: 42, CommentID: 12, Commit: head},
	}}
	ev := &emitted{}
	withFixes(gh, fixes, ev).checkThreadFixes(slices.Clone(fixes.list))
	reads := 0
	for _, call := range gh.history {
		if slices.Contains(call, "view") {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("pull request reads = %d, want one for both threads", reads)
	}
	joined := strings.Join(slices.Concat(gh.history...), " ")
	for _, want := range []string{"id=PRRT_1", "id=PRRT_2", "comments/11/replies", "comments/12/replies", "body=Fixed in " + fix[:shortSHA] + ".", "body=Fixed in " + head[:shortSHA] + "."} {
		if !strings.Contains(joined, want) {
			t.Errorf("gh calls lack %q: %v", want, gh.history)
		}
	}
	if len(fixes.list) != 0 {
		t.Errorf("waiting = %v, want nothing", fixes.list)
	}
	if !slices.Contains(ev.names, api.ThreadFixesEvent.Name) {
		t.Errorf("events = %v, want %s", ev.names, api.ThreadFixesEvent.Name)
	}
}

func TestCheckThreadFixesKeepsWaiting(t *testing.T) {
	t.Parallel()
	dir, fix, head, aside := fixedRepo(t)
	marked := api.ThreadFix{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11, Commit: fix}
	t.Run("a thread nobody marked, whatever was pushed", func(t *testing.T) {
		gh := &fakeGH{out: openPR(head)}
		unmarked := marked
		unmarked.Commit = ""
		fixes, ev := &memFixes{list: []api.ThreadFix{unmarked}}, &emitted{}
		withFixes(gh, fixes, ev).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 1 || len(fixes.list) != 1 || len(ev.names) != 0 {
			t.Errorf("gh calls = %v, waiting = %v, events = %v; want one read, still waiting, no event", gh.history, fixes.list, ev.names)
		}
	})
	t.Run("a head that does not hold the fix", func(t *testing.T) {
		gh := &fakeGH{out: openPR(aside)}
		fixes := &memFixes{list: []api.ThreadFix{marked}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 1 || len(fixes.list) != 1 {
			t.Errorf("gh calls = %v, waiting = %v; want one read, still waiting", gh.history, fixes.list)
		}
	})
	t.Run("a head this checkout has not fetched", func(t *testing.T) {
		gh := &fakeGH{out: openPR("0123456789abcdef0123456789abcdef01234567")}
		fixes := &memFixes{list: []api.ThreadFix{marked}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 1 || len(fixes.list) != 1 {
			t.Errorf("gh calls = %v, waiting = %v; want one read, still waiting", gh.history, fixes.list)
		}
	})
	t.Run("when the resolve fails, with no reply sent", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{openPR(head)}, errs: []error{nil, errors.New("HTTP 502")}}
		fixes := &memFixes{list: []api.ThreadFix{marked}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 2 || len(fixes.list) != 1 {
			t.Errorf("gh calls = %d, waiting = %v; want the read and the resolve, still waiting", gh.calls, fixes.list)
		}
	})
	t.Run("when the read fails", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("offline")}
		fixes := &memFixes{list: []api.ThreadFix{marked, {ThreadID: "PRRT_2", Path: dir, Number: 42, CommentID: 12, Commit: fix}}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 1 || len(fixes.list) != 2 {
			t.Errorf("gh calls = %d, waiting = %v; want one read for the pull request, both still waiting", gh.calls, fixes.list)
		}
	})
}

func TestThreadFixWatchReadsOnlyMarkedPullRequestsAfterItsFirstPass(t *testing.T) {
	t.Parallel()
	fixes := []api.ThreadFix{
		{ThreadID: "PRRT_1", Path: "/repo", Number: 42},
		{ThreadID: "PRRT_2", Path: "/repo", Number: 43, Commit: "abc"},
	}
	if got := markedFixes(fixes); !slices.Equal(got, fixes[1:]) {
		t.Errorf("markedFixes = %+v, want only the marked one", got)
	}
}

func TestCheckThreadFixesDropsWhatCanNoLongerBeFixed(t *testing.T) {
	t.Parallel()
	t.Run("a merged pull request, marked or not", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"headRefOid":"abc","state":"MERGED"}`)}
		dir := t.TempDir()
		fixes := &memFixes{list: []api.ThreadFix{{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11, Commit: "abc"}, {ThreadID: "PRRT_2", Path: dir, Number: 42, CommentID: 12}}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 1 || len(fixes.list) != 0 {
			t.Errorf("gh calls = %d, waiting = %v; want the read only and nothing waiting", gh.calls, fixes.list)
		}
	})
	t.Run("a checkout that is gone", func(t *testing.T) {
		gh := &fakeGH{}
		fixes := &memFixes{list: []api.ThreadFix{{ThreadID: "PRRT_1", Path: t.TempDir() + "/gone", Number: 42, CommentID: 11}}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes(slices.Clone(fixes.list))
		if gh.calls != 0 || len(fixes.list) != 0 {
			t.Errorf("gh calls = %d, waiting = %v; want no call and nothing waiting", gh.calls, fixes.list)
		}
	})
}

func TestThreadFixesUnwired(t *testing.T) {
	t.Parallel()
	s := withGH(&fakeGH{})
	if got, err := s.ThreadFixes(t.Context(), api.None{}); err != nil || got != nil {
		t.Errorf("ThreadFixes() = %v, %v; want nothing", got, err)
	}
	if _, err := s.AwaitThreadFix(t.Context(), api.ThreadFixOptions{Path: "/repo", Number: 1, ThreadID: "PRRT_1", CommentID: 1}); err == nil {
		t.Error("expected an error with nowhere to keep the thread")
	}
	if _, err := s.MarkThreadFixed(t.Context(), api.MarkThreadFixedOptions{ThreadID: "PRRT_1", Commit: "abc"}); err == nil {
		t.Error("expected an error with no thread kept")
	}
}
