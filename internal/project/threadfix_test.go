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

func TestAwaitThreadFixRepliesAndWaitsOnTheHeadItReads(t *testing.T) {
	t.Parallel()
	gh := &fakeGH{outs: [][]byte{[]byte(`{"headRefOid":"abc1234567","state":"OPEN"}`)}}
	fixes, ev := &memFixes{}, &emitted{}
	opts := api.ThreadFixOptions{Path: "/repo", Number: 42, ThreadID: "PRRT_1", CommentID: 900001}
	if _, err := withFixes(gh, fixes, ev).AwaitThreadFix(t.Context(), opts); err != nil {
		t.Fatalf("AwaitThreadFix: %v", err)
	}
	if want := []string{"pr", "view", "42", "--json", "headRefOid,state"}; !slices.Equal(gh.history[0], want) {
		t.Errorf("read = %v, want %v", gh.history[0], want)
	}
	if reply := gh.history[1]; !slices.Contains(reply, "repos/{owner}/{repo}/pulls/42/comments/900001/replies") || !slices.Contains(reply, "body="+threadFixingReply) {
		t.Errorf("reply = %v, want the fixing reply on the thread's comment", reply)
	}
	want := []api.ThreadFix{{ThreadID: "PRRT_1", Path: "/repo", Number: 42, CommentID: 900001, Head: "abc1234567"}}
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
	t.Run("a closed pull request never gets the reply", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"headRefOid":"abc","state":"MERGED"}`)}
		fixes := &memFixes{}
		if _, err := withFixes(gh, fixes, &emitted{}).AwaitThreadFix(t.Context(), open); err == nil {
			t.Error("expected an error for a merged pull request")
		}
		if gh.calls != 1 || len(fixes.list) != 0 {
			t.Errorf("gh calls = %d, waiting = %v; want only the read and nothing waiting", gh.calls, fixes.list)
		}
	})
	t.Run("a failed reply leaves nothing waiting", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{[]byte(`{"headRefOid":"abc","state":"OPEN"}`)}, errs: []error{nil, errors.New("HTTP 403")}}
		fixes := &memFixes{}
		if _, err := withFixes(gh, fixes, &emitted{}).AwaitThreadFix(t.Context(), open); err == nil {
			t.Error("expected the reply's error")
		}
		if len(fixes.list) != 0 {
			t.Errorf("waiting = %v, want nothing", fixes.list)
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

func TestCheckThreadFixesResolvesOnANewHead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gh := &fakeGH{outs: [][]byte{[]byte(`{"headRefOid":"def4567890","state":"OPEN"}`)}}
	fixes := &memFixes{list: []api.ThreadFix{
		{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11, Head: "abc"},
		{ThreadID: "PRRT_2", Path: dir, Number: 42, CommentID: 12, Head: "abc"},
	}}
	ev := &emitted{}
	withFixes(gh, fixes, ev).checkThreadFixes()
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
	for _, want := range []string{"id=PRRT_1", "id=PRRT_2", "comments/11/replies", "comments/12/replies", "body=Fixed in def4567."} {
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
	dir := t.TempDir()
	waiting := api.ThreadFix{ThreadID: "PRRT_1", Path: dir, Number: 42, CommentID: 11, Head: "abc"}
	t.Run("on the same head", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"headRefOid":"abc","state":"OPEN"}`)}
		fixes, ev := &memFixes{list: []api.ThreadFix{waiting}}, &emitted{}
		withFixes(gh, fixes, ev).checkThreadFixes()
		if gh.calls != 1 || len(fixes.list) != 1 || len(ev.names) != 0 {
			t.Errorf("gh calls = %d, waiting = %v, events = %v; want one read, still waiting, no event", gh.calls, fixes.list, ev.names)
		}
	})
	t.Run("when the resolve fails, with no reply sent", func(t *testing.T) {
		gh := &fakeGH{outs: [][]byte{[]byte(`{"headRefOid":"def","state":"OPEN"}`)}, errs: []error{nil, errors.New("HTTP 502")}}
		fixes := &memFixes{list: []api.ThreadFix{waiting}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes()
		if gh.calls != 2 || len(fixes.list) != 1 {
			t.Errorf("gh calls = %d, waiting = %v; want the read and the resolve, still waiting", gh.calls, fixes.list)
		}
	})
	t.Run("when the read fails", func(t *testing.T) {
		gh := &fakeGH{err: errors.New("offline")}
		fixes := &memFixes{list: []api.ThreadFix{waiting, {ThreadID: "PRRT_2", Path: dir, Number: 42, CommentID: 12, Head: "abc"}}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes()
		if gh.calls != 1 || len(fixes.list) != 2 {
			t.Errorf("gh calls = %d, waiting = %v; want one read for the pull request, both still waiting", gh.calls, fixes.list)
		}
	})
}

func TestCheckThreadFixesDropsWhatCanNoLongerBeFixed(t *testing.T) {
	t.Parallel()
	t.Run("a merged pull request", func(t *testing.T) {
		gh := &fakeGH{out: []byte(`{"headRefOid":"abc","state":"MERGED"}`)}
		fixes := &memFixes{list: []api.ThreadFix{{ThreadID: "PRRT_1", Path: t.TempDir(), Number: 42, CommentID: 11, Head: "abc"}}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes()
		if gh.calls != 1 || len(fixes.list) != 0 {
			t.Errorf("gh calls = %d, waiting = %v; want the read only and nothing waiting", gh.calls, fixes.list)
		}
	})
	t.Run("a checkout that is gone", func(t *testing.T) {
		gh := &fakeGH{}
		fixes := &memFixes{list: []api.ThreadFix{{ThreadID: "PRRT_1", Path: t.TempDir() + "/gone", Number: 42, CommentID: 11, Head: "abc"}}}
		withFixes(gh, fixes, &emitted{}).checkThreadFixes()
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
}
