package extensions

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

// moreLich records the second batch's calls. Its services are web, running,
// and db, stopped, in project p1; s3 is the one live session.
type moreLich struct {
	asks     []api.AskSessionOptions
	waits    []api.WaitOptions
	acts     []string
	reads    []string
	notes    []api.NotifyOptions
	shown    []api.ShowCodeOptions
	opened   []api.BrowserOpenOptions
	waitFn   func(api.WaitOptions) api.SendResult
	detached bool
}

func (f *fakeLich) wire(l *Lich) {
	m := &f.more
	exit := 0
	services := []api.Service{
		{ServiceConfig: api.ServiceConfig{ID: "w", ProjectID: "p1", Name: "web", Command: "npm run dev"}, State: api.ServiceRunning, StartedAt: 5},
		{ServiceConfig: api.ServiceConfig{ID: "d", ProjectID: "p1", Name: "db", Command: "sleep 30"}, State: api.ServiceStopped, ExitCode: &exit},
		{ServiceConfig: api.ServiceConfig{ID: "o", ProjectID: "p2", Name: "other"}, State: api.ServiceStopped},
	}
	act := func(verb string) serviceAct {
		return func(_ context.Context, opts api.ServiceIDOptions) (api.None, error) {
			f.record(func() { m.acts = append(m.acts, verb+" "+opts.ID) })
			return api.None{}, nil
		}
	}
	output := func(kind string) func(context.Context, api.ServiceIDOptions) ([]byte, error) {
		return func(_ context.Context, opts api.ServiceIDOptions) ([]byte, error) {
			f.record(func() { m.reads = append(m.reads, kind+" "+opts.ID) })
			return []byte(kind + " output"), nil
		}
	}
	l.Ask = func(_ context.Context, opts api.AskSessionOptions) (api.SessionAnswer, error) {
		f.record(func() { m.asks = append(m.asks, opts) })
		return api.SessionAnswer{ID: "s3", Project: "lich", Label: "api-3", Answer: "it is green"}, nil
	}
	l.Wait = func(_ context.Context, opts api.WaitOptions) (api.SendResult, error) {
		f.record(func() { m.waits = append(m.waits, opts) })
		if m.waitFn != nil {
			return m.waitFn(opts), nil
		}
		return api.SendResult{Ticket: opts.Ticket, Target: "api-3", Status: "answered", Answer: "late"}, nil
	}
	l.Project = func(_ context.Context, opts api.ProjectRefOptions) (api.ProjectRef, error) {
		if opts.Project != "lich" || opts.From != "" {
			return api.ProjectRef{}, errors.New("no project " + opts.Project)
		}
		return api.ProjectRef{ID: "p1", Name: "lich", Path: "/src/lich"}, nil
	}
	l.Services = func(context.Context, api.None) ([]api.Service, error) { return services, nil }
	l.StartService, l.StopService, l.RestartService = act("start"), act("stop"), act("restart")
	l.Output, l.LiveOutput = output("last"), output("live")
	l.Diff = func(_ context.Context, opts api.PathOptions) (api.DiffStats, error) {
		return api.DiffStats{Files: 2, Added: 10, Deleted: 3, Head: "abc", Branch: "feat"}, nil
	}
	l.BaseStatus = func(context.Context, api.PathOptions) (*api.BaseStatus, error) {
		return &api.BaseStatus{Base: "origin/main", Behind: 4}, nil
	}
	l.Upstream = func(context.Context, api.PathOptions) (api.BaseFreshness, error) {
		return api.BaseFreshness{Upstream: "origin/feat", Ahead: 2, Behind: 1}, nil
	}
	l.PullRequest = func(_ context.Context, opts api.PathOptions) (*api.PullRequest, error) {
		if opts.Path != "/wt/feat" {
			return nil, nil
		}
		return &api.PullRequest{Number: 9, URL: "u", Title: "Add it", State: "OPEN", HeadRefName: "feat", BaseRefName: "main"}, nil
	}
	l.Notify = func(_ context.Context, opts api.NotifyOptions) (api.None, error) {
		f.record(func() { m.notes = append(m.notes, opts) })
		return api.None{}, nil
	}
	l.ShowCode = func(_ context.Context, opts api.ShowCodeOptions) (api.ShowCode, error) {
		f.record(func() { m.shown = append(m.shown, opts) })
		return api.ShowCode{SessionID: opts.ID, Rel: "main.go", Line: opts.Line, EndLine: opts.EndLine, Comment: opts.Comment}, nil
	}
	l.OpenURL = func(_ context.Context, opts api.BrowserOpenOptions) (api.BrowserHandle, error) {
		f.record(func() { m.opened = append(m.opened, opts) })
		return api.BrowserHandle{URL: opts.URL, Title: "Example", Owner: opts.ID}, nil
	}
	l.Attached = func() bool { return !m.detached }
}

