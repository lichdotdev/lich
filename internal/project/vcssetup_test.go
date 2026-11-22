package project

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// The status of a glab signed in to one host and failing another, which glab
// answers with a non-zero exit.
const glabStatusSample = `gitlab.com
  ✓ Logged in to gitlab.com as ana.souza (keyring)
  ✓ Git operations for gitlab.com configured to use ssh protocol.
git.example.org
  x git.example.org: API call failed: 401 Unauthorized
  ! No token found (checked config file, keyring, and environment variables).
`

func TestGitLabAccountsReadsTheSignedInHostsOnly(t *testing.T) {
	s := &Service{glabStatus: func() string { return glabStatusSample }}
	got, err := s.GitLabAccounts(context.Background(), api.None{})
	if err != nil || !reflect.DeepEqual(got, []string{"gitlab.com/ana.souza"}) {
		t.Errorf("accounts %v, %v", got, err)
	}
	s.glabStatus = func() string { return "" }
	if got, _ := s.GitLabAccounts(context.Background(), api.None{}); len(got) != 0 {
		t.Errorf("no glab answered %v", got)
	}
}

func TestGlobalCommitIdentityWritesBothAndReadsThemBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	s := &Service{}
	ctx := context.Background()
	if got, err := s.GlobalCommitIdentity(ctx, api.None{}); err != nil || got != (CommitIdentity{}) {
		t.Fatalf("an empty config: %+v, %v", got, err)
	}
	if _, err := s.SetGlobalCommitIdentity(ctx, api.CommitIdentityOptions{Name: " Ana ", Email: ""}); err == nil {
		t.Error("a name without an email was written")
	}
	if _, err := s.SetGlobalCommitIdentity(ctx, api.CommitIdentityOptions{Name: " Ana Souza ", Email: "ana@example.dev"}); err != nil {
		t.Fatal(err)
	}
	want := CommitIdentity{Name: "Ana Souza", Email: "ana@example.dev"}
	if got, err := s.GlobalCommitIdentity(ctx, api.None{}); err != nil || got != want {
		t.Errorf("read back %+v, %v; want %+v", got, err, want)
	}
}
