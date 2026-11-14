package extensions

import (
	"archive/tar"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/extension"
)

// checkTemp points the temporary directory Check works in at a fresh one, so
// a test can see it left nothing there.
func checkTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, tmp)
	}
	return tmp
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Check left %v behind", entries)
	}
}

func writeArchive(t *testing.T, name string, entries []tarEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, tarball(t, entries), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func manifestEntries(t *testing.T, m extension.Manifest) []tarEntry {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return []tarEntry{{name: extension.ManifestFile, body: raw}, {name: testExecutable(), body: []byte("bin")}}
}

func TestCheckPassesAFolderAndLeavesNothing(t *testing.T) {
	tmp := checkTemp(t)
	root := install(t, t.TempDir(), "hello", modeEcho, func(m *extension.Manifest) { m.Description = "Says hello" })
	m, err := Check(root)
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}
	if m.ID != "hello" || m.Description != "Says hello" {
		t.Errorf("manifest = %+v, want hello with its description", m)
	}
	if _, err := os.Stat(filepath.Join(root, sourceFile)); err == nil {
		t.Error("Check wrote into the folder it checked")
	}
	assertEmpty(t, tmp)
}

func TestCheckPassesAnArchiveForAnotherPlatform(t *testing.T) {
	tmp := checkTemp(t)
	m := validManifest("my_ext", testExecutable())
	m.Description = "Does a thing"
	got, err := Check(writeArchive(t, "my_ext_plan9_arm.tar.gz", manifestEntries(t, m)))
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}
	if got.ID != "my_ext" {
		t.Errorf("manifest id = %q, want my_ext", got.ID)
	}
	assertEmpty(t, tmp)
}

func TestCheckFailsNamingWhatIsWrong(t *testing.T) {
	valid := validManifest("fake", testExecutable())
	twoLines := valid
	twoLines.Description = "one\ntwo"
	noIcon := valid
	noIcon.Surfaces = []extension.Surface{{ID: "main", Title: "Main", Icon: "box", IconFile: "logo.png"}}
	cases := []struct {
		name    string
		archive string
		entries []tarEntry
		want    string
	}{
		{"not named for a platform", "fake.tar.gz", manifestEntries(t, valid), "is not named"},
		{"not an id", "Fake_linux_amd64.tar.gz", manifestEntries(t, valid), "valid extension id"},
		{"not gzip", "fake_linux_amd64.tar.gz", nil, "gzip"},
		{"a link", "fake_linux_amd64.tar.gz", append(manifestEntries(t, valid), tarEntry{name: "l", typ: tar.TypeSymlink, link: "/etc/passwd"}), "links are not accepted"},
		{"another id", "other_linux_amd64.tar.gz", manifestEntries(t, valid), "differs from the release asset"},
		{"a bad description", "fake_linux_amd64.tar.gz", manifestEntries(t, twoLines), "one line"},
		{"no executable", "fake_linux_amd64.tar.gz", manifestEntries(t, valid)[:1], "executable"},
		{"an iconFile missing", "fake_linux_amd64.tar.gz", manifestEntries(t, noIcon), "logo.png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmp := checkTemp(t)
			path := filepath.Join(t.TempDir(), c.archive)
			body := []byte("plain text")
			if c.entries != nil {
				body = tarball(t, c.entries)
			}
			writeFile(t, path, string(body))
			if _, err := Check(path); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Check() = %v, want an error naming %q", err, c.want)
			}
			assertEmpty(t, tmp)
		})
	}
}

func TestCheckFailsABadLocaleCatalog(t *testing.T) {
	root := install(t, t.TempDir(), "hello", modeEcho)
	if err := os.MkdirAll(filepath.Join(root, "locales", "en"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "locales", "en", "extension.json"), "{")
	if _, err := Check(root); err == nil || !strings.Contains(err.Error(), "locales") {
		t.Errorf("Check() = %v, want an error naming the locales", err)
	}
}

func TestCheckRefusesALinkAnywhere(t *testing.T) {
	root := install(t, t.TempDir(), "hello", modeEcho)
	link := filepath.Join(t.TempDir(), "hello")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if _, err := Check(link); err == nil || !strings.Contains(err.Error(), "links are not accepted") {
		t.Errorf("Check(link) = %v, want links refused", err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "inner")); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(root); err == nil || !strings.Contains(err.Error(), "links are not accepted") {
		t.Errorf("Check(folder with a link) = %v, want links refused", err)
	}
}

func TestCheckFailsWhatIsNotThere(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("Check(absent) passed")
	}
	if _, err := Check(t.TempDir()); err == nil || !strings.Contains(err.Error(), extension.ManifestFile) {
		t.Errorf("Check(empty folder) = %v, want it to name %s", err, extension.ManifestFile)
	}
}
