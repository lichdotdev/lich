//go:build windows

package codexprofile

import (
	"strings"
	"testing"
)

func TestLoginCommandRunsAsTheProfile(t *testing.T) {
	machine(t)
	s := New(missingBins{`C:\Tools\codex.exe`}, func(string, any) {})
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LoginCommand(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	want := "Remove-Item Env:CODEX_API_KEY,Env:CODEX_ACCESS_TOKEN,Env:OPENAI_API_KEY,Env:CODEX_SQLITE_HOME," +
		"Env:CODEX_HOME -ErrorAction SilentlyContinue; " +
		"$env:CODEX_HOME = '" + p.Dir + "'; & 'C:\\Tools\\codex.exe' login"
	if got != want {
		t.Errorf("LoginCommand =\n%s\nwant\n%s", got, want)
	}
	if got, _ := s.LoginCommand(Personal); !strings.HasPrefix(got, "& ") {
		t.Errorf("Personal: %s", got)
	}
}
