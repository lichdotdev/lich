//go:build !windows

package codexprofile

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

func TestLoginCommandRunsAsTheProfile(t *testing.T) {
	machine(t)
	s := New(missingBins{"/opt/my codex"}, func(string, any) {})
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LoginCommand(t.Context(), api.ProfileSlugOptions{Slug: p.Slug})
	if err != nil {
		t.Fatal(err)
	}
	want := "env -u CODEX_API_KEY -u CODEX_ACCESS_TOKEN -u OPENAI_API_KEY -u CODEX_SQLITE_HOME -u CODEX_HOME" +
		" CODEX_HOME='" + p.Dir + "' '/opt/my codex' login"
	if got != want {
		t.Errorf("LoginCommand =\n%s\nwant\n%s", got, want)
	}
	if got, _ := s.LoginCommand(t.Context(), api.ProfileSlugOptions{Slug: Personal}); got != "'/opt/my codex' login" {
		t.Errorf("Personal: %s", got)
	}
	if _, err := s.LoginCommand(t.Context(), api.ProfileSlugOptions{Slug: "gone"}); err == nil {
		t.Error("a command for a profile that does not exist")
	}
}
