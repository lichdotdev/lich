package browser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestDownloadsNameTheFileWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "report.csv"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := newDownloads(dir)
	a := d.begin("ga", "report.csv")
	b := d.begin("gb", "report.csv")
	if a.Path != filepath.Join(dir, "report (1).csv") || b.Path != filepath.Join(dir, "report (2).csv") {
		t.Fatalf("reserved %q and %q", a.Path, b.Path)
	}
	if err := os.WriteFile(filepath.Join(dir, "ga"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok, err := d.progress("ga", 3, api.DownloadCompleted)
	if err != nil || !ok || got.Path != a.Path || got.Bytes != 3 {
		t.Fatalf("completed %+v ok=%v err=%v", got, ok, err)
	}
	if b, _ := os.ReadFile(a.Path); string(b) != "new" {
		t.Fatalf("%s holds %q, want the download", a.Path, b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "report.csv")); string(b) != "old" {
		t.Fatal("the file already there was overwritten")
	}
}

func TestDownloadsRenameAroundAFileThatTookTheName(t *testing.T) {
	dir := t.TempDir()
	d := newDownloads(dir)
	want := d.begin("g", "a.txt").Path
	for _, name := range []string{"a.txt", "g"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := d.progress("g", 1, api.DownloadCompleted)
	if err != nil || got.Path != filepath.Join(dir, "a (1).txt") {
		t.Fatalf("got %q (err %v) instead of a free name beside %q", got.Path, err, want)
	}
}

func TestDownloadsCanceledFreesTheNameAndUnknownIsIgnored(t *testing.T) {
	dir := t.TempDir()
	d := newDownloads(dir)
	first := d.begin("g", "x.bin").Path
	if got, ok, _ := d.progress("g", 0, api.DownloadCanceled); !ok || got.State != api.DownloadCanceled {
		t.Fatalf("canceled reported %+v", got)
	}
	if again := d.begin("h", "x.bin").Path; again != first {
		t.Fatalf("a canceled download kept %q reserved; got %q", first, again)
	}
	if _, ok, _ := d.progress("nobody", 1, api.DownloadCompleted); ok {
		t.Fatal("a GUID never begun was reported")
	}
}

func TestDownloadsReportTheWrittenFileWhenTheRenameFails(t *testing.T) {
	d := newDownloads(t.TempDir())
	d.begin("g", "a.txt")
	got, ok, err := d.progress("g", 1, api.DownloadCompleted)
	if err == nil || !ok || got.Path != filepath.Join(d.dir, "g") {
		t.Fatalf("got %+v ok=%v err=%v, want the GUID file and the error", got, ok, err)
	}
}

func TestSafeNameKeepsTheFileInTheFolder(t *testing.T) {
	for in, want := range map[string]string{
		"report.csv":       "report.csv",
		"../../etc/passwd": ".._.._etc_passwd",
		`..\win.ini`:       ".._win.ini",
		"..":               fallbackName,
		"  ":               fallbackName,
		"a\nb":             "a_b",
	} {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}
