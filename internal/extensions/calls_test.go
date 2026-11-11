package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

var allPermissions = extension.Permissions

// fakeLich records what each call reached the services with. Its zero value
// answers every call with nothing.
type fakeLich struct {
	mu     sync.Mutex
	peers  []api.PeersOptions
	trees  []api.WorktreesOptions
	opens  []api.OpenSessionOptions
	sends  []api.SendOptions
	sendFn func(api.SendOptions) (api.SendResult, error)
	block  chan struct{}
}

func (f *fakeLich) lich() Lich {
	return Lich{
		Projects: func(context.Context, api.None) ([]api.Project, error) {
			return []api.Project{{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []api.Session{{ID: "s1"}}}}, nil
		},
		Peers: func(ctx context.Context, opts api.PeersOptions) ([]api.Peer, error) {
			f.record(func() { f.peers = append(f.peers, opts) })
			if f.block != nil {
				select {
				case <-f.block:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return []api.Peer{{Label: "api-3", Project: "lich", Kind: "codex", State: "busy", ID: "s3", Tool: "Bash", Focused: true}}, nil
		},
		Worktrees: func(_ context.Context, opts api.WorktreesOptions) ([]api.WorktreeCheckout, error) {
			f.record(func() { f.trees = append(f.trees, opts) })
			if opts.Project == "" {
				return nil, errors.New("no project given, and this is not running in a lich session — name one")
			}
			return []api.WorktreeCheckout{{Name: "feat", Path: "/wt/feat", Dirty: true, Sessions: []string{"feat"}}}, nil
		},
		Open: func(_ context.Context, opts api.OpenSessionOptions) (api.OpenedSession, error) {
			f.record(func() { f.opens = append(f.opens, opts) })
			return api.OpenedSession{ID: "s9", ProjectID: "p1", Project: "lich", Label: "feat", Kind: "claude", Path: "/wt/feat"}, nil
		},
		Send: func(_ context.Context, opts api.SendOptions) (api.SendResult, error) {
			f.record(func() { f.sends = append(f.sends, opts) })
			if f.sendFn != nil {
				return f.sendFn(opts)
			}
			return api.SendResult{Ticket: "t1", Target: opts.Target, Status: "answered", Answer: "done"}, nil
		},
	}
}

func (f *fakeLich) record(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

// apiService is a Service holding one running extension, x, whose stdin is
// x.queue: a call is answered there.
func apiService(t *testing.T, f *fakeLich, permissions ...extension.Permission) (*Service, *installed) {
	t.Helper()
	s := New(t.TempDir(), t.TempDir(), "1.2.3", nil, func(string, any) {})
	t.Cleanup(s.Shutdown)
	s.SetLich(f.lich())
	x := &installed{
		id: "ext", manifest: extension.Manifest{Permissions: permissions},
		inFlight: make(chan struct{}, maxCallsInFlight), queue: make(chan []byte, sendQueue),
	}
	s.all = []*installed{x}
	return s, x
}

func callLine(t *testing.T, msg extension.Message) []byte {
	t.Helper()
	line, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return line
}

func next(t *testing.T, x *installed) extension.Message {
	t.Helper()
	select {
	case line := <-x.queue:
		var msg extension.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			t.Fatalf("lich wrote a line that does not decode: %v: %s", err, line)
		}
		return msg
	case <-time.After(10 * time.Second):
		t.Fatal("no answer")
		return extension.Message{}
	}
}

func result(t *testing.T, s *Service, x *installed, msg extension.Message) extension.ResultData {
	t.Helper()
	s.handleLine(x, callLine(t, msg))
	res, ok := next(t, x).Data.(extension.ResultData)
	if !ok {
		t.Fatal("the answer is not a result")
	}
	return res
}

func call[P, R any](t *testing.T, s *Service, x *installed, m extension.Method[P, R], params P) (R, error) {
	t.Helper()
	res := result(t, s, x, m.Call("c1", params))
	if res.ID != "c1" {
		t.Errorf("result under id %q, want c1", res.ID)
	}
	return m.Result(res)
}

func TestEveryDeclaredMethodIsRouted(t *testing.T) {
	s, _ := apiService(t, &fakeLich{})
	for _, name := range extension.MethodNames() {
		if _, ok := s.calls[name]; !ok {
			t.Errorf("method %s has no route", name)
		}
	}
}

func TestAnUnknownMethodIsAnErrorResult(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, allPermissions...)
	res := result(t, s, x, extension.CallMethod.New(extension.CallData{ID: "7", Method: "sessions.delete"}))
	if res.ID != "7" || !strings.Contains(res.Error, `no method "sessions.delete"`) {
		t.Errorf("got %+v", res)
	}
}

func TestAMethodTheManifestDidNotAskForIsRefused(t *testing.T) {
	for name, msg := range map[extension.Permission]extension.Message{
		extension.PermissionProjectsRead:  extension.ListWorktrees.Call("1", extension.WorktreesParams{Project: "lich"}),
		extension.PermissionSessionsRead:  extension.ListSessions.Call("1", extension.NoParams{}),
		extension.PermissionSessionsWrite: extension.SendToSession.Call("1", extension.SendParams{Session: "a", Prompt: "b"}),
	} {
		t.Run(string(name), func(t *testing.T) {
			f := &fakeLich{}
			var others []extension.Permission
			for _, p := range allPermissions {
				if p != name {
					others = append(others, p)
				}
			}
			s, x := apiService(t, f, others...)
			res := result(t, s, x, msg)
			if !strings.Contains(res.Error, `needs the permission "`+string(name)+`"`) {
				t.Errorf("got %+v, want a refusal naming %s", res, name)
			}
			if len(f.sends)+len(f.peers)+len(f.trees) > 0 {
				t.Error("a refused call reached a service")
			}
		})
	}
}

func TestParamsThatDoNotDecodeAreAnErrorResult(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, allPermissions...)
	res := result(t, s, x, extension.CallMethod.New(extension.CallData{
		ID: "1", Method: extension.SendToSession.Name, Params: json.RawMessage(`{"session":7}`),
	}))
	if !strings.Contains(res.Error, "sessions.send: params") {
		t.Errorf("got %+v", res)
	}
}

