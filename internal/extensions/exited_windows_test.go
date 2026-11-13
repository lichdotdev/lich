package extensions

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAfterExitWaitsOutAFileStillHeld(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   func(dir string) error
	}{
		{"move", func(dir string) error { return os.Rename(dir, filepath.Join(filepath.Dir(dir), replacedDir)) }},
		{"delete", os.RemoveAll},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), testID)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			held := filepath.Join(dir, testExecutable())
			writeFile(t, held, "x")
			f, err := os.Open(held)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.op(dir); !stillHeld(err) {
				f.Close()
				t.Fatalf("%s with a file in it held = %v, want it refused as held", tc.name, err)
			}
			go func() {
				time.Sleep(100 * time.Millisecond)
				f.Close()
			}()
			if err := afterExit(func() error { return tc.op(dir) }); err != nil {
				t.Fatalf("afterExit(%s) = %v, want it to wait out the handle", tc.name, err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("%s left the directory in place: Stat = %v", tc.name, err)
			}
		})
	}
}
