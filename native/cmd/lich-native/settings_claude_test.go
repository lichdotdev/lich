package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// profileBackend answers the claudeprofile calls a dialog makes and records
// their order, so a test can say which came after which.
type profileBackend struct {
	mu        sync.Mutex
	calls     []string
	loggedIn  bool
	deleteErr string
}

func (b *profileBackend) record(call string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, call)
}

func (b *profileBackend) log() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.calls)
}

func (b *profileBackend) client(t *testing.T) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		method := strings.TrimPrefix(r.URL.Path, "/rpc/")
		var out any
		switch method {
		case "claudeprofile.CreateProfile":
			out = lichclient.ClaudeProfile{Slug: "work", Name: "Work"}
		case "claudeprofile.Plugins":
			out = []lichclient.ClaudeProfilePlugin{{Key: "a@m", Name: "a", Marketplace: "m"}, {Key: "b@m", Name: "b", Marketplace: "m"}}
		case "claudeprofile.Profiles":
			b.mu.Lock()
			in := b.loggedIn
			b.mu.Unlock()
			out = []lichclient.ClaudeProfile{{Slug: personalProfile, Name: "Personal", Personal: true, LoggedIn: true},
				{Slug: "work", Name: "Work", LoggedIn: in, Email: "w@x.y", Plan: "max"}}
		case "claudeprofile.InstallPlugins":
			out = []lichclient.ClaudeProfilePluginResult{{Key: "a@m", OK: true}, {Key: "b@m", Message: "blocked by your organization"}}
		case "claudeprofile.DeleteProfile":
			if b.deleteErr != "" {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": b.deleteErr})
				return
			}
		}
		if strings.HasPrefix(method, "claudeprofile.") && method != "claudeprofile.Plugins" && method != "claudeprofile.Profiles" {
			b.record(method + string(body))
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func flowScreen(t *testing.T, b *profileBackend) *settingsScreen {
	t.Helper()
	c := b.client(t)
	s := newSettingsScreen(t.Context(), c, new(ui.Root), func() {})
	s.providers.claude.book = newProfileBook(c, func() {}, newPlanQuota(quotaReader(c), func() {}))
	return s
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("never happened: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestNewProfileBringsItsPluginsOnlyOnceTheLoginIsThrough(t *testing.T) {
	b := &profileBackend{}
	s := flowScreen(t, b)
	f := &s.providers.claude.flow
	s.startNewProfile()
	eventually(t, "the checklist has Personal's plugins", func() bool { return len(f.view().plugins) == 2 })
	f.name.SetText("Work")
	s.confirmFlow(f.view())
	eventually(t, "the login is waiting", func() bool { return f.view().step == stepWaiting })

	s.loginProgress(lichclient.ClaudeProfileLogin{Profile: "work", Output: "visit", URL: "https://claude.com/oauth"})
	if got := f.view().url; got != "https://claude.com/oauth" {
		t.Errorf("address = %q", got)
	}
	if calls := b.log(); slices.ContainsFunc(calls, func(c string) bool { return strings.HasPrefix(c, "claudeprofile.InstallPlugins") }) {
		t.Fatalf("plugins were installed before the login finished: %v", calls)
	}

	b.mu.Lock()
	b.loggedIn = true
	b.mu.Unlock()
	s.loginProgress(lichclient.ClaudeProfileLogin{Profile: "work", Done: true, LoggedIn: true})
	eventually(t, "the result", func() bool { return f.view().step == stepResult })

	d := f.view()
	if d.account != "Logged in as w@x.y · Max" || len(d.results) != 2 || d.failure != "" {
		t.Errorf("result = %+v", d)
	}
	want := []string{`claudeprofile.CreateProfile["Work"]`, `claudeprofile.Login["work"]`, `claudeprofile.InstallPlugins["work",["a@m","b@m"]]`}
	if got := b.log(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
}

func TestCancellingAWaitingLoginEndsItAndClosesTheDialog(t *testing.T) {
	b := &profileBackend{}
	s := flowScreen(t, b)
	f := &s.providers.claude.flow
	s.startLogin(lichclient.ClaudeProfile{Slug: "work", Name: "Work"}, nil)
	eventually(t, "waiting", func() bool { return f.view().step == stepWaiting })
	s.dismissFlow(f.view())
	if f.view().open {
		t.Error("the dialog stayed open")
	}
	eventually(t, "the backend was told", func() bool {
		return slices.Contains(b.log(), `claudeprofile.CancelLogin["work"]`)
	})
	s.loginProgress(lichclient.ClaudeProfileLogin{Profile: "work", Done: true, Cancelled: true, Error: "login cancelled"})
	if f.view().open {
		t.Error("the cancelled event reopened it")
	}
}

func TestALoginThatDidNotTakeShowsItsReason(t *testing.T) {
	b := &profileBackend{}
	s := flowScreen(t, b)
	f := &s.providers.claude.flow
	s.startLogin(lichclient.ClaudeProfile{Slug: "work", Name: "Work"}, []string{"a@m"})
	eventually(t, "waiting", func() bool { return f.view().step == stepWaiting })
	s.loginProgress(lichclient.ClaudeProfileLogin{Profile: "work", Done: true, Error: "claude auth login: exit status 1"})
	d := f.view()
	if d.step != stepResult || d.failure != "claude auth login: exit status 1" {
		t.Errorf("flow = %+v", d)
	}
	title, _ := flowHeading(d)
	if title != "Could not log in to Work" {
		t.Errorf("title = %q", title)
	}
	for _, c := range b.log() {
		if strings.HasPrefix(c, "claudeprofile.InstallPlugins") {
			t.Errorf("plugins installed into a profile nobody is logged in to: %v", b.log())
		}
	}
}

func TestADeleteTheBackendRefusesStaysOnTheForm(t *testing.T) {
	b := &profileBackend{deleteErr: `profile "work" has 1 running session(s); close them before deleting it`}
	s := flowScreen(t, b)
	f := &s.providers.claude.flow
	s.startDelete(lichclient.ClaudeProfile{Slug: "work", Name: "Work"})
	s.confirmFlow(f.view())
	eventually(t, "the refusal", func() bool { return f.view().failure != "" })
	d := f.view()
	if d.step != stepForm || !d.open || !strings.Contains(d.failure, "1 running session") {
		t.Errorf("flow = %+v", d)
	}
}

func TestCopySourceIsPersonalUnlessPersonalIsTheTarget(t *testing.T) {
	list := []lichclient.ClaudeProfile{{Slug: personalProfile}, {Slug: "work"}, {Slug: "side"}}
	if got := defaultCopySource(list, "work"); got != personalProfile {
		t.Errorf("into work: %q", got)
	}
	if got := defaultCopySource(list, personalProfile); got != "work" {
		t.Errorf("into personal: %q", got)
	}
	if got := defaultCopySource(list[:1], personalProfile); got != personalProfile {
		t.Errorf("alone: %q", got)
	}
}

func TestTheProjectsProfileIsTheOneItSetsElseGlobals(t *testing.T) {
	cases := []struct {
		name, global, project string
		set                   bool
		shown, tag            string
	}{
		{"follows a global profile", "work", "", false, "work", ""},
		{"follows Personal", "", "", false, personalProfile, ""},
		{"a profile over Personal", "", "work", true, "work", "work"},
		{"Personal named over a profile", "work", personalProfile, true, personalProfile, personalProfile},
		{"restates Global", "work", "work", true, "work", ""},
		{"Personal restated", "", personalProfile, true, personalProfile, ""},
	}
	for _, c := range cases {
		if got := effectiveScopeProfile(c.global, c.project, c.set); got != c.shown {
			t.Errorf("%s: shown %q, want %q", c.name, got, c.shown)
		}
		if got := differingProfile(c.global, c.project, c.set); got != c.tag {
			t.Errorf("%s: tag %q, want %q", c.name, got, c.tag)
		}
	}
}

func TestTheProfileKeyIsReadPerProjectAndSearchable(t *testing.T) {
	if !slices.Contains(scopedKeys(), claudeProfileKey) {
		t.Error("a project's profile is not read per project")
	}
	if !slices.Contains(providerGlobalKeys(claudeKind), claudeProfileKey) || slices.Contains(providerGlobalKeys("codex"), claudeProfileKey) {
		t.Error("the global profile is Claude Code's alone")
	}
	claude := launchProvider{id: claudeKind, name: "Claude Code", enabled: true}
	codex := launchProvider{id: "codex", name: "Codex", enabled: true}
	hits := searchSettings("profile", claude, codex)
	if len(hits) < 2 || hits[0].providerID != claudeKind {
		t.Fatalf("hits = %+v", hits)
	}
	for _, h := range hits {
		if h.providerID == "codex" {
			t.Errorf("a profile is not Codex's: %+v", h)
		}
	}
}

func TestResumeNamesTheProfileTheConversationLivesIn(t *testing.T) {
	m := newModel(nil, func() {})
	m.profiles.list = []lichclient.ClaudeProfile{{Slug: personalProfile, Name: "Personal"}, {Slug: "work", Name: "Work", Email: "w@x.y"}}
	if got := m.resumeProfileLine(lichclient.Session{Kind: claudeKind}); got != "" {
		t.Errorf("Personal said %q", got)
	}
	if got := m.resumeProfileLine(lichclient.Session{Kind: "codex", Profile: "work"}); got != "" {
		t.Errorf("another provider said %q", got)
	}
	got := m.resumeProfileLine(lichclient.Session{Kind: claudeKind, Profile: "work"})
	if got != "It resumes in Work · w@x.y, the profile it started in." {
		t.Errorf("line = %q", got)
	}
}

func TestContinueInOffersOnlyOtherLoggedInProfiles(t *testing.T) {
	m := newModel(nil, func() {})
	m.profiles.list = []lichclient.ClaudeProfile{
		{Slug: personalProfile, Name: "Personal", LoggedIn: true},
		{Slug: "work", Name: "Work", LoggedIn: true},
		{Slug: "side", Name: "Side"},
	}
	m.quota.seed(profileQuotaKey("work"), []lichclient.QuotaPlan{{Provider: claudeKind, Status: "ok", Windows: []lichclient.QuotaWindow{{Seconds: 5 * hour, Percent: 100}}}})
	got := m.continueTargets(claudeKind, "")
	if len(got) != 1 || got[0].slug != "work" || got[0].usage != profileUsageLimit || !got[0].limited {
		t.Errorf("from Personal = %+v", got)
	}
	if got := m.continueTargets(claudeKind, "work"); len(got) != 1 || got[0].slug != personalProfile {
		t.Errorf("from work = %+v", got)
	}
	if got := m.continueTargets("codex", ""); got != nil {
		t.Errorf("Codex was offered %+v", got)
	}
	m.profiles.list[1].LoggedIn = false
	if got := m.continueTargets(claudeKind, ""); got != nil {
		t.Errorf("with one login left: %+v", got)
	}
}

func TestSessionProfileEventMovesTheCard(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{{ID: "p", Sessions: []lichclient.Session{{ID: "s1", Kind: claudeKind}}}}
	m.setProfile("s1", "work")
	if got := m.projects[0].Sessions[0].Profile; got != "work" {
		t.Errorf("profile = %q", got)
	}
	if got := m.profileNameOf(claudeKind, "work"); got != "work" {
		t.Errorf("an unknown profile is named by its slug, got %q", got)
	}
	if got := m.profileNameOf(claudeKind, ""); got != "" {
		t.Errorf("Personal carries no tag, got %q", got)
	}
}

func TestADeletedProfileClosesTheDialogAndHasSettingsReadAgain(t *testing.T) {
	b := &profileBackend{}
	s := flowScreen(t, b)
	f := &s.providers.claude.flow
	s.startDelete(lichclient.ClaudeProfile{Slug: "work", Name: "Work"})
	s.confirmFlow(f.view())
	eventually(t, "the delete reached the backend", func() bool { return !f.view().open })
	if !f.view().refresh {
		t.Error("a project set to the deleted profile would keep showing it: Settings was not asked to read again")
	}
	if !slices.Contains(b.log(), `claudeprofile.DeleteProfile["work"]`) {
		t.Errorf("calls = %v", b.log())
	}
}

func TestANewProfileDialogOpensOnAnEmptyName(t *testing.T) {
	b := &profileBackend{}
	s := flowScreen(t, b)
	f := &s.providers.claude.flow
	f.name.SetText("left over")
	s.startNewProfile()
	if got := f.name.Text(); got != "" {
		t.Errorf("name = %q, want it emptied", got)
	}
	if s.canConfirm(f.view(), f) {
		t.Error("an empty name can be created")
	}
}

func TestSettingsFollowsThePlanReadingsItDraws(t *testing.T) {
	r := newQuotaRig()
	r.set([]lichclient.QuotaPlan{quotaPlan()}, nil)
	s := flowScreen(t, &profileBackend{})
	s.footer = &footer{quota: r.q}
	s.open, s.section = true, sectionProviders
	s.providers.book.machine = providersMachine{loaded: true, globals: map[string]string{claudeProfileKey: "work"}}

	if got := s.planKeys(scopedValues{}); len(got) != 1 || got[0] != "" {
		t.Errorf("provider list follows %q, want the machine's reading alone", got)
	}
	s.providers.open = claudeKind
	want := []string{"", "profile:work"}
	if got := s.planKeys(scopedValues{}); !slices.Equal(got, want) {
		t.Errorf("Claude Code's screen follows %q, want %q", got, want)
	}
	s.providers.book.machine.globals[claudeProfileKey] = personalProfile
	if got := s.planKeys(scopedValues{}); len(got) != 1 {
		t.Errorf("Personal follows %q, want the machine's reading alone", got)
	}

	s.watchPlans(want...)
	waitChanged(t, r.changed)
	waitChanged(t, r.changed)
	if _, ok := s.claudePlan("work"); !ok {
		t.Error("the watched profile's reading was not found in the store")
	}
	s.watchPlans("")
	if len(s.providers.unwatch) != 1 {
		t.Errorf("%d readings followed after the profile screen left, want the machine's alone", len(s.providers.unwatch))
	}
	s.watchPlans()
	if len(s.providers.unwatch) != 0 {
		t.Errorf("%d readings still followed after the pane left", len(s.providers.unwatch))
	}
	s.open = false
	if got := s.planKeys(scopedValues{}); got != nil {
		t.Errorf("a closed screen follows %q", got)
	}
}
