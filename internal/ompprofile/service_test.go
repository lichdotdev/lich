package ompprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeOmpGuard turns the test binary into a stand-in for the omp CLI
// (TestMain): `usage accounts --json` answers from the profile it was run in,
// which holds one account once an accounts.json is in its agent directory.
const fakeOmpGuard = "LICH_TEST_FAKE_OMP"

func TestMain(m *testing.M) {
	if os.Getenv(fakeOmpGuard) == "" {
		os.Exit(m.Run())
	}
	os.Exit(fakeOmp(os.Args[1:]))
}

func fakeOmp(args []string) int {
	if os.Getenv(HomeVar) != "" && os.Getenv("OMP_AUTH_BROKER_URL") != "" {
		fmt.Println("a broker leaked into the profile")
		return 3
	}
	if strings.Join(args, " ") != "usage accounts --json" {
		return 2
	}
	if os.Getenv("LICH_TEST_FAKE_BROKEN") != "" {
		fmt.Fprintln(os.Stderr, "Error: invalid config.yml")
		return 1
	}
	if os.Getenv("LICH_TEST_FAKE_WARNS") != "" {
		fmt.Fprintln(os.Stderr, "(node:1) Warning: something deprecated")
	}
	dir, err := PersonalDir()
	if err != nil {
		return 4
	}
	rows, err := os.ReadFile(filepath.Join(dir, "accounts.json"))
	if err != nil {
		fmt.Println(`{"accounts": []}`)
		return 0
	}
	fmt.Printf(`{"accounts": %s}`+"\n", rows)
	return 0
}

type fakeBins struct{}

func (fakeBins) ProviderBin(_, _ string) string { return os.Args[0] }

type missingBins struct{ path string }

func (b missingBins) ProviderBin(_, _ string) string { return b.path }

func fakeService(t *testing.T) *Service {
	t.Helper()
	t.Setenv(fakeOmpGuard, "1")
	t.Setenv("OMP_AUTH_BROKER_URL", "https://broker.example")
	return New(fakeBins{})
}

func TestProfilesReportWhoIsLoggedInWhere(t *testing.T) {
	machine(t)
	s := fakeService(t)
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(p.Dir, "accounts.json"),
		`[{"provider":"anthropic","identityKey":"email:me@work.example|org:o1"},`+
			`{"provider":"openai-codex","identityKey":"email:me@work.example"},`+
			`{"provider":"github-copilot","identityKey":null}]`)

	list, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("profiles = %+v", list)
	}
	if list[0].LoggedIn || list[0].StatusError != "" {
		t.Errorf("Personal = %+v, want logged out and no error", list[0])
	}
	work := list[1]
	if !work.LoggedIn || work.Email != "me@work.example" || strings.Join(work.Providers, ",") != "anthropic,openai-codex,github-copilot" {
		t.Errorf("Work = %+v", work)
	}
}

func TestAProfileTheCLICannotReadIsAnErrorNotALoggedOutProfile(t *testing.T) {
	machine(t)
	s := fakeService(t)
	t.Setenv("LICH_TEST_FAKE_BROKEN", "1")
	list, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if list[0].LoggedIn || !strings.Contains(list[0].StatusError, "invalid config.yml") {
		t.Errorf("Personal = %+v", list[0])
	}
}

func TestProfilesReadPastAWarningOnStderr(t *testing.T) {
	machine(t)
	s := fakeService(t)
	t.Setenv("LICH_TEST_FAKE_WARNS", "1")
	list, err := s.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if list[0].StatusError != "" {
		t.Errorf("a list with a warning on stderr read as %+v", list[0])
	}
}

func TestDeleteProfileWaitsForItsSessions(t *testing.T) {
	machine(t)
	s := fakeService(t)
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	var forgotten []string
	live := 1
	s.SetSessions(func(string) int { return live }, func(slug string) error {
		forgotten = append(forgotten, slug)
		return nil
	})
	if err := s.DeleteProfile(p.Slug); err == nil {
		t.Fatal("deleted a profile with a session running in it")
	}
	if _, err := ConfigDir(p.Slug); err != nil {
		t.Errorf("a refused delete still removed it: %v", err)
	}
	live = 0
	if err := s.DeleteProfile(p.Slug); err != nil {
		t.Fatal(err)
	}
	if len(forgotten) != 1 || forgotten[0] != p.Slug {
		t.Errorf("forgotten = %v", forgotten)
	}
}

func TestLoginCommandRunsAsTheProfile(t *testing.T) {
	machine(t)
	s := New(missingBins{"/opt/my omp"})
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LoginCommand(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	var want string
	if os.PathSeparator == '/' {
		want = "env -u OMP_AUTH_BROKER_URL -u OMP_AUTH_BROKER_TOKEN -u OMP_PROFILE OMP_PROFILE='work' '/opt/my omp' login"
	} else {
		want = "Remove-Item Env:OMP_AUTH_BROKER_URL,Env:OMP_AUTH_BROKER_TOKEN,Env:OMP_PROFILE -ErrorAction SilentlyContinue; " +
			"$env:OMP_PROFILE = 'work'; & '/opt/my omp' login"
	}
	if got != want {
		t.Errorf("LoginCommand =\n%s\nwant\n%s", got, want)
	}
	if got, _ := s.LoginCommand(Personal); !strings.HasSuffix(got, "'/opt/my omp' login") || strings.Contains(got, "OMP_PROFILE") {
		t.Errorf("Personal: %s", got)
	}
	if _, err := s.LoginCommand("gone"); err == nil {
		t.Error("a command for a profile that does not exist")
	}
}