func TestEachNewMethodIsRefusedWithoutItsPermission(t *testing.T) {
	calls := map[extension.Permission]extension.Message{
		extension.PermissionSessionsAsk:   extension.AskSession.Call("1", extension.AskParams{Session: "api-3", Question: "q"}),
		extension.PermissionServicesRead:  extension.ListServices.Call("1", extension.ServicesParams{Project: "lich"}),
		extension.PermissionServicesWrite: extension.StartService.Call("1", extension.ServiceParams{Project: "lich", Service: "db"}),
		extension.PermissionGitRead:       extension.GitStatus.Call("1", extension.GitParams{Path: "/src/lich"}),
		extension.PermissionNotify:        extension.Notify.Call("1", extension.NotifyParams{Title: "hi"}),
		extension.PermissionWindow:        extension.OpenURL.Call("1", extension.OpenURLParams{Session: "s3", URL: "https://example.com"}),
	}
	for p, msg := range calls {
		t.Run(string(p), func(t *testing.T) {
			f := &fakeLich{}
			s, x := apiService(t, f, allBut(p)...)
			res := result(t, s, x, msg)
			if !strings.Contains(res.Error, `needs the permission "`+string(p)+`"`) {
				t.Errorf("got %+v, want a refusal naming %s", res, p)
			}
			m := f.more
			if len(m.asks)+len(m.acts)+len(m.notes)+len(m.opened) > 0 {
				t.Error("a refused call reached a service")
			}
		})
	}
	others := map[string]extension.Permission{
		extension.CollectAnswer.Name: extension.PermissionSessionsWrite, extension.StopService.Name: extension.PermissionServicesWrite,
		extension.RestartService.Name: extension.PermissionServicesWrite, extension.ServiceOutput.Name: extension.PermissionServicesRead,
		extension.GitPullRequest.Name: extension.PermissionGitRead, extension.ShowCode.Name: extension.PermissionWindow,
	}
	s, _ := apiService(t, &fakeLich{})
	for name, p := range others {
		if got := s.calls[name].permission; got != p {
			t.Errorf("%s needs %q, want %q", name, got, p)
		}
	}
}

func allBut(p extension.Permission) []extension.Permission {
	var out []extension.Permission
	for _, q := range allPermissions {
		if q != p {
			out = append(out, q)
		}
	}
	return out
}

func TestAskSpeaksAsNoSession(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, extension.PermissionSessionsAsk)
	got, err := call(t, s, x, extension.AskSession, extension.AskParams{Session: "api-3", Project: "lich", Question: "green?"})
	if err != nil || got != (extension.AskResult{ID: "s3", Project: "lich", Label: "api-3", Answer: "it is green"}) {
		t.Errorf("got %+v, %v", got, err)
	}
	if f.more.asks[0] != (api.AskSessionOptions{Target: "api-3", Project: "lich", Question: "green?"}) {
		t.Errorf("asked with %+v", f.more.asks[0])
	}
}

func TestAnExtensionCollectsOnlyItsOwnPendingTickets(t *testing.T) {
	f := &fakeLich{sendFn: func(opts api.SendOptions) (api.SendResult, error) {
		return api.SendResult{Ticket: "t-" + opts.Prompt, Target: opts.Target, Status: "pending"}, nil
	}}
	s, x := apiService(t, f, extension.PermissionSessionsWrite)
	other := &installed{id: "other", manifest: x.manifest, inFlight: make(chan struct{}, maxCallsInFlight), queue: make(chan []byte, sendQueue)}
	s.all = append(s.all, other)
	if _, err := call(t, s, x, extension.SendToSession, extension.SendParams{Session: "api-3", Prompt: "mine"}); err != nil {
		t.Fatal(err)
	}
	for who, ticket := range map[*installed]string{other: "t-mine", x: "t-guessed"} {
		if _, err := call(t, s, who, extension.CollectAnswer, extension.AnswerParams{Ticket: ticket}); err == nil || !strings.Contains(err.Error(), "is not one this extension") {
			t.Errorf("%s collected %s: %v", who.id, ticket, err)
		}
	}
	if len(f.more.waits) != 0 {
		t.Fatalf("a refused collect reached the relay: %+v", f.more.waits)
	}
	got, err := call(t, s, x, extension.CollectAnswer, extension.AnswerParams{Ticket: "t-mine", WaitSeconds: 500})
	if err != nil || got.Answer != "late" || f.more.waits[0] != (api.WaitOptions{Ticket: "t-mine", WaitSeconds: maxSendWaitSeconds}) {
		t.Errorf("got %+v, %v after %+v", got, err, f.more.waits)
	}
	if _, err := call(t, s, x, extension.CollectAnswer, extension.AnswerParams{Ticket: "t-mine"}); err == nil {
		t.Error("a ticket with its answer collected could be collected again")
	}
}

