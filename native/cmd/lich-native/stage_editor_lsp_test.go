package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lichdotdev/godemirror"
)

func TestApplyTextEditsMakesEveryEditInTheTextAsItWas(t *testing.T) {
	text := "one\ntwo\nthree\n"
	got := applyTextEdits(text, []godemirror.TextEdit{
		{Start: godemirror.LineColumn{Line: 0, Column: 0}, End: godemirror.LineColumn{Line: 0, Column: 3}, Text: "ONE"},
		{Start: godemirror.LineColumn{Line: 2, Column: 0}, End: godemirror.LineColumn{Line: 2, Column: 5}, Text: "3"},
		{Start: godemirror.LineColumn{Line: 1, Column: 9}, End: godemirror.LineColumn{Line: 1, Column: 9}, Text: "!"},
	})
	if want := "ONE\ntwo!\n3\n"; got != want {
		t.Fatalf("edited text %q, want %q", got, want)
	}
}

func TestLspRootIsTheNearestFolderWithAMarker(t *testing.T) {
	checkout := t.TempDir()
	for _, p := range []string{"go.mod", "native/go.mod", "native/cmd/x/main.go"} {
		path := filepath.Join(checkout, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	markers := [][]string{{"go.work"}, {"go.mod"}}
	if got := lspRoot(checkout, "native/cmd/x/main.go", markers); got != filepath.Join(checkout, "native") {
		t.Fatalf("root %q, want the nested module's", got)
	}
	if got := lspRoot(checkout, "cmd/y/main.go", markers); got != checkout {
		t.Fatalf("root %q, want the checkout's", got)
	}
}

func TestTheEditorShowsItsLanguageServersDiagnostics(t *testing.T) {
	gopls, err := exec.LookPath("gopls")
	dir := t.TempDir()
	if err == nil {
		version := exec.Command(gopls, "version")
		version.Dir = dir
		err = version.Run()
	}
	if err != nil {
		t.Skip("no gopls that runs here on the PATH")
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	text := "package main\n\nfunc main() { undefinedThing() }\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, text)
	f.checkout, f.rel = dir, "main.go"
	f.env.lsp = newLspHost(func() {})
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		f.syncLSP()
		layOut(t, f)
		if errs, _ := f.lspCounts(); errs > 0 {
			if text, _, _ := f.lspText(); text != "gopls" {
				t.Fatalf("header names %q, want gopls", text)
			}
			return
		}
	}
	t.Fatalf("gopls never reported the undefined call; header says %q", func() string { s, _, _ := f.lspText(); return s }())
}
