package codexprofile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// fakeCodexGuard turns the test binary into a stand-in for the Codex CLI
// (TestMain), answering from the CODEX_HOME it was run with: logged in once an
// auth.json is there, which its `login` writes.
const fakeCodexGuard = "LICH_TEST_FAKE_CODEX"

func TestMain(m *testing.M) {
	if os.Getenv(fakeCodexGuard) == "" {
		os.Exit(m.Run())
	}
	os.Exit(fakeCodex(os.Args[1:]))
}

func fakeCodex(args []string) int {
	dir := os.Getenv(HomeVar)
	if os.Getenv("CODEX_API_KEY") != "" {
		fmt.Println("a key leaked into the profile")
		return 3
	}
	switch strings.Join(args, " ") {
	case "login status":
		if os.Getenv("LICH_TEST_FAKE_BROKEN") != "" {
			fmt.Fprintln(os.Stderr, "Error: invalid config.toml")
			return 1
		}
		if _, err := os.Stat(filepath.Join(dir, "auth.json")); err != nil {
			fmt.Fprintln(os.Stderr, "Not logged in")
			return 1
		}
		fmt.Fprintln(os.Stderr, "Logged in using ChatGPT")
		return 0
	case "login":
		fmt.Println("Starting local login server on http://localhost:1455.")
		fmt.Println("https://auth.openai.com/oauth/authorize?response_type=code")
		if os.Getenv("LICH_TEST_FAKE_LOGIN_WAITS") != "" {
			time.Sleep(time.Minute)
		}
		_ = os.WriteFile(filepath.Join(dir, "auth.json"), []byte("{}"), 0o600)
		return 0
	}
	if len(args) == 3 && args[0] == "plugin" && args[1] == "add" {
		return fakePluginAdd(dir, args[2])
	}
	if len(args) == 4 && strings.Join(args[:3], " ") == "plugin marketplace upgrade" {
		_ = os.WriteFile(filepath.Join(dir, "upgraded-"+args[3]), nil, 0o600)
		return 0
	}
	return 2
}

