package xdgprofile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// fakeCLIGuard turns the test binary into a stand-in for the three CLIs
// (TestMain): each status call answers logged in when a `logged-in` file is in
// the provider's folder under the base directory it was run with.
const fakeCLIGuard = "LICH_TEST_FAKE_XDG_CLI"

func TestMain(m *testing.M) {
	if os.Getenv(fakeCLIGuard) == "" {
		os.Exit(m.Run())
	}
	os.Exit(fakeCLI(os.Args[1:]))
}

func fakeCLI(args []string) int {
	data, config := os.Getenv("XDG_DATA_HOME"), os.Getenv("XDG_CONFIG_HOME")
	leaks := map[string]string{"auth list": "OPENCODE_DB", "status --format json": "CURSOR_API_KEY", "whoami --format json": "KIRO_API_KEY"}
	inProfile := strings.Contains(data+config, "profiles")
	if leaked := leaks[strings.Join(args, " ")]; inProfile && os.Getenv(leaked) != "" {
		fmt.Println(leaked + " leaked into the profile")
		return 3
	}
	logged := func(base, folder string) bool {
		_, err := os.Stat(filepath.Join(base, folder, "logged-in"))
		return err == nil
	}
	switch strings.Join(args, " ") {
	case "auth list":
		n := 0
		if logged(data, "opencode") {
			n = 2
		}
		fmt.Printf("┌  Credentials\n│\n└  %d credentials\n", n)
	case "status --format json":
		if logged(config, "cursor") {
			fmt.Println(`{"isAuthenticated":true,"userInfo":{"email":"me@work.example"}}`)
		} else {
			fmt.Println(`{"status":"unauthenticated","isAuthenticated":false}`)
		}
	case "whoami --format json":
		if !logged(data, "kiro-cli") {
			fmt.Println(`{"account":null}`)
			return 1
		}
		fmt.Println(`{"account":{"email":"me@kiro.example"}}`)
	default:
		return 2
	}
	return 0
}

type fakeBins struct{}

func (fakeBins) ProviderBin(_, _ string) string { return os.Args[0] }

// machine points every base directory at a scratch home holding some other
// tool's state and Personal's own for each provider, and returns the home.
func machine(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("LICH_DEV", "")
	t.Setenv("CURSOR_CONFIG_DIR", "")
	for _, f := range []string{
		".config/gh/hosts.yml", ".config/cursor/cli-config.json",
		".local/share/mise/installs", ".local/share/opencode/opencode.db",
		".local/share/kiro-cli/tui.js", ".local/share/kiro-cli/data.sqlite3",
	} {
		mustWrite(t, filepath.Join(home, f), "personal")
	}
	return home
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAProfileIsALinkHomeWithTheProvidersFolderItsOwn(t *testing.T) {
	home := machine(t)
	cases := []struct {
		p              *Provider
		linked, absent []string
		envVar         string
	}{
		{OpenCode, []string{"mise"}, []string{"opencode"}, "XDG_DATA_HOME"},
		{Cursor, []string{"gh"}, []string{"cursor"}, "XDG_CONFIG_HOME"},
		{Kiro, []string{"mise", "kiro-cli/tui.js"}, []string{"kiro-cli/data.sqlite3"}, "XDG_DATA_HOME"},
	}
	for _, c := range cases {
		p, err := c.p.Create("Work")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(p.Dir, filepath.Join(home, ".config", "lich", "profiles", c.p.ID)) {
			t.Errorf("%s profile lives at %s", c.p.ID, p.Dir)
		}
		for _, name := range c.linked {
			if !profilestore.IsLink(filepath.Join(p.Dir, name)) {
				t.Errorf("%s: %s is not linked", c.p.ID, name)
			}
		}
		for _, name := range c.absent {
			if _, err := os.Lstat(filepath.Join(p.Dir, name)); err == nil {
				t.Errorf("%s: a new profile holds %s, which is its own login's", c.p.ID, name)
			}
		}
		env := c.p.Env([]string{c.envVar + "=/elsewhere", "PATH=/bin"}, p.Dir)
		if !slices.Equal(env, []string{"PATH=/bin", c.envVar + "=" + p.Dir}) {
			t.Errorf("%s: Env = %v", c.p.ID, env)
		}
	}
}

func TestStatesNamePersonalsStateWhereTheCLIReadsIt(t *testing.T) {
	home := machine(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	p, err := Cursor.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	states := Cursor.States()
	if len(states) != 2 {
		t.Fatalf("states = %+v", states)
	}
	// With no XDG_CONFIG_HOME Cursor keeps Personal's chats in ~/.cursor.
	if want := filepath.Join(home, ".cursor"); states[0].Dir != want || !states[0].Personal {
		t.Errorf("Personal's state = %+v, want %s", states[0], want)
	}
	if want := filepath.Join(p.Dir, "cursor"); states[1].Dir != want {
		t.Errorf("Work's state = %s, want %s", states[1].Dir, want)
	}
}

func TestACursorChatMovesBetweenProfilesAndBack(t *testing.T) {
	home := machine(t)
	work, err := Cursor.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	chat := filepath.Join(home, ".config", "cursor", "chats", "abc123", "chat-1")
	mustWrite(t, filepath.Join(chat, "store.db"), "x")

	if err := Cursor.MoveConversation("chat-1", work.Slug); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(work.Dir, "cursor", "chats", "abc123", "chat-1", "store.db")
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("the chat did not reach the profile: %v", err)
	}
	owner, path, ok := Cursor.Owner("chat-1", CursorChat)
	if !ok || owner.Slug != work.Slug || path != filepath.Dir(moved) {
		t.Errorf("Owner = %+v, %s, %v", owner, path, ok)
	}
	if err := Cursor.MoveConversation("chat-1", Personal); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(chat, "store.db")); err != nil {
		t.Errorf("the chat did not come back: %v", err)
	}
	if _, _, ok := Cursor.Owner("../chat-1", CursorChat); ok {
		t.Error("an id with a path in it found a chat")
	}
}

