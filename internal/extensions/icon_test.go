package extensions

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func withIconFile(name string) func(*extension.Manifest) {
	return func(m *extension.Manifest) { m.Surfaces[0].IconFile = name }
}

func TestSurfaceIconFileReachesTheList(t *testing.T) {
	dir := t.TempDir()
	logo := pngOf(t, 32, 16)
	root := install(t, dir, "ext", modeEcho, withIconFile("assets/logo.png"))
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "assets", "logo.png"), string(logo))
	s, _ := boot(t, dir)

	list, err := s.List(context.Background(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Status == api.ExtensionInvalid {
		t.Fatalf("list = %+v", list)
	}
	if got := list[0].Icons["main"]; list[0].Surfaces[0].Icon != "box" || !bytes.Equal(got, logo) {
		t.Errorf("surface %+v with %d image bytes, want box with the logo's %d", list[0].Surfaces[0], len(got), len(logo))
	}
}

func TestSurfaceWithoutIconFileHasNoImage(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, "ext", modeEcho)
	s, _ := boot(t, dir)
	list, err := s.List(context.Background(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Icons != nil {
		t.Errorf("icons = %v, want none", list[0].Icons)
	}
}

func TestIconFileFailures(t *testing.T) {
	cases := []struct {
		name, file string
		content    []byte
		want       string
	}{
		{"escapes", "../logo.png", nil, "must stay inside"},
		{"absolute", filepath.Join(os.TempDir(), "logo.png"), nil, "must stay inside"},
		{"missing", "nope.png", nil, "nope.png"},
		{"a directory", ".", nil, "regular file"},
		{"not a png", "logo.png", []byte("GIF89a not really"), "not a PNG"},
		{"file too large", "logo.png", append(pngOf(t, 1, 1), make([]byte, maxIconFileBytes)...), "at most"},
		{"too many pixels", "logo.png", pngOf(t, maxIconSide+1, 1), "at most"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			root := install(t, dir, "ext", modeEcho, withIconFile(c.file))
			if c.content != nil {
				writeFile(t, filepath.Join(root, c.file), string(c.content))
			}
			found := discover(dir, "1.2.3")
			if len(found) != 1 || found[0].status != api.ExtensionInvalid {
				t.Fatalf("discover = %+v, want one invalid extension", found)
			}
			if !strings.Contains(found[0].errText, "iconFile") || !strings.Contains(found[0].errText, c.want) {
				t.Errorf("error %q, want one about iconFile naming %q", found[0].errText, c.want)
			}
		})
	}
}

func TestInstallRefusesABadIconFile(t *testing.T) {
	from := install(t, t.TempDir(), testID, modeEcho, withIconFile("logo.png"))
	writeFile(t, filepath.Join(from, "logo.png"), "not a png")
	s, _ := boot(t, filepath.Join(t.TempDir(), "extensions"))
	_, err := s.PrepareFromDir(context.Background(), api.ExtensionDirInstallOptions{Dir: from})
	if err == nil || !strings.Contains(err.Error(), "iconFile") {
		t.Fatalf("PrepareFromDir() = %v, want the icon refused", err)
	}
}
