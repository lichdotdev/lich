package codexprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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
	if name != LoginEventName || !ok {
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
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	infos, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if infos[1].LoggedIn || infos[1].StatusError != "" {
		t.Errorf("a profile nobody logged in to reads as %+v", infos[1])
	}
	if err := s.Login(p.Slug); err != nil {
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
	infos, err = s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if !infos[1].LoggedIn {
		t.Errorf("profile info %+v", infos[1])
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
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	infos, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if infos[1].Email != "" || asked[p.Slug] {
		t.Errorf("a profile nobody is logged in to was asked for its account: %+v", infos[1])
	}
	if err := s.Login(p.Slug); err != nil {
		t.Fatal(err)
	}
	rec.wait(t)
	infos, err = s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if infos[1].Email != "w@example.com" || infos[1].Plan != "Plus" {
		t.Errorf("profile info %+v, want the account the reader named", infos[1])
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
	s.CancelLogin(p.Slug)
	done := rec.wait(t)
	if !done.Cancelled || done.LoggedIn {
		t.Errorf("finished with %+v", done)
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

func TestStatusErrorIsKeptInTheCLIsWords(t *testing.T) {
	machine(t)
	s := New(missingBins{filepath.Join(t.TempDir(), "codex")}, func(string, any) {})
	infos, err := s.Profiles()
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
	infos, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].LoggedIn || !strings.Contains(infos[0].StatusError, "invalid config.toml") {
		t.Errorf("a status that failed for another reason read as %+v", infos[0])
	}
}