// fakePluginAdd installs key the way Codex records it, refusing a marketplace
// the profile has not declared (except "builtin", one Codex knows itself) and
// any plugin named "refused".
func fakePluginAdd(dir, key string) int {
	name, market, _ := strings.Cut(key, "@")
	config := filepath.Join(dir, "config.toml")
	data, _ := os.ReadFile(config)
	if market != "builtin" && !strings.Contains(string(data), "[marketplaces."+market+"]") {
		fmt.Printf("Error: marketplace `%s` is not configured\n", market)
		return 1
	}
	if name == "refused" {
		fmt.Println("Error: plugin `refused` is not available to this account")
		return 1
	}
	data = append(data, fmt.Sprintf("\n[plugins.%q]\nenabled = true\n", key)...)
	_ = os.WriteFile(config, data, 0o600)
	return 0
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
	if name != api.CodexProfileLoginEvent.Name || !ok {
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
	t.Setenv(fakeCodexGuard, "1")
	t.Setenv("CODEX_API_KEY", "personal-key")
	rec := newRecorder()
	return New(fakeBins{}, rec.emit), rec
}

func TestLoginLogsTheProfileIn(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	infos, err := s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if infos[1].LoggedIn || infos[1].StatusError != "" {
		t.Errorf("a profile nobody logged in to reads as %+v", infos[1])
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
		t.Fatal(err)
	}
	done := rec.wait(t)
	if !done.LoggedIn || done.Error != "" {
		t.Fatalf("finished with %+v", done)
	}
	sawURL := false
	for _, e := range rec.events {
		if strings.HasPrefix(e.URL, "http://") {
			t.Errorf("the local callback address was taken for the sign-in one: %q", e.URL)
		}
		sawURL = sawURL || strings.HasPrefix(e.URL, "https://auth.openai.com/oauth/authorize")
	}
	if !sawURL {
		t.Errorf("no event carried the sign-in address: %+v", rec.events)
	}
	infos, err = s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if !infos[1].LoggedIn {
		t.Errorf("profile info %+v", infos[1])
	}
}

func TestLoginReportsAllOutputBeforeItIsDone(t *testing.T) {
	machine(t)
	t.Setenv(fakeCodexGuard, "1")
	t.Setenv("CODEX_API_KEY", "personal-key")
	rec := newRecorder()
	slowEmit := func(name string, data any) {
		if e, ok := data.(LoginEvent); ok && !e.Done {
			time.Sleep(100 * time.Millisecond)
		}
		rec.emit(name, data)
	}
	s := New(fakeBins{}, slowEmit)
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
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

func TestALoggedInProfileCarriesItsAccount(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	asked := map[string]bool{}
	s.SetAccount(func(slug string) (string, string) {
		asked[slug] = true
		return "w@example.com", "Plus"
	})
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	infos, err := s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if infos[1].Email != "" || asked[p.Slug] {
		t.Errorf("a profile nobody is logged in to was asked for its account: %+v", infos[1])
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
		t.Fatal(err)
	}
	rec.wait(t)
	infos, err = s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if infos[1].Email != "w@example.com" || infos[1].Plan != "Plus" {
		t.Errorf("profile info %+v, want the account the reader named", infos[1])
	}
}

func TestLoginSaysItIsAuthenticatingOnceTheCLIIsThrough(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
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
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err == nil {
		t.Error("a second login of the same profile started")
	}
	s.cancelLogin(p.Slug)
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
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteProfile(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err == nil {
		t.Fatal("deleted a profile with a session running in it")
	}
	running = 0
	if _, err := s.DeleteProfile(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
		t.Fatal(err)
	}
	if len(forgotten) != 1 || forgotten[0] != p.Slug {
		t.Errorf("settings forgotten for %v", forgotten)
	}
}

func TestStatusErrorIsKeptInTheCLIsWords(t *testing.T) {
	machine(t)
	s := New(missingBins{filepath.Join(t.TempDir(), "codex")}, func(string, any) {})
	infos, err := s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].LoggedIn || infos[0].StatusError == "" {
		t.Errorf("a CLI that could not run read as %+v", infos[0])
	}
}

func TestABrokenConfigIsNotALoggedOutProfile(t *testing.T) {
	machine(t)
	s, _ := fakeService(t)
	t.Setenv("CODEX_API_KEY", "")
	t.Setenv("LICH_TEST_FAKE_BROKEN", "1")
	infos, err := s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].LoggedIn || !strings.Contains(infos[0].StatusError, "invalid config.toml") {
		t.Errorf("a status that failed for another reason read as %+v", infos[0])
	}
}

func TestLoginInstallsTheLichPlugin(t *testing.T) {
	machine(t)
	s, rec := fakeService(t)
	var installedIn []string
	s.SetLichPlugin(func(slug string) error {
		installedIn = append(installedIn, slug)
		return errors.New("no network")
	}, "lich@lich-plugin")
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(t.Context(), api.ProfileSlugOptions{Slug: p.Slug}); err != nil {
		t.Fatal(err)
	}
	done := rec.wait(t)
	if !done.LoggedIn || done.PluginError != "no network" {
		t.Errorf("finished with %+v, want logged in with the plugin's failure", done)
	}
	if len(installedIn) != 1 || installedIn[0] != p.Slug {
		t.Errorf("lich plugin installed into %v", installedIn)
	}
}

func TestPluginsListsAPortableSet(t *testing.T) {
	personal := machine(t)
	s, _ := fakeService(t)
	s.SetLichPlugin(nil, "lich@lich-plugin")
	mustWrite(t, filepath.Join(personal, "config.toml"), `[plugins."b@m"]
enabled = true

[plugins."lich@lich-plugin"]
enabled = true

[plugins."a@m"]
enabled = false
`)
	got, err := s.Plugins(t.Context(), api.ProfileSlugOptions{Slug: Personal})
	if err != nil {
		t.Fatal(err)
	}
	want := []Plugin{{Key: "b@m", Name: "b", Marketplace: "m"}, {Key: "a@m", Name: "a", Marketplace: "m"}}
	if !slices.Equal(got, want) {
		t.Errorf("plugins %+v, want %+v", got, want)
	}
	if _, err := s.Plugins(t.Context(), api.ProfileSlugOptions{Slug: "gone"}); err == nil {
		t.Error("listed the plugins of a profile that does not exist")
	}
}

func TestInstallPluginsBringsTheMarketplaceAsPersonalDeclaresIt(t *testing.T) {
	personal := machine(t)
	s, _ := fakeService(t)
	shared := `[marketplaces.m]
source_type = "git"
source = "https://example.com/m.git"
ref = "v1"

[plugins."a@m"]
enabled = true
`
	mustWrite(t, filepath.Join(personal, "config.toml"), shared)
	p, err := s.CreateProfile(t.Context(), api.CreateProfileOptions{Name: "Work"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.InstallPlugins(t.Context(), api.InstallPluginsOptions{Slug: p.Slug, Keys: []string{"a@m", "--evil@m", "refused@m", "tool@builtin", "x@unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	ok := []bool{true, false, false, true, false}
	for i, r := range got {
		if r.OK != ok[i] {
			t.Errorf("%s: ok %v (%q), want %v", r.Key, r.OK, r.Message, ok[i])
		}
	}
	if len(got) != len(ok) {
		t.Fatalf("results %+v", got)
	}
	if !strings.Contains(got[2].Message, "not available to this account") {
		t.Errorf("refusal carried %q, want Codex's own sentence", got[2].Message)
	}
	config, _ := os.ReadFile(filepath.Join(p.Dir, "config.toml"))
	if !strings.Contains(string(config), "ref = \"v1\"") {
		t.Errorf("the marketplace was not declared with Personal's ref:\n%s", config)
	}
	if strings.Count(string(config), "[marketplaces.m]") != 1 {
		t.Errorf("the marketplace was declared more than once:\n%s", config)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "upgraded-m")); err != nil {
		t.Errorf("the declared marketplace was never fetched: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(personal, "config.toml")); string(data) != shared {
		t.Errorf("Personal's config.toml changed to %q", data)
	}
	infos, err := s.Profiles(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].Plugins != 1 || infos[1].Plugins != 2 {
		t.Errorf("plugin counts %d and %d, want Personal's 1 and the profile's 2", infos[0].Plugins, infos[1].Plugins)
	}
	if _, err := s.InstallPlugins(t.Context(), api.InstallPluginsOptions{Slug: "gone", Keys: []string{"a@m"}}); err == nil {
		t.Error("installed into a profile that does not exist")
	}
}
