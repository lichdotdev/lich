package browser

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestDownloadsNameTheFileWithoutOverwriting(t *testing.T) {
	d := testDownloadsAt(t)
	dir := d.dir
	if err := os.WriteFile(filepath.Join(dir, "report.csv"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := d.begin("ga", "report.csv")
	b := d.begin("gb", "report.csv")
	if a.Path != filepath.Join(dir, "report (1).csv") || b.Path != filepath.Join(dir, "report (2).csv") {
		t.Fatalf("reserved %q and %q", a.Path, b.Path)
	}
	if err := os.WriteFile(filepath.Join(d.staging, "ga"), []byte("new"), 0o600); err != nil {
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
	if _, err := os.Lstat(filepath.Join(d.staging, "ga")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the staged file is still there (%v)", err)
	}
}

func TestDownloadsRenameAroundAFileThatTookTheName(t *testing.T) {
	d := testDownloadsAt(t)
	dir := d.dir
	want := d.begin("g", "a.txt").Path
	for _, path := range []string{filepath.Join(dir, "a.txt"), filepath.Join(d.staging, "g")} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := d.progress("g", 1, api.DownloadCompleted)
	if err != nil || got.Path != filepath.Join(dir, "a (1).txt") {
		t.Fatalf("got %q (err %v) instead of a free name beside %q", got.Path, err, want)
	}
}

func TestDownloadsCanceledFreesTheNameAndUnknownIsIgnored(t *testing.T) {
	d := testDownloadsAt(t)
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
	d := testDownloadsAt(t)
	d.begin("g", "a.txt")
	got, ok, err := d.progress("g", 1, api.DownloadCompleted)
	if err == nil || !ok || got.Path != filepath.Join(d.staging, "g") {
		t.Fatalf("got %+v ok=%v err=%v, want the GUID file and the error", got, ok, err)
	}
}

func TestDownloadsCopyAcrossFilesystems(t *testing.T) {
	d := testDownloadsAt(t)
	d.rename = func(from, to string) error {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: errCrossDevice}
	}
	want := d.begin("g", "a.txt").Path
	staged := filepath.Join(d.staging, "g")
	if err := os.WriteFile(staged, []byte("data"), 0o640); err != nil {
		t.Fatal(err)
	}
	got, _, err := d.progress("g", 4, api.DownloadCompleted)
	if err != nil || got.Path != want {
		t.Fatalf("got %+v (err %v), want %s", got, err, want)
	}
	if b, err := os.ReadFile(want); err != nil || string(b) != "data" {
		t.Fatalf("%s holds %q (%v)", want, b, err)
	}
	if _, err := os.Lstat(staged); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the staged file is still there (%v)", err)
	}
	if names := dirNames(t, d.dir); len(names) != 1 {
		t.Fatalf("Downloads holds %v, want only the download", names)
	}
}

func TestDownloadsLeaveWhatNoPageAnnouncedInStaging(t *testing.T) {
	d := testDownloadsAt(t)
	stray := filepath.Join(d.staging, "stray")
	if err := os.WriteFile(stray, []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := d.progress("stray", 5, api.DownloadCompleted); ok || err != nil {
		t.Fatalf("a file no page announced was handled (ok=%v err=%v)", ok, err)
	}
	if names := dirNames(t, d.dir); len(names) != 0 {
		t.Fatalf("Downloads holds %v, want nothing", names)
	}
	if _, err := os.Lstat(stray); err != nil {
		t.Fatalf("the stray file left staging: %v", err)
	}
}

// testDownloadsAt is a downloads staging in one temporary folder and saving
// into another.
func testDownloadsAt(t *testing.T) *downloads {
	return newDownloads(t.TempDir(), t.TempDir())
}

func dirNames(t *testing.T, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
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
