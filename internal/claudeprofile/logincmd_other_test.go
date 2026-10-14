//go:build !windows

package claudeprofile

import "testing"

func TestLoginCommandRunsAsTheProfile(t *testing.T) {
	machine(t)
	s := New(missingBins{"/opt/my claude"}, func(string, any) {})
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LoginCommand(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	want := "env -u CLAUDE_CODE_OAUTH_TOKEN -u ANTHROPIC_API_KEY -u ANTHROPIC_AUTH_TOKEN" +
		" -u CLAUDE_SECURESTORAGE_CONFIG_DIR -u CLAUDE_CODE_PLUGIN_CACHE_DIR -u CLAUDE_CONFIG_DIR CLAUDE_CONFIG_DIR='" + p.Dir +
		"' '/opt/my claude' auth login --claudeai"
	if got != want {
		t.Errorf("LoginCommand =\n%s\nwant\n%s", got, want)
	}
	if got, _ := s.LoginCommand(Personal); got != "'/opt/my claude' auth login --claudeai" {
		t.Errorf("Personal: %s", got)
	}
	if _, err := s.LoginCommand("gone"); err == nil {
		t.Error("a command for a profile that does not exist")
	}
}
