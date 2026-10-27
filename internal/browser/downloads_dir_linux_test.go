//go:build linux

package browser

import (
	"os"
	"testing"
)

func TestUserDirsDownload(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"# comment\nXDG_DESKTOP_DIR=\"$HOME/Desktop\"\nXDG_DOWNLOAD_DIR=\"$HOME/Transferências\"\n", "/home/u/Transferências"},
		{"XDG_DOWNLOAD_DIR=\"/data/dl\"", "/data/dl"},
		{"XDG_DOWNLOAD_DIR=\"relative\"", ""},
		{"XDG_MUSIC_DIR=\"$HOME/Music\"", ""},
	} {
		if got := userDirsDownload(tc.text, "/home/u"); got != tc.want {
			t.Errorf("userDirsDownload(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestDownloadDirReadsUserDirs(t *testing.T) {
	home, conf := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DOWNLOAD_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", conf)
	got, err := downloadDir()
	if err != nil || got != home+"/Downloads" {
		t.Fatalf("without user-dirs.dirs: %q, %v; want ~/Downloads", got, err)
	}
	if err := os.WriteFile(conf+"/user-dirs.dirs", []byte("XDG_DOWNLOAD_DIR=\"$HOME/Baixados\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := downloadDir(); got != home+"/Baixados" {
		t.Fatalf("got %q, want the folder user-dirs.dirs names", got)
	}
	t.Setenv("XDG_DOWNLOAD_DIR", "/env/dl")
	if got, _ := downloadDir(); got != "/env/dl" {
		t.Fatalf("got %q, want the environment's", got)
	}
}
