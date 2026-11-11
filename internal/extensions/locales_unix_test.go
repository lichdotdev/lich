//go:build !windows

package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestACatalogLinkedFromOutsideIsRefused(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.json")
	writeFile(t, outside, `{"a": "b"}`)
	for name, link := range map[string]string{"a catalog": "locales/en/extension.json", "a locale": "locales/en", "the folder": "locales"} {
		t.Run(name, func(t *testing.T) {
			root := install(t, t.TempDir(), testID, modeEcho)
			target := outside
			if name != "a catalog" {
				target = filepath.Dir(outside)
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
				t.Fatal(err)
			}
			if x := load(root, testID, "1.2.3"); x.invalid == nil || !strings.Contains(x.errText, "links are not accepted") {
				t.Fatalf("error %v, want the link refused", x.invalid)
			}
		})
	}
}
