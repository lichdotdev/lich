package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func TestDirIsSeparateForADevInstance(t *testing.T) {
	t.Setenv("LICH_DEV", "")
	if got, want := Dir("/cfg"), filepath.Join("/cfg", "lich", "extensions"); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
	t.Setenv("LICH_DEV", "1")
	if got, want := Dir("/cfg"), filepath.Join("/cfg", "lich", "extensions-dev"); got != want {
		t.Errorf("under LICH_DEV Dir = %q, want %q", got, want)
	}
}

func TestDiscoverWithNoDirectoryIsEmpty(t *testing.T) {
	if got := discover(filepath.Join(t.TempDir(), "absent"), "1.0.0"); len(got) != 0 {
		t.Errorf("discover = %v, want nothing", got)
	}
}

func TestDiscoverIgnoresAStrayFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes.txt"), "hi")
	if got := discover(dir, "1.0.0"); len(got) != 0 {
		t.Errorf("discover = %v, want nothing", got)
	}
}

func TestValidationFailures(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*extension.Manifest)
		want   string
	}{
		{"format zero", func(m *extension.Manifest) { m.FormatVersion = 0 }, "positive"},
		{"format too new", func(m *extension.Manifest) { m.FormatVersion = extension.ManifestFormat + 1 }, "update lich"},
		{"id unsafe", func(m *extension.Manifest) { m.ID = "../up" }, "lowercase"},
		{"id empty", func(m *extension.Manifest) { m.ID = "" }, "lowercase"},
		{"id uppercase", func(m *extension.Manifest) { m.ID = "Ext" }, "lowercase"},
		{"id too long", func(m *extension.Manifest) { m.ID = strings.Repeat("a", idMaxLength+1) }, "lowercase"},
		{"id not its directory", func(m *extension.Manifest) { m.ID = "other" }, "must equal"},
		{"needs newer lich", func(m *extension.Manifest) { m.MinLichVersion = "9.0.0" }, "update lich"},
		{"malformed minimum", func(m *extension.Manifest) { m.MinLichVersion = "soon" }, "MAJOR.MINOR.PATCH"},
		{"no executable", func(m *extension.Manifest) { m.Executable = "" }, "required"},
		{"executable escapes", func(m *extension.Manifest) { m.Executable = "../ext" }, "must stay inside"},
		{"executable absolute", func(m *extension.Manifest) { m.Executable = filepath.Join(t.TempDir(), "ext") }, "must stay inside"},
		{"executable missing", func(m *extension.Manifest) { m.Executable = "nope" }, "nope"},
		{"executable is the directory", func(m *extension.Manifest) { m.Executable = "." }, "regular file"},
		{"surface without id", func(m *extension.Manifest) { m.Surfaces = []extension.Surface{{Title: "x"}} }, "needs an id"},
		{"surface twice", func(m *extension.Manifest) { m.Surfaces = []extension.Surface{{ID: "a"}, {ID: "a"}} }, "twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			root := install(t, dir, "ext", modeEcho, c.mutate)
			found := discover(dir, "1.2.3")
			if len(found) != 1 || found[0].invalid == nil {
				t.Fatalf("discover = %+v, want one invalid extension", found)
			}
			if !strings.Contains(found[0].errText, c.want) || found[0].status != api.ExtensionInvalid {
				t.Errorf("status %q, error %q, want invalid naming %q", found[0].status, found[0].errText, c.want)
			}
			if runCount(root) != 0 {
				t.Error("an invalid extension ran")
			}
		})
	}
}

func TestManifestFileFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "absent"), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := install(t, dir, "broken", modeEcho)
	writeFile(t, filepath.Join(broken, extension.ManifestFile), "{not json")

	found := discover(dir, "1.2.3")
	if len(found) != 2 {
		t.Fatalf("discover = %d extensions, want 2", len(found))
	}
	if !strings.Contains(found[0].errText, "read manifest.json") {
		t.Errorf("missing manifest: %q", found[0].errText)
	}
	if !strings.Contains(found[1].errText, "parse manifest.json") {
		t.Errorf("bad manifest: %q", found[1].errText)
	}
}

// A development build has no version to compare, the way themes treat one.
func TestDevBuildPassesAnyMinimum(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeEcho, func(m *extension.Manifest) { m.MinLichVersion = "9.0.0" })
	if found := discover(dir, "dev"); found[0].invalid != nil {
		t.Errorf("rejected under a dev build: %v", found[0].invalid)
	}
}

func TestBadExtensionDoesNotStopTheOthers(t *testing.T) {
	dir := t.TempDir()
	bad := install(t, dir, "bad", modeEcho, func(m *extension.Manifest) { m.Name = "Broken one"; m.FormatVersion = 99 })
	good := install(t, dir, "good", modeEcho)
	s, _ := boot(t, dir)

	waitUntil(t, "the good extension to run", func() bool { return runCount(good) == 1 })
	list, err := s.List(context.Background(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "bad" || list[1].ID != "good" {
		t.Fatalf("list = %+v", list)
	}
	if list[0].Status != api.ExtensionInvalid || list[0].Name != "Broken one" || !strings.Contains(list[0].Error, "update lich") {
		t.Errorf("invalid entry = %+v", list[0])
	}
	if len(list[1].Surfaces) != 1 || list[1].Surfaces[0] != (api.ExtensionSurface{ID: "main", Title: "Main", Icon: "box"}) {
		t.Errorf("surfaces = %+v", list[1].Surfaces)
	}
	if runCount(bad) != 0 {
		t.Error("the invalid extension ran")
	}
}

func TestListWithNoExtensionIsAnEmptyArray(t *testing.T) {
	s, _ := boot(t, filepath.Join(t.TempDir(), "absent"))
	list, err := s.List(context.Background(), api.None{})
	if err != nil || list == nil || len(list) != 0 {
		t.Errorf("list = %#v, %v; want empty and non-nil", list, err)
	}
}
