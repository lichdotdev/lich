package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedDiffShowsOnlyTheLinesThatChanged(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, old, next, want string
	}{
		{"one line", "a\nb\nc\n", "a\nB\nc\n", "--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"},
		{"a line added", "a\nc", "a\nb\nc", "--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,3 @@\n a\n+b\n c\n"},
		{"two changes far apart", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12", "x\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\ny",
			"--- a/a.go\n+++ b/a.go\n@@ -1,4 +1,4 @@\n-1\n+x\n 2\n 3\n 4\n@@ -9,4 +9,4 @@\n 9\n 10\n 11\n-12\n+y\n"},
		{"new file", "", "one\ntwo\n", "--- /dev/null\n+++ b/a.go\n@@ -0,0 +1,2 @@\n+one\n+two\n"},
		{"unchanged", "same", "same", ""},
	} {
		if got := unifiedDiff("a.go", c.old, c.next); got != c.want {
			t.Errorf("%s: diff = %q, want %q", c.name, got, c.want)
		}
	}
}

// An edit names a stretch of the file, not its place: the diff is numbered
// from where the stretch sits in the file, before the edit or after it.
func TestUnifiedDiffIsNumberedWhereTheEditSitsInTheFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "a.go")
	lines := []string{"l1", "l2", "l3", "l4", "five", "six", "l7"}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "--- a/" + path + "\n+++ b/" + path + "\n@@ -5,2 +5,2 @@\n five\n-six\n+SIX\n"
	if got := unifiedDiff(path, "five\nsix", "five\nSIX"); got != want {
		t.Errorf("before the edit: diff = %q, want %q", got, want)
	}
	if got := unifiedDiff(path, "five\nseis", "five\nsix"); !strings.Contains(got, "@@ -5,2 +5,2 @@") {
		t.Errorf("after the edit: diff = %q, want it numbered from line 5", got)
	}
}

func TestUnifiedDiffReplacesAStretchTooLongToCompare(t *testing.T) {
	t.Parallel()
	var old, next []string
	for i := range 3000 {
		old, next = append(old, "o"+strings.Repeat("x", i%7)), append(next, "n")
	}
	got := unifiedDiff("a.go", strings.Join(old, "\n"), strings.Join(next, "\n"))
	if !strings.Contains(got, "@@ -1,3000 +1,3000 @@") || strings.Count(got, "\n-") != 3000 {
		t.Errorf("diff head = %q, want every line replaced", got[:min(len(got), 80)])
	}
}
