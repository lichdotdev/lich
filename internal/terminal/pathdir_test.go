package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWithPathDirPutsTheFolderAhead(t *testing.T) {
	sep := string(os.PathListSeparator)
	dir := filepath.Join("home", "u", ".local", "bin")
	got := WithPathDir([]string{"HOME=/h", "PATH=/usr/bin" + sep + "/bin"}, dir)
	if !slices.Equal(got, []string{"HOME=/h", "PATH=" + dir + sep + "/usr/bin" + sep + "/bin"}) {
		t.Errorf("got %q", got)
	}
	if again := WithPathDir(got, dir); !slices.Equal(again, got) {
		t.Errorf("a folder already on PATH went in twice: %q", again)
	}
	if got := WithPathDir([]string{"HOME=/h"}, dir); !slices.Equal(got, []string{"HOME=/h", "PATH=" + dir}) {
		t.Errorf("no PATH: %q", got)
	}
}
