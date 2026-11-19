package project

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestWriteFileOverwritesTheTextAndKeepsTheMode(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	write(t, repo, "run.sh", "echo one\r\n")
	full := filepath.Join(repo, "run.sh")
	if err := os.Chmod(full, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "echo two\r\necho three"
	if _, err := New(nil).WriteFile(t.Context(), api.WriteFileOptions{Path: repo, Rel: "run.sh", Content: text}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(full)
	if err != nil || string(got) != text {
		t.Fatalf("file = %q, %v; want %q byte for byte", got, err, text)
	}
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want the file's own 0755", info.Mode().Perm())
	}
}

// A file ReadFile decoded from Windows-1252 would be written back as UTF-8,
// every accented byte of it changed.
func TestWriteFileRefusesTextThatIsNotUTF8(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	old := "Servi\xe7os\n"
	write(t, repo, "menu.xml", old)
	if _, err := New(nil).WriteFile(t.Context(), api.WriteFileOptions{Path: repo, Rel: "menu.xml", Content: "Serviços\n"}); err == nil {
		t.Fatal("WriteFile wrote over a Latin-1 file, want an error")
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "menu.xml")); string(got) != old {
		t.Errorf("file = %q, want it untouched", got)
	}
}

func TestWriteFileRefusesWhatIsNoFileOfTheCheckout(t *testing.T) {
	t.Parallel()
	repo, _ := initRepo(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	rels := []string{"../outside.txt", "/etc/passwd", "a/../../b", "missing.txt", "dir"}
	if err := os.Symlink(outside, filepath.Join(repo, "link.txt")); err == nil {
		rels = append(rels, "link.txt")
	}
	for _, rel := range rels {
		if _, err := New(nil).WriteFile(t.Context(), api.WriteFileOptions{Path: repo, Rel: rel, Content: "x"}); err == nil {
			t.Errorf("WriteFile(%q) = nil, want an error", rel)
		}
	}
	if got, _ := os.ReadFile(outside); string(got) != "keep\n" {
		t.Errorf("the link's target = %q, want it untouched", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "missing.txt")); !os.IsNotExist(err) {
		t.Errorf("WriteFile made a file that was not there: %v", err)
	}
}