func TestAPendingCollectKeepsItsTicket(t *testing.T) {
	f := &fakeLich{sendFn: func(opts api.SendOptions) (api.SendResult, error) {
		return api.SendResult{Ticket: "t1", Target: opts.Target, Status: "pending"}, nil
	}}
	f.more.waitFn = func(opts api.WaitOptions) api.SendResult {
		return api.SendResult{Ticket: opts.Ticket, Status: "pending"}
	}
	s, x := apiService(t, f, extension.PermissionSessionsWrite)
	opened, err := call(t, s, x, extension.OpenSession, extension.OpenSessionParams{Project: "lich", Prompt: "go"})
	if err != nil || opened.Task == nil || opened.Task.Status != "pending" {
		t.Fatalf("opened %+v, %v", opened, err)
	}
	for range 2 {
		if got, err := call(t, s, x, extension.CollectAnswer, extension.AnswerParams{Ticket: "t1", WaitSeconds: 3}); err != nil || got.Status != "pending" {
			t.Errorf("got %+v, %v", got, err)
		}
	}
	if f.more.waits[0].WaitSeconds != 3 {
		t.Errorf("waited with %+v", f.more.waits[0])
	}
}

func TestServicesMirrorTheServiceTools(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, extension.PermissionServicesRead, extension.PermissionServicesWrite)
	list, err := call(t, s, x, extension.ListServices, extension.ServicesParams{Project: "lich"})
	if err != nil || len(list) != 2 || list[0].Name != "web" || list[0].StartedAt != 5 || *list[1].ExitCode != 0 {
		t.Errorf("listed %+v, %v", list, err)
	}
	for m, verb := range map[extension.Method[extension.ServiceParams, extension.Service]]string{
		extension.StartService: "start", extension.StopService: "stop", extension.RestartService: "restart",
	} {
		f.more.acts = nil
		got, err := call(t, s, x, m, extension.ServiceParams{Project: "lich", Service: "DB"})
		if err != nil || got.ID != "d" || len(f.more.acts) != 1 || f.more.acts[0] != verb+" d" {
			t.Errorf("%s: %+v, %v after %v", m.Name, got, err, f.more.acts)
		}
	}
	_, err = call(t, s, x, extension.StartService, extension.ServiceParams{Project: "lich", Service: "other"})
	if err == nil || !strings.Contains(err.Error(), `no service named "other" in lich. Its services: "web", "db"`) {
		t.Errorf("another project's service: %v", err)
	}
	if _, err := call(t, s, x, extension.ListServices, extension.ServicesParams{Project: "nope"}); err == nil {
		t.Error("an unknown project listed")
	}
}

func TestServiceOutputReadsLiveOrLastAsReadServiceOutputDoes(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, extension.PermissionServicesRead)
	for name, want := range map[string]string{"web": "live output", "db": "last output"} {
		got, err := call(t, s, x, extension.ServiceOutput, extension.ServiceParams{Project: "lich", Service: name})
		if err != nil || got.Output != want {
			t.Errorf("%s: %+v, %v; want %q", name, got, err, want)
		}
	}
}

func TestServiceStatusFiresWhenAStateMoves(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, extension.PermissionServicesRead)
	if _, err := call(t, s, x, extension.Subscribe, extension.SubscribeParams{Names: []string{extension.ServiceStatus.Name}}); err != nil {
		t.Fatal(err)
	}
	code := 2
	svc := func(id, state string, exit *int) api.Service {
		return api.Service{ServiceConfig: api.ServiceConfig{ID: id, ProjectID: "p1", Name: id}, State: state, ExitCode: exit}
	}
	s.Observe(api.ServicesEvent.Name, []api.Service{svc("new", api.ServiceStopped, nil), svc("up", api.ServiceRunning, nil)})
	s.Observe(api.ServicesEvent.Name, []api.Service{svc("new", api.ServiceStopped, nil), svc("up", api.ServiceRunning, nil)})
	s.Observe(api.ServicesEvent.Name, []api.Service{svc("new", api.ServiceFailed, &code), svc("up", api.ServiceRunning, nil)})
	want := []extension.ServiceStatusData{
		{ID: "up", Name: "up", ProjectID: "p1", State: api.ServiceRunning},
		{ID: "new", Name: "new", ProjectID: "p1", State: api.ServiceFailed, ExitCode: &code},
	}
	for _, w := range want {
		got, err := extension.ServiceStatus.Decode(next(t, x).Data.(extension.LichEventData))
		if err != nil || got.ID != w.ID || got.State != w.State || (w.ExitCode != nil && *got.ExitCode != *w.ExitCode) {
			t.Errorf("event %+v, %v; want %+v", got, err, w)
		}
	}
	if len(x.queue) != 0 {
		t.Error("a list with nothing moved fired an event")
	}
}