func TestListsMirrorTheirServices(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, allPermissions...)

	projects, err := call(t, s, x, extension.ListProjects, extension.NoParams{})
	if err != nil || len(projects) != 1 || projects[0] != (extension.Project{ID: "p1", Name: "lich", Path: "/src/lich"}) {
		t.Errorf("projects %+v, %v", projects, err)
	}
	sessions, err := call(t, s, x, extension.ListSessions, extension.NoParams{})
	want := extension.Session{Label: "api-3", Project: "lich", Kind: "codex", State: "busy", ID: "s3", Tool: "Bash", Focused: true}
	if err != nil || len(sessions) != 1 || sessions[0] != want {
		t.Errorf("sessions %+v, %v", sessions, err)
	}
	trees, err := call(t, s, x, extension.ListWorktrees, extension.WorktreesParams{Project: "lich"})
	if err != nil || len(trees) != 1 || trees[0].Name != "feat" || !trees[0].Dirty || trees[0].Sessions[0] != "feat" {
		t.Errorf("worktrees %+v, %v", trees, err)
	}
	if f.peers[0].From != "" || f.trees[0] != (api.WorktreesOptions{Project: "lich"}) {
		t.Errorf("the calls spoke as a session: %+v %+v", f.peers, f.trees)
	}
}

func TestAServiceErrorIsAnErrorResult(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, allPermissions...)
	_, err := call(t, s, x, extension.ListWorktrees, extension.WorktreesParams{})
	if err == nil || !strings.Contains(err.Error(), "name one") {
		t.Errorf("got %v, want the service's own words", err)
	}
}

