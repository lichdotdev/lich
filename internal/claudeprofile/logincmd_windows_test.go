//go:build windows

package claudeprofile

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestLoginCommandRunsAsTheProfile(t *testing.T) {
	machine(t)
	s := New(missingBins{`C:\Tools\claude.exe`}, func(string, any) {})
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LoginCommand(t.Context(), api.ProfileSlugOptions{Slug: p.Slug})
	if err != nil {
		t.Fatal(err)
	}
	want := "Remove-Item Env:CLAUDE_CODE_OAUTH_TOKEN,Env:ANTHROPIC_API_KEY,Env:ANTHROPIC_AUTH_TOKEN," +
		"Env:CLAUDE_SECURESTORAGE_CONFIG_DIR,Env:CLAUDE_CODE_PLUGIN_CACHE_DIR,Env:CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue; " +
		"$env:CLAUDE_CONFIG_DIR = '" + p.Dir + "'; & 'C:\\Tools\\claude.exe' auth login --claudeai"
	if got != want {
		t.Errorf("LoginCommand =\n%s\nwant\n%s", got, want)
	}
	if got, _ := s.LoginCommand(t.Context(), api.ProfileSlugOptions{Slug: Personal}); !strings.HasPrefix(got, "& ") {
		t.Errorf("Personal: %s", got)
	}
}
