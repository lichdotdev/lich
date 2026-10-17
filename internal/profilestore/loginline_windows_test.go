//go:build windows

package profilestore

import (
	"path/filepath"
	"testing"
)

func TestLoginLineSetsTheProfileInPowerShell(t *testing.T) {
	s := toy(t.TempDir())
	s.Pinned = []PinnedVar{{Name: "TOY_STATE", Rel: "Toy"}}
	dir := filepath.Join(`C:\profiles`, "work")
	got := s.LoginLine(`C:\toy\toy.exe`, "login", dir)
	want := "Remove-Item Env:TOY_TOKEN,Env:TOY_HOME -ErrorAction SilentlyContinue; " +
		"$env:TOY_HOME = 'C:\\profiles\\work'; $env:TOY_STATE = 'C:\\profiles\\work\\Toy'; & 'C:\\toy\\toy.exe' login"
	if got != want {
		t.Errorf("LoginLine =\n%s\nwant\n%s", got, want)
	}
	if got := s.LoginLine(`C:\toy\toy.exe`, "login", ""); got != `& 'C:\toy\toy.exe' login` {
		t.Errorf("Personal: %s", got)
	}
}