func TestOpenSessionHandsItsPromptOverAsOpenSessionDoes(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, allPermissions...)
	opened, err := call(t, s, x, extension.OpenSession, extension.OpenSessionParams{
		Project: "lich", Worktree: "feat", Provider: "codex", Prompt: "fix it",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.opens[0] != (api.OpenSessionOptions{Project: "lich", Kind: "codex", Worktree: "feat"}) {
		t.Errorf("opened with %+v", f.opens[0])
	}
	wantSend := api.SendOptions{Target: "feat", Project: "lich", Prompt: "fix it", WaitSeconds: handOverWaitSeconds}
	if f.sends[0] != wantSend {
		t.Errorf("handed over with %+v, want %+v", f.sends[0], wantSend)
	}
	if opened.Label != "feat" || opened.Task == nil || opened.Task.Answer != "done" {
		t.Errorf("opened %+v", opened)
	}
}

func TestOpenSessionReportsAFailedHandOverBesideTheSession(t *testing.T) {
	f := &fakeLich{sendFn: func(api.SendOptions) (api.SendResult, error) { return api.SendResult{}, errors.New("pty gone") }}
	s, x := apiService(t, f, allPermissions...)
	opened, err := call(t, s, x, extension.OpenSession, extension.OpenSessionParams{Project: "lich", Prompt: "go"})
	if err != nil || opened.ID != "s9" || opened.Task != nil || opened.TaskError != "pty gone" {
		t.Errorf("opened %+v, %v", opened, err)
	}
	f2 := &fakeLich{}
	s2, x2 := apiService(t, f2, allPermissions...)
	if _, err := call(t, s2, x2, extension.OpenSession, extension.OpenSessionParams{Project: "lich"}); err != nil || len(f2.sends) != 0 {
		t.Errorf("an open with no prompt sent %+v, %v", f2.sends, err)
	}
}

func TestSendClampsItsWaitAsSendToSessionDoes(t *testing.T) {
	for asked, want := range map[int]int{0: maxSendWaitSeconds, -3: maxSendWaitSeconds, 5: 5, 500: maxSendWaitSeconds} {
		f := &fakeLich{}
		s, x := apiService(t, f, extension.PermissionSessionsWrite)
		got, err := call(t, s, x, extension.SendToSession, extension.SendParams{Session: "api-3", Project: "lich", Prompt: "hi", WaitSeconds: asked})
		if err != nil || got.Status != "answered" || got.Target != "api-3" {
			t.Errorf("got %+v, %v", got, err)
		}
		if f.sends[0] != (api.SendOptions{Target: "api-3", Project: "lich", Prompt: "hi", WaitSeconds: want}) {
			t.Errorf("asked %d: sent %+v", asked, f.sends[0])
		}
	}
}

func TestCallsPastTheBoundAreRefused(t *testing.T) {
	f := &fakeLich{block: make(chan struct{})}
	s, x := apiService(t, f, allPermissions...)
	for range maxCallsInFlight {
		s.handleLine(x, callLine(t, extension.ListSessions.Call("held", extension.NoParams{})))
	}
	waitUntil(t, "every held call to reach the service", func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.peers) == maxCallsInFlight
	})
	res := result(t, s, x, extension.ListSessions.Call("over", extension.NoParams{}))
	if res.ID != "over" || !strings.Contains(res.Error, "already running") {
		t.Fatalf("the call past the bound got %+v", res)
	}
	close(f.block)
	for range maxCallsInFlight {
		if res := next(t, x).Data.(extension.ResultData); res.Error != "" {
			t.Errorf("a held call failed: %s", res.Error)
		}
	}
	if _, err := call(t, s, x, extension.ListSessions, extension.NoParams{}); err != nil {
		t.Errorf("a call after the others finished: %v", err)
	}
}

func TestACallPastItsTimeoutIsAnErrorResult(t *testing.T) {
	f := &fakeLich{block: make(chan struct{})}
	s, x := apiService(t, f, allPermissions...)
	s.timing.callTimeout = 20 * time.Millisecond
	_, err := call(t, s, x, extension.ListSessions, extension.NoParams{})
	if err == nil || !strings.Contains(err.Error(), "ran past its 20ms") {
		t.Errorf("got %v", err)
	}
	if len(x.inFlight) != 0 {
		t.Error("the timed-out call kept its slot")
	}
}

func TestSubscribeChecksEveryNameFirst(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, extension.PermissionSessionsRead)
	_, err := call(t, s, x, extension.Subscribe, extension.SubscribeParams{Names: []string{extension.SessionStatus.Name}})
	if err == nil || !strings.Contains(err.Error(), `needs the permission "events:session"`) {
		t.Errorf("got %v", err)
	}
	_, err = call(t, s, x, extension.Subscribe, extension.SubscribeParams{Names: []string{"session.deleted"}})
	if err == nil || !strings.Contains(err.Error(), `no event "session.deleted"`) {
		t.Errorf("got %v", err)
	}
	s.Observe(api.SessionStatusEvent.Name, api.SessionStatus{ID: "s1", State: "busy"})
	if len(x.queue) != 0 {
		t.Error("a refused subscription still delivers")
	}
}

