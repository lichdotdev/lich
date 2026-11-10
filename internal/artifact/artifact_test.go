package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "report.html")
	if err := os.WriteFile(page, []byte("<h1>report</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return page
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestSessionDirRefusesAKeyThatIsNotOneElement(t *testing.T) {
	root := t.TempDir()
	if got := SessionDir(root, "s1"); got != filepath.Join(root, "s1") {
		t.Fatalf("SessionDir(s1) = %q", got)
	}
	for _, id := range []string{"", ".", "..", "a/b", "../s1"} {
		if got := SessionDir(root, id); got != "" {
			t.Errorf("SessionDir(%q) = %q, want empty", id, got)
		}
	}
	if got := SessionDir("", "s1"); got != "" {
		t.Errorf("SessionDir with no root = %q, want empty", got)
	}
}

func TestPurgeDeletesOnlyThatSession(t *testing.T) {
	root := t.TempDir()
	gone, kept := write(t, root, "s1"), write(t, root, "s2")
	New(root, nil).Purge("s1")
	if exists(gone) || !exists(kept) {
		t.Fatalf("after Purge(s1): s1 exists %v, s2 exists %v", exists(gone), exists(kept))
	}
	New(root, nil).Purge("..")
	if !exists(root) {
		t.Fatal("Purge(..) removed the root")
	}
}

func TestPruneOrphansKeepsTheSessionsStillThere(t *testing.T) {
	root := t.TempDir()
	orphan, parked := write(t, root, "gone"), write(t, root, "parked")
	New(root, func(id string) bool { return id == "parked" }).PruneOrphans()
	if exists(orphan) || !exists(parked) {
		t.Fatalf("after prune: orphan exists %v, parked exists %v", exists(orphan), exists(parked))
	}
	New(filepath.Join(root, "missing"), func(string) bool { return false }).PruneOrphans()
}
