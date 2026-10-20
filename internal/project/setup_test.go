package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSetup(t *testing.T, dir, script string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".lich"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lich", "setup-worktree.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestSetupScriptReadsTheRepoFile pins the contract the spawn path relies on:
// the script is .lich/setup-worktree.sh in the given checkout, trimmed, and a
// repository without one answers "" — which is what tells wrapSetup not to
// wrap at all.
func TestSetupScriptReadsTheRepoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := SetupScript(dir); got != "" {
		t.Errorf("missing file: got %q, want empty", got)
	}
	writeSetup(t, dir, "pnpm install\n")
	if got := SetupScript(dir); got != "pnpm install" {
		t.Errorf("got %q, want %q", got, "pnpm install")
	}
}

// A whitespace-only file must read as "no script": wrapping the spawn to run
// nothing would still make the session wait out a marker for an empty setup.
func TestSetupScriptTreatsBlankFileAsAbsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSetup(t, dir, "\n  \n")
	if got := SetupScript(dir); got != "" {
		t.Errorf("blank file: got %q, want empty", got)
	}
}
