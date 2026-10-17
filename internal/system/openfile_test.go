package system

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpenFileHandsTheFileToTheDefaultOpener proves the file reaches the
// platform's opener as its own last argument, and that $EDITOR is never
// consulted: a terminal EDITOR would come back as a command, a GUI one would
// be launched by name.
func TestOpenFileHandsTheFileToTheDefaultOpener(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shot.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	var name string
	var args []string
	s := &Service{env: []string{"EDITOR=zed"}, run: captureRun(&name, &args)}

	if err := s.OpenFile(dir, "shot.png"); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(dir, "shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if name == "zed" || len(args) == 0 || args[len(args)-1] != want {
		t.Errorf("launched %q %v, want the default opener with %s last", name, args, want)
	}
}

func TestOpenFileRefusesWhatIsNotACheckoutFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	launched := false
	s := &Service{run: func(string, ...string) error {
		launched = true
		return nil
	}}
	for _, rel := range []string{"../escape", "/etc/passwd", "missing.png", "sub"} {
		if err := s.OpenFile(dir, rel); err == nil {
			t.Errorf("rel %q: want an error", rel)
		}
	}
	if launched {
		t.Error("a refused path reached the launcher")
	}
}