func TestOnlyCursorConversationsMove(t *testing.T) {
	machine(t)
	if _, err := Kiro.Create("Work"); err != nil {
		t.Fatal(err)
	}
	if err := Kiro.MoveConversation("any", "work"); err != nil {
		t.Errorf("Kiro, whose conversations every profile shares: %v", err)
	}
	if err := Kiro.MoveConversation("any", "gone"); !errors.Is(err, ErrProfileGone) {
		t.Errorf("Kiro into a profile that does not exist: %v", err)
	}
	if err := OpenCode.MoveConversation("any", Personal); !errors.Is(err, ErrNoMove) {
		t.Errorf("opencode: %v, want ErrNoMove", err)
	}
}

func TestProvidersAreTheThreeOnLinux(t *testing.T) {
	var ids []string
	for _, p := range Providers() {
		ids = append(ids, p.ID)
	}
	if !slices.Equal(ids, []string{"opencode", "cursor", "kiro"}) || Of("cursor") != Cursor || Of("claude") != nil {
		t.Errorf("Providers = %v", ids)
	}
}

func TestServicesReportWhoIsLoggedInWhere(t *testing.T) {
	machine(t)
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv("CURSOR_API_KEY", "leak")
	t.Setenv("KIRO_API_KEY", "leak")
	t.Setenv("OPENCODE_DB", "/leak")
	for _, prov := range Providers() {
		s := New(prov, fakeBins{}, func(string, any) {})
		p, err := s.CreateProfile("Work")
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(p.Dir, prov.folder, "logged-in"), "")
		list, err := s.Profiles()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 || list[0].LoggedIn || list[0].StatusError != "" {
			t.Fatalf("%s: Personal = %+v", prov.ID, list)
		}
		if !list[1].LoggedIn || list[1].StatusError != "" {
			t.Errorf("%s: Work = %+v", prov.ID, list[1])
		}
	}
}

func TestDeleteRefusesARunningProfileAndForgetsIt(t *testing.T) {
	home := machine(t)
	s := New(Kiro, fakeBins{}, func(string, any) {})
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	running := 1
	var forgot string
	s.SetSessions(func(string) int { return running }, func(slug string) error { forgot = slug; return nil })
	if err := s.DeleteProfile(p.Slug); err == nil {
		t.Fatal("deleted a profile with a session running in it")
	}
	running = 0
	if err := s.DeleteProfile(p.Slug); err != nil || forgot != p.Slug {
		t.Fatalf("delete = %v, forgot %q", err, forgot)
	}
	for _, f := range []string{".local/share/kiro-cli/tui.js", ".local/share/kiro-cli/data.sqlite3", ".local/share/mise/installs"} {
		if _, err := os.Stat(filepath.Join(home, f)); err != nil {
			t.Errorf("deleting the profile took Personal's %s: %v", f, err)
		}
	}
}

func TestLoginCommandRunsTheLoginInTheProfile(t *testing.T) {
	machine(t)
	s := New(Cursor, fakeBins{}, func(string, any) {})
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	line, err := s.LoginCommand(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"-u CURSOR_CONFIG_DIR", "-u CURSOR_API_KEY", "XDG_CONFIG_HOME='" + p.Dir + "'", " login"} {
		if !strings.Contains(line, part) {
			t.Errorf("login line %q lacks %q", line, part)
		}
	}
	if line, _ := s.LoginCommand(Personal); strings.Contains(line, "env") {
		t.Errorf("Personal's login line = %q", line)
	}
	if _, err := s.LoginCommand("gone"); !errors.Is(err, ErrProfileGone) {
		t.Errorf("login of a profile that does not exist: %v", err)
	}
}
