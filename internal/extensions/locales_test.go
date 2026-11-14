package extensions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func writeCatalog(t *testing.T, root, tag, content string) {
	t.Helper()
	dir := filepath.Join(root, "locales", tag)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "extension.json"), content)
}

func TestTheManifestsStringsComeFromEachCatalog(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, testID, modeEcho)
	writeCatalog(t, root, "en", `{"manifest": {"name": "Fake", "surfaces": {"main": {"title": "Main"}}}}`)
	writeCatalog(t, root, "es", `{"manifest": {"description": "Una de prueba"}}`)
	writeCatalog(t, root, "pt-br", `{"manifest": {"name": "Falso", "surfaces": {"main": {"title": "Principal"}, "gone": {"title": "x"}}}}`)
	writeCatalog(t, root, "ja", `{"manifest": {"surfaces": {"main": {"title": "メイン"}}}, "extra": "y"}`)
	writeCatalog(t, root, "ko", `{"manifest": {"name": {"one": "a", "other": "b"}}}`)
	x := load(root, testID, "1.2.3")
	if x.invalid != nil {
		t.Fatalf("invalid: %v", x.invalid)
	}
	text := x.view().Text
	if got := text["pt-br"]; got.Name != "Falso" || len(got.Surfaces) != 1 || got.Surfaces["main"] != "Principal" {
		t.Errorf("pt-br = %+v", got)
	}
	if got := text["ja"]; got.Name != "" || got.Surfaces["main"] != "メイン" {
		t.Errorf("ja = %+v, want only its own title", got)
	}
	if got := text["es"]; got.Description != "Una de prueba" || got.Name != "" {
		t.Errorf("es = %+v, want only its own description", got)
	}
	if _, ok := text["ko"]; ok || text["en"].Name != "Fake" {
		t.Errorf("text = %+v, want no ko (a plural name) and the English name", text)
	}
}

func TestNoLocalesFolderIsNoTranslation(t *testing.T) {
	x := load(install(t, t.TempDir(), testID, modeEcho), testID, "1.2.3")
	if x.invalid != nil || x.view().Text != nil {
		t.Fatalf("invalid %v, text %v", x.invalid, x.view().Text)
	}
}

func TestABadCatalogMakesTheExtensionInvalid(t *testing.T) {
	cases := map[string]struct {
		lay  func(t *testing.T, root string)
		want string
	}{
		"not json": {func(t *testing.T, root string) { writeCatalog(t, root, "en", `{"manifest": `) }, "extension.json"},
		"a number": {func(t *testing.T, root string) { writeCatalog(t, root, "en", `{"n": 1}`) }, "want a string"},
		"locales is a file": {func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, "locales"), "x")
		}, "must be a folder"},
		"too many locales": {func(t *testing.T, root string) {
			for i := range maxLocales + 1 {
				writeCatalog(t, root, fmt.Sprintf("x%d", i), `{}`)
			}
		}, "more than"},
		"too big": {func(t *testing.T, root string) {
			writeCatalog(t, root, "en", `{"a": "`+strings.Repeat("x", maxCatalogsBytes)+`"}`)
		}, "MiB"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := install(t, t.TempDir(), testID, modeEcho)
			c.lay(t, root)
			x := load(root, testID, "1.2.3")
			if x.invalid == nil || x.status != statusInvalid || !strings.Contains(x.errText, c.want) {
				t.Fatalf("status %s, error %v, want invalid with %q", x.status, x.invalid, c.want)
			}
			if x.view().Name != "Fake "+testID {
				t.Errorf("an invalid extension lost its manifest name: %q", x.view().Name)
			}
		})
	}
}

func TestInstallRefusesABadCatalog(t *testing.T) {
	from := install(t, t.TempDir(), testID, modeEcho)
	writeCatalog(t, from, "en", `not json`)
	s, _ := boot(t, filepath.Join(t.TempDir(), "extensions"))
	_, err := s.PrepareFromDir(context.Background(), api.ExtensionDirInstallOptions{Dir: from})
	if err == nil || !strings.Contains(err.Error(), "locales are not valid") {
		t.Fatalf("PrepareFromDir() = %v, want the catalog refused", err)
	}
}

func TestAnExtensionStartsWithTheInterfaceLanguage(t *testing.T) {
	for _, c := range []struct {
		language func() string
		want     string
	}{{nil, "language en"}, {func() string { return "pt-BR" }, "language pt-BR"}} {
		dir := t.TempDir()
		install(t, dir, testID, modeLanguage)
		r := &recorder{}
		s := New(dir, filepath.Join(t.TempDir(), "data"), "1.2.3", append(os.Environ(), fakeEnv+"=1"), r.emit)
		s.timing.stopGrace, s.timing.killWait = 5*time.Second, 10*time.Second
		s.SetLanguage(c.language)
		t.Cleanup(s.Shutdown)
		s.Boot()
		waitUntil(t, c.want, func() bool { return slices.Contains(r.titles(t), c.want) })
	}
}

func TestSendDeliversTheLanguage(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, testID, modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, statusRunning)
	if _, err := s.Send(context.Background(), sendOptions(t, testID, extension.Language.New(extension.LanguageData{Tag: "ja"}))); err != nil {
		t.Fatalf("Send(language) = %v", err)
	}
	waitUntil(t, "the language delivered", func() bool { return slices.Contains(r.titles(t), "got language") })
}