func TestGitReadsTheCardsAnswers(t *testing.T) {
	s, x := apiService(t, &fakeLich{}, extension.PermissionGitRead)
	status, err := call(t, s, x, extension.GitStatus, extension.GitParams{Path: "/wt/feat"})
	want := extension.GitStatusResult{Branch: "feat", Head: "abc", Files: 2, Added: 10, Deleted: 3,
		Upstream: "origin/feat", Ahead: 2, Behind: 1, Base: "origin/main", BehindBase: 4, Conflicts: []string{}}
	if err != nil || !reflect.DeepEqual(status, want) {
		t.Errorf("status %+v, %v; want %+v with an empty conflict list", status, err, want)
	}
	pr, err := call(t, s, x, extension.GitPullRequest, extension.GitParams{Path: "/wt/feat"})
	if err != nil || pr.PullRequest == nil || *pr.PullRequest != (extension.PullRequest{Number: 9, URL: "u", Title: "Add it", Head: "feat", Base: "main"}) {
		t.Errorf("pull request %+v, %v", pr.PullRequest, err)
	}
	none, err := call(t, s, x, extension.GitPullRequest, extension.GitParams{Path: "/src/lich"})
	if err != nil || none.PullRequest != nil {
		t.Errorf("no pull request read as %+v, %v", none.PullRequest, err)
	}
}

func TestNotifyIsBoundedPerExtension(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, extension.PermissionNotify)
	for range maxNotifications {
		if _, err := call(t, s, x, extension.Notify, extension.NotifyParams{Title: "done", Body: "all green"}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := call(t, s, x, extension.Notify, extension.NotifyParams{Title: "again"})
	if err == nil || !strings.Contains(err.Error(), "the most one extension may raise") {
		t.Errorf("the notification past the bound: %v", err)
	}
	if len(f.more.notes) != maxNotifications || f.more.notes[0] != (api.NotifyOptions{Summary: "done", Detail: "all green"}) {
		t.Errorf("raised %+v", f.more.notes)
	}
	later := time.Now().Add(notifyWindow)
	if err := s.takeNotification(x, later); err != nil {
		t.Errorf("a notification once the window passed: %v", err)
	}
}

func TestWindowMethodsNeedAnAttachedWindow(t *testing.T) {
	f := &fakeLich{}
	f.more.detached = true
	s, x := apiService(t, f, extension.PermissionWindow)
	_, err := call(t, s, x, extension.ShowCode, extension.ShowCodeParams{Session: "s3", Path: "main.go"})
	if err == nil || !strings.Contains(err.Error(), errNoWindow.Error()) {
		t.Errorf("show code with no window: %v", err)
	}
	_, err = call(t, s, x, extension.OpenURL, extension.OpenURLParams{Session: "s3", URL: "https://example.com"})
	if err == nil || !strings.Contains(err.Error(), errNoWindow.Error()) {
		t.Errorf("open url with no window: %v", err)
	}
	if len(f.more.shown)+len(f.more.opened) != 0 {
		t.Error("a window method acted with no window")
	}
}

func TestWindowMethodsMirrorShowCodeAndBrowserOpen(t *testing.T) {
	f := &fakeLich{}
	s, x := apiService(t, f, extension.PermissionWindow)
	shown, err := call(t, s, x, extension.ShowCode, extension.ShowCodeParams{Session: "s3", Path: "/src/lich/main.go", Line: 3, EndLine: 5, Comment: "here"})
	if err != nil || shown != (extension.ShownCode{Session: "s3", Path: "main.go", Line: 3, EndLine: 5}) {
		t.Errorf("shown %+v, %v", shown, err)
	}
	if f.more.shown[0] != (api.ShowCodeOptions{ID: "s3", Path: "/src/lich/main.go", Line: 3, EndLine: 5, Comment: "here"}) {
		t.Errorf("show code reached with %+v", f.more.shown[0])
	}
	page, err := call(t, s, x, extension.OpenURL, extension.OpenURLParams{Session: "s3", URL: "https://example.com"})
	if err != nil || page != (extension.OpenedURL{URL: "https://example.com", Title: "Example"}) {
		t.Errorf("opened %+v, %v", page, err)
	}
	for _, bad := range []extension.OpenURLParams{
		{Session: "s3", URL: "file:///etc/passwd"}, {Session: "s3", URL: "notes/report.html"}, {Session: "gone", URL: "https://example.com"},
	} {
		if _, err := call(t, s, x, extension.OpenURL, bad); err == nil {
			t.Errorf("opened %+v", bad)
		}
	}
	if len(f.more.opened) != 1 {
		t.Errorf("the browser was reached %d times, want once", len(f.more.opened))
	}
}
