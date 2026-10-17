package terminal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeAntigravitySettings(t *testing.T, home string, doc map[string]any) {
	t.Helper()
	path := antigravitySettingsFile(home)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

type antigravityDoc struct {
	TrustedWorkspaces []string `json:"trustedWorkspaces"`
	Theme             string   `json:"theme"`
}

func readAntigravityDoc(t *testing.T, home string) antigravityDoc {
	t.Helper()
	var doc antigravityDoc
	data, err := os.ReadFile(antigravitySettingsFile(home))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func resolved(t *testing.T, dir string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAntigravityTrustCarriesARepositorysAnswerToItsWorktreeAndSubfolders(t *testing.T) {
	t.Parallel()
	main, worktree := gitWorktree(t)
	sub := filepath.Join(main, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	writeAntigravitySettings(t, home, map[string]any{"theme": "dark", "trustedWorkspaces": []string{main}})

	for _, dir := range []string{worktree, sub} {
		if err := trustAntigravityFolder(home, dir, false); err != nil {
			t.Fatal(err)
		}
	}
	doc := readAntigravityDoc(t, home)
	for _, dir := range []string{resolved(t, worktree), resolved(t, sub)} {
		if !slices.Contains(doc.TrustedWorkspaces, dir) {
			t.Errorf("%s is not trusted: %v", dir, doc.TrustedWorkspaces)
		}
	}
	if doc.Theme != "dark" {
		t.Errorf("the other settings did not survive: %+v", doc)
	}
}

func TestAntigravityTrustNeverTrustsWhatNobodyTrustedUnconfined(t *testing.T) {
	t.Parallel()
	main, worktree := gitWorktree(t)
	home := t.TempDir()
	writeAntigravitySettings(t, home, map[string]any{"trustedWorkspaces": []string{filepath.Dir(main)}})
	before, err := os.ReadFile(antigravitySettingsFile(home))
	if err != nil {
		t.Fatal(err)
	}
	if err := trustAntigravityFolder(home, worktree, false); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(antigravitySettingsFile(home))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("an untrusted worktree was trusted:\n%s", after)
	}
}

// A confined session is trusted whatever the list says, and the file is made
// when Antigravity has not written one yet.
func TestAntigravityTrustsAConfinedFolder(t *testing.T) {
	t.Parallel()
	plain := t.TempDir()
	home := t.TempDir()
	if err := trustAntigravityFolder(home, plain, true); err != nil {
		t.Fatal(err)
	}
	if doc := readAntigravityDoc(t, home); !slices.Equal(doc.TrustedWorkspaces, []string{resolved(t, plain)}) {
		t.Errorf("trustedWorkspaces = %v, want the confined folder", doc.TrustedWorkspaces)
	}
	if err := trustAntigravityFolder(home, plain, true); err != nil {
		t.Fatal(err)
	}
	if doc := readAntigravityDoc(t, home); len(doc.TrustedWorkspaces) != 1 {
		t.Errorf("a trusted folder was added twice: %v", doc.TrustedWorkspaces)
	}
}

func TestAntigravityTrustRefusesSettingsItCannotRead(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	path := antigravitySettingsFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{`, `{"trustedWorkspaces":"x"}`} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := trustAntigravityFolder(home, t.TempDir(), true); err == nil {
			t.Errorf("settings %s were read as an empty list", data)
		}
	}
}
