package extensions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenameExitedWaitsOutAFileStillHeld(t *testing.T) {
	from := t.TempDir()
	held := filepath.Join(from, testExecutable())
	writeFile(t, held, "x")
	f, err := os.Open(held)
	if err != nil {
		t.Fatal(err)
	}
	to := filepath.Join(t.TempDir(), replacedDir)
	if err := os.Rename(from, to); !renameBusy(err) {
		f.Close()
		t.Fatalf("os.Rename with a file in it held = %v, want it refused as busy", err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		f.Close()
	}()
	if err := renameExited(from, to); err != nil {
		t.Fatalf("renameExited() = %v, want it to wait out the handle", err)
	}
}
