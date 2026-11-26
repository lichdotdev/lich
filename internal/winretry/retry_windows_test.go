package winretry

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Moved from internal/extensions (TestAfterExitWaitsOutAFileStillHeld) with
// the retry it covers.
func TestDoWaitsOutAFileStillHeld(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func(dir string) error
	}{
		{"move", func(dir string) error { return os.Rename(dir, filepath.Join(filepath.Dir(dir), "moved")) }},
		{"delete", os.RemoveAll},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "held")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "app.exe")
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.op(dir); !held(err) {
				f.Close()
				t.Fatalf("%s with a file in it held = %v, want it refused as held", tc.name, err)
			}
			go func() {
				time.Sleep(100 * time.Millisecond)
				f.Close()
			}()
			if err := Do(func() error { return tc.op(dir) }); err != nil {
				t.Fatalf("Do(%s) = %v, want it to wait out the handle", tc.name, err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("%s left the directory in place: Stat = %v", tc.name, err)
			}
		})
	}
}

func TestDoGivesUpOnAnotherError(t *testing.T) {
	other := errors.New("gone")
	calls := 0
	if err := Do(func() error { calls++; return other }); !errors.Is(err, other) || calls != 1 {
		t.Fatalf("Do = %v after %d calls, want the error after 1", err, calls)
	}
}
