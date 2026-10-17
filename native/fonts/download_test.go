package fonts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gioui.org/font"
)

// served is a server handing out Onest's regular and bold as the files to
// download, counting the requests it answers.
func served(t *testing.T) ([]RemoteFile, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	body := map[string][]byte{}
	for _, name := range []string{"Onest-Regular.ttf", "Onest-Bold.ttf"} {
		data, err := onest.ReadFile("onest/" + name)
		if err != nil {
			t.Fatal(err)
		}
		body["/"+name] = data
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		data, ok := body[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	file := func(name string, weight font.Weight) RemoteFile {
		data := body["/"+name]
		sum := sha256.Sum256(data)
		return RemoteFile{Name: name, URL: srv.URL + "/" + name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Weight: weight}
	}
	return []RemoteFile{file("Onest-Regular.ttf", font.Normal), file("Onest-Bold.ttf", font.Bold)}, &hits
}

func noProgress(int64, int64) {}

func TestFetchStoresTheFilesAndLoadDownloadedReadsThem(t *testing.T) {
	files, _ := served(t)
	dir := t.TempDir()
	var last, total int64
	if err := Fetch(context.Background(), files, dir, func(d, tot int64) { last, total = d, tot }); err != nil {
		t.Fatal(err)
	}
	if want := TotalSize(files); last != want || total != want {
		t.Errorf("progress ended at %d of %d, want %d of %d", last, total, want, want)
	}
	faces, err := LoadDownloaded(files, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != len(interfaceWeights) || faces[0].Font != (font.Font{Typeface: Han, Weight: font.Normal}) || faces[1].Font.Weight != font.Bold {
		t.Errorf("loaded %+v", faces)
	}
}

func TestCoverWeightsLendsTheNearestFaceTheHeavierOnATie(t *testing.T) {
	files, _ := served(t)
	dir := t.TempDir()
	if err := Fetch(context.Background(), files, dir, noProgress); err != nil {
		t.Fatal(err)
	}
	faces, err := LoadDownloaded(files, dir)
	if err != nil {
		t.Fatal(err)
	}
	from := map[font.Weight]font.Face{}
	for _, f := range faces {
		from[f.Font.Weight] = f.Face
	}
	if from[font.Medium] != from[font.Normal] || from[font.SemiBold] != from[font.Bold] {
		t.Errorf("medium lent by normal %v, semibold by bold %v", from[font.Medium] == from[font.Normal], from[font.SemiBold] == from[font.Bold])
	}
}

func TestFetchReusesTheFilesAlreadyDownloaded(t *testing.T) {
	files, hits := served(t)
	dir := t.TempDir()
	if err := Fetch(context.Background(), files, dir, noProgress); err != nil {
		t.Fatal(err)
	}
	before := hits.Load()
	if err := Fetch(context.Background(), files, dir, noProgress); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != before {
		t.Errorf("the second fetch made %d requests", hits.Load()-before)
	}
}

func TestFetchRefusesAFileWithTheWrongHash(t *testing.T) {
	files, _ := served(t)
	files[0].SHA256 = strings.Repeat("0", 64)
	dir := t.TempDir()
	err := Fetch(context.Background(), files, dir, noProgress)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("got %v, want a SHA-256 mismatch", err)
	}
	assertEmpty(t, dir)
}

func TestFetchReportsAnHTTPError(t *testing.T) {
	files, _ := served(t)
	files[1].URL += "-missing"
	dir := t.TempDir()
	err := Fetch(context.Background(), files, dir, noProgress)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("got %v, want a 404", err)
	}
	if _, err := LoadDownloaded(files, dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("load after a failed fetch: %v, want not exist", err)
	}
}

func TestFetchStopsWhenCanceled(t *testing.T) {
	files, _ := served(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Fetch(ctx, files, dir, noProgress); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want canceled", err)
	}
	assertEmpty(t, dir)
}

func TestLoadDownloadedRefusesAFileChangedOnDisk(t *testing.T) {
	files, _ := served(t)
	dir := t.TempDir()
	if err := Fetch(context.Background(), files, dir, noProgress); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[0].Name), []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDownloaded(files, dir); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("got %v, want a SHA-256 mismatch", err)
	}
}

func TestNotoSansSCIsPinned(t *testing.T) {
	for _, f := range NotoSansSC {
		if !strings.Contains(f.URL, notoCJKCommit) || len(f.SHA256) != 64 || f.Size == 0 {
			t.Errorf("%s is not pinned: %+v", f.Name, f)
		}
	}
}

func TestNotoSansJPAndKRArePinnedInSCsWeights(t *testing.T) {
	for _, files := range [][]RemoteFile{NotoSansJP, NotoSansKR} {
		if len(files) != len(NotoSansSC) {
			t.Fatalf("%d files, want SC's %d", len(files), len(NotoSansSC))
		}
		for i, f := range files {
			if !strings.Contains(f.URL, notoCJKCommit) || !strings.HasSuffix(f.URL, "/"+f.Name) || len(f.SHA256) != 64 || f.Size == 0 {
				t.Errorf("%s is not pinned: %+v", f.Name, f)
			}
			if f.Weight != NotoSansSC[i].Weight {
				t.Errorf("%s weighs %v, want SC's %v", f.Name, f.Weight, NotoSansSC[i].Weight)
			}
		}
	}
}

func TestEachCJKLanguageDownloadsItsOwnNotoSans(t *testing.T) {
	for _, c := range []struct {
		language, family string
		files            []RemoteFile
	}{
		{"zh-CN", "Noto Sans SC", NotoSansSC},
		{"ja", "Noto Sans JP", NotoSansJP},
		{"ko", "Noto Sans KR", NotoSansKR},
		{"ru", "Noto Sans SC", NotoSansSC},
	} {
		family, files := NotoSansFor(c.language)
		if family != c.family || &files[0] != &c.files[0] {
			t.Errorf("%s downloads %s (%s), want %s", c.language, family, files[0].Name, c.family)
		}
	}
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("left %d files behind, the first %s", len(entries), entries[0].Name())
	}
}
