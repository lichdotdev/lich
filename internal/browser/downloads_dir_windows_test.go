package browser

import (
	"path/filepath"
	"testing"
)

func TestDownloadDirIsAbsolute(t *testing.T) {
	got, err := downloadDir()
	if err != nil {
		t.Fatalf("downloadDir() error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("downloadDir() = %q, want an absolute path", got)
	}
}
