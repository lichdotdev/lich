package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestADownloadTakesTheNextFreeNameAsChromeDoes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "taken.txt"), nil, 0o644)
	for _, tc := range []struct{ suggested, want string }{
		{"report.pdf", "report.pdf"},
		{"report.pdf", "report (1).pdf"},
		{"report.pdf", "report (2).pdf"},
		{"taken.txt", "taken (1).txt"},
		{"README", "README"},
		{"README", "README (1)"},
		{"../../etc/passwd", "passwd"},
		{`..\..\evil.exe`, "evil.exe"},
		{"", "download"},
	} {
		path, err := reserve(dir, tc.suggested)
		if err != nil {
			t.Fatalf("reserve %q: %v", tc.suggested, err)
		}
		if path != filepath.Join(dir, tc.want) {
			t.Errorf("reserve %q = %s, want %s in the directory", tc.suggested, path, tc.want)
		}
	}
}
