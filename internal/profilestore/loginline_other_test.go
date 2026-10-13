//go:build !windows

package profilestore

import "testing"

func TestLoginLineRunsTheCommandAsTheProfile(t *testing.T) {
	s := toy(t.TempDir())
	got := s.LoginLine("/opt/my toy", "auth login", "/profile dir")
	want := "env -u TOY_TOKEN -u TOY_HOME TOY_HOME='/profile dir' '/opt/my toy' auth login"
	if got != want {
		t.Errorf("LoginLine =\n%s\nwant\n%s", got, want)
	}
	if got := s.LoginLine("/opt/my toy", "auth login", ""); got != "'/opt/my toy' auth login" {
		t.Errorf("Personal: %s", got)
	}
}