func TestASubscribedEventReachesOnlyItsSubscribers(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, extension.PermissionSessionEvents)
	quiet := &installed{id: "quiet", inFlight: make(chan struct{}, 1), queue: make(chan []byte, sendQueue)}
	s.all = append(s.all, quiet)
	if _, err := call(t, s, x, extension.Subscribe, extension.SubscribeParams{Names: []string{extension.SessionStatus.Name}}); err != nil {
		t.Fatal(err)
	}
	s.Observe(api.SessionTitleEvent.Name, api.SessionTitle{ID: "s1", Label: "x"})
	s.Observe(api.SessionStatusEvent.Name, api.SessionStatus{ID: "s1", State: "waiting", Reason: "permission", Program: true})
	ev, ok := next(t, x).Data.(extension.LichEventData)
	if !ok {
		t.Fatal("the subscriber got something other than an event")
	}
	got, err := extension.SessionStatus.Decode(ev)
	if err != nil || got != (extension.SessionStatusData{ID: "s1", State: "waiting", Reason: "permission"}) {
		t.Errorf("event %+v, %v", got, err)
	}
	if len(x.queue) != 0 || len(quiet.queue) != 0 {
		t.Error("an event reached an extension that did not subscribe to it")
	}
}

func TestAManifestNamingAnUnknownPermissionIsInvalid(t *testing.T) {
	m := validManifest("ext", "ext")
	m.Permissions = []extension.Permission{"sessions:delete"}
	if err := validate(m, "ext", "1.2.3"); err == nil || !strings.Contains(err.Error(), `"sessions:delete"`) {
		t.Errorf("got %v", err)
	}
}

// apiExtension is the fake extension written with the SDK: it lists the
// sessions, subscribes and reports each status, all as toasts.
func apiExtension() {
	conn := extension.NewConn(os.Stdin, os.Stdout)
	ctx := context.Background()
	sessions, err := conn.Sessions(ctx)
	if err != nil {
		_ = conn.Send(extension.ShowToast.New(extension.ToastData{Title: "error " + err.Error()}))
	} else {
		_ = conn.Send(extension.ShowToast.New(extension.ToastData{Title: "session " + sessions[0].Label}))
	}
	if err := conn.Subscribe(ctx, extension.SessionStatus.Name); err != nil {
		_ = conn.Send(extension.ShowToast.New(extension.ToastData{Title: "error " + err.Error()}))
	}
	_ = conn.Send(extension.ShowToast.New(extension.ToastData{Title: "subscribed"}))
	for msg := range conn.Messages() {
		if ev, ok := msg.Data.(extension.LichEventData); ok {
			status, _ := extension.SessionStatus.Decode(ev)
			_ = conn.Send(extension.ShowToast.New(extension.ToastData{Title: "status " + status.State}))
		}
	}
}

func TestTheSDKTalksToTheBackend(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeAPI, func(m *extension.Manifest) {
		m.Permissions = []extension.Permission{extension.PermissionSessionsRead, extension.PermissionSessionEvents}
	})
	r := &recorder{}
	s := New(dir, filepath.Join(t.TempDir(), "data"), "1.2.3", append(os.Environ(), fakeEnv+"=1"), r.emit)
	s.timing.stopGrace, s.timing.killWait = 5*time.Second, 10*time.Second
	s.SetLich((&fakeLich{}).lich())
	t.Cleanup(s.Shutdown)
	s.Boot()

	waitUntil(t, "the subscription", func() bool { return strings.Contains(strings.Join(r.titles(t), "|"), "subscribed") })
	s.Observe(api.SessionStatusEvent.Name, api.SessionStatus{ID: "s3", State: "done"})
	want := "session api-3|subscribed|status done"
	waitUntil(t, want, func() bool { return strings.Join(r.titles(t), "|") == want })
	if got := r.lastStatus().Permissions; len(got) != 2 || got[0] != "sessions:read" {
		t.Errorf("the extension reports permissions %v", got)
	}
}
