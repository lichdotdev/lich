package claudeprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// fakeClaudeGuard turns the test binary into a stand-in for the Claude Code CLI
// (TestMain), answering from the CLAUDE_CONFIG_DIR it was run with: logged in
// once a .credentials.json is there, which its `auth login` writes.
const fakeClaudeGuard = "LICH_TEST_FAKE_CLAUDE"

func TestMain(m *testing.M) {
	if os.Getenv(fakeClaudeGuard) == "" {
		os.Exit(m.Run())
	}
	os.Exit(fakeClaude(os.Args[1:]))
}

func fakeClaude(args []string) int {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" {
		fmt.Println("a token leaked into the profile")
		return 3
	}
	switch strings.Join(args, " ") {
	case "auth status":
		if os.Getenv("LICH_TEST_FAKE_STATUS_WARNS") != "" {
			fmt.Fprintln(os.Stderr, "(node:1) Warning: something deprecated")
		}
		_, err := os.Stat(filepath.Join(dir, ".credentials.json"))
		if err != nil {
			fmt.Println(`{"loggedIn":false,"authMethod":"none"}`)
			return 1
		}
		fmt.Println(`{"loggedIn":true,"authMethod":"claude.ai","email":"w@example.com","orgName":"Org","subscriptionType":"team"}`)
		return 0
	case "auth login --claudeai":
		fmt.Println("If the browser didn't open, visit: https://claude.com/cai/oauth/authorize?code=x")
		if os.Getenv("LICH_TEST_FAKE_LOGIN_WAITS") != "" {
			time.Sleep(time.Minute)
		}
		_ = os.WriteFile(filepath.Join(dir, ".credentials.json"), []byte("{}"), 0o600)
		return 0
	}
	if len(args) == 3 && args[0] == "plugin" && args[1] == "install" {
		if strings.HasPrefix(args[2], "blocked") {
			fmt.Fprintln(os.Stderr, "Plugin "+args[2]+" is blocked by your organization")
			return 1
		}
		return 0
	}
	return 2
}

type fakeBins struct{}

func (fakeBins) ProviderBin(_, _ string) string { return os.Args[0] }

type missingBins struct{ path string }

func (b missingBins) ProviderBin(_, _ string) string { return b.path }

// recorder collects the events a login emits.
type recorder struct {
	mu     sync.Mutex
	events []LoginEvent
	done   chan LoginEvent
}

func newRecorder() *recorder { return &recorder{done: make(chan LoginEvent, 1)} }

func (r *recorder) emit(name string, data any) {
	e, ok := data.(LoginEvent)
	if name != api.ClaudeProfileLoginEvent.Name || !ok {
		return
	}
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
	if e.Done {
		r.done <- e
	}
}

func (r *recorder) wait(t *testing.T) LoginEvent {
	t.Helper()
	select {
	case e := <-r.done:
		return e
	case <-time.After(30 * time.Second):
		t.Fatal("the login never finished")
		return LoginEvent{}
	}
}

func fakeService(t *testing.T) (*Service, *recorder) {
	t.Helper()
	t.Setenv(fakeClaudeGuard, "1")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "personal-token")
	rec := newRecorder()
	return New(fakeBins{}, rec.emit), rec
}

func TestLoginLogsTheProfileInAndInstallsLich(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	var installedIn []string
	s.SetLichPlugin(func(slug string) error { installedIn = append(installedIn, slug); return nil }, "lich@lich-plugin")
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Login(p.Slug); err != nil {
		t.Fatal(err)
	}
	done := rec.wait(t)
	if !done.LoggedIn || done.Error != "" {
		t.Fatalf("finished with %+v", done)
	}
	if len(installedIn) != 1 || installedIn[0] != p.Slug {
		t.Errorf("lich plugin installed into %v", installedIn)
	}
	sawURL := false
	for _, e := range rec.events {
		sawURL = sawURL || strings.HasPrefix(e.URL, "https://claude.com/cai/oauth/authorize")
	}
	if !sawURL {
		t.Errorf("no event carried the sign-in address: %+v", rec.events)
	}

	infos, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	work := infos[1]
	if !work.LoggedIn || work.Email != "w@example.com" || work.Plan != "team" || work.OrgName != "Org" {
		t.Errorf("profile info %+v", work)
	}
}

func TestLoginReportsAllOutputBeforeItIsDone(t *testing.T) {
	machine(t)
	t.Setenv(fakeClaudeGuard, "1")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "personal-token")
	rec := newRecorder()
	slowEmit := func(name string, data any) {
		if e, ok := data.(LoginEvent); ok && !e.Done {
			time.Sleep(100 * time.Millisecond)
		}
		rec.emit(name, data)
	}
	s := New(fakeBins{}, slowEmit)
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Login(p.Slug); err != nil {
		t.Fatal(err)
	}
	rec.wait(t)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	last := rec.events[len(rec.events)-1]
	if !last.Done {
		t.Errorf("an event came after the last one: %+v", rec.events)
	}
	if len(rec.events) < 2 {
		t.Errorf("the login was done before its output arrived: %+v", rec.events)
	}
}

func TestLoginSaysItIsAuthenticatingOnceTheCLIIsThrough(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Login(p.Slug); err != nil {
		t.Fatal(err)
	}
	rec.wait(t)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	n := len(rec.events)
	if n < 2 || !rec.events[n-2].Authenticating || rec.events[n-2].Done {
		t.Errorf("the event before the last one is not Authenticating: %+v", rec.events)
	}
}

func TestCancelledLoginSaysSo(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	t.Setenv("LICH_TEST_FAKE_LOGIN_WAITS", "1")
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Login(p.Slug); err != nil {
		t.Fatal(err)
	}
	if err := s.Login(p.Slug); err == nil {
		t.Error("a second login of the same profile started")
	}
	if err := s.LoginCode(p.Slug, "abc"); err != nil {
		t.Errorf("hand the code over: %v", err)
	}
	s.CancelLogin(p.Slug)
	done := rec.wait(t)
	if !done.Cancelled || done.LoggedIn {
		t.Errorf("finished with %+v", done)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, e := range rec.events {
		if e.Authenticating {
			t.Errorf("a cancelled login said it was authenticating: %+v", rec.events)
		}
	}
	if err := s.LoginCode(p.Slug, "abc"); err == nil {
		t.Error("a code was accepted with no login running")
	}
}

func TestInstallPluginsReportsEachInClaudesWords(t *testing.T) {
	machine(t)
	s, _ := fakeService(t)
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.InstallPlugins(p.Slug, []string{"good@market", "blocked@market", "--flag"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || !got[0].OK || got[1].OK || got[2].OK {
		t.Fatalf("results %+v", got)
	}
	if !strings.Contains(got[1].Message, "blocked by your organization") {
		t.Errorf("refusal carried %q, want Claude's own sentence", got[1].Message)
	}
	if _, err := s.InstallPlugins("gone", []string{"good@market"}); err == nil {
		t.Error("installed into a profile that does not exist")
	}
}

func TestPluginsListsAPortableSet(t *testing.T) {
	personal := machine(t)
	s, _ := fakeService(t)
	s.SetLichPlugin(nil, "lich@lich-plugin")
	doc := map[string]any{"version": 2, "plugins": map[string]any{
		"lich@lich-plugin": []any{map[string]any{"scope": "user", "version": "0.20.0"}},
		"b@m":              []any{map[string]any{"scope": "user", "version": "2"}},
		"a@m":              []any{map[string]any{"scope": "user", "version": "1"}},
		"proj@m":           []any{map[string]any{"scope": "project"}},
	}}
	data, _ := json.Marshal(doc)
	mustWrite(t, filepath.Join(personal, "plugins", "installed_plugins.json"), string(data))
	got, err := s.Plugins(Personal)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Key != "a@m" || got[1].Key != "b@m" || got[0].Marketplace != "m" || got[0].Name != "a" {
		t.Errorf("plugins %+v", got)
	}
}

func TestPersonalPluginsAreReadWhereClaudeKeepsThem(t *testing.T) {
	personal := machine(t)
	moved := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", moved)
	s, _ := fakeService(t)
	mustWrite(t, filepath.Join(personal, "plugins", "installed_plugins.json"),
		`{"version":2,"plugins":{"stale@m":[{"scope":"user"}]}}`)
	mustWrite(t, filepath.Join(moved, "installed_plugins.json"),
		`{"version":2,"plugins":{"live@m":[{"scope":"user"}]}}`)
	got, err := s.Plugins(Personal)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "live@m" {
		t.Errorf("plugins %+v, want live@m", got)
	}
}

func TestDeleteProfileWaitsForItsSessions(t *testing.T) {
	machine(t)
	s, _ := fakeService(t)
	running := 1
	var forgotten []string
	s.SetSessions(func(string) int { return running }, func(slug string) error {
		forgotten = append(forgotten, slug)
		return nil
	})
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProfile(p.Slug); err == nil {
		t.Fatal("deleted a profile with a session running in it")
	}
	running = 0
	if err := s.DeleteProfile(p.Slug); err != nil {
		t.Fatal(err)
	}
	if len(forgotten) != 1 || forgotten[0] != p.Slug {
		t.Errorf("settings forgotten for %v", forgotten)
	}
}

func TestStatusReadsPastAWarningOnStderr(t *testing.T) {
	machine(t)
	s, _ := fakeService(t)
	t.Setenv("LICH_TEST_FAKE_STATUS_WARNS", "1")
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Dir, ".credentials.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	infos, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if work := infos[1]; !work.LoggedIn || work.StatusError != "" {
		t.Errorf("a status with a warning on stderr read as %+v", work)
	}
}

func TestStatusErrorIsKeptInTheCLIsWords(t *testing.T) {
	machine(t)
	s := New(missingBins{filepath.Join(t.TempDir(), "claude")}, func(string, any) {})
	infos, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].LoggedIn || infos[0].StatusError == "" {
		t.Errorf("a CLI that answered no JSON read as %+v", infos[0])
	}
}
