package extensions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	exti18n "github.com/lichdotdev/lich/extension/i18n"
)

// The bounds of an extension's catalogs, read at every discovery: well past
// any real translation, short of what would make a boot slow.
const (
	maxLocales       = 64
	maxCatalogFiles  = 512
	maxCatalogsBytes = 8 << 20
)

// readLocales checks the extension's locales folder, if it has one, and
// reads the manifest's strings from it. A link anywhere inside is refused:
// a catalog must not be read from outside the extension's folder.
func readLocales(root string, m extension.Manifest) (map[string]api.ExtensionText, error) {
	dir := filepath.Join(root, exti18n.LocalesDir)
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", exti18n.LocalesDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s must be a folder; links are not accepted", exti18n.LocalesDir)
	}
	if err := checkLocales(dir); err != nil {
		return nil, err
	}
	b, err := exti18n.Open(os.DirFS(root))
	if err != nil {
		return nil, err
	}
	return manifestText(b, m), nil
}

// catalogBudget is what checkLocales has counted so far.
type catalogBudget struct {
	files int
	bytes int64
}

func checkLocales(dir string) error {
	tags, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", exti18n.LocalesDir, err)
	}
	if len(tags) > maxLocales {
		return fmt.Errorf("%s holds more than %d entries", exti18n.LocalesDir, maxLocales)
	}
	var budget catalogBudget
	for _, tag := range tags {
		rel := exti18n.LocalesDir + "/" + tag.Name()
		if tag.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a link; links are not accepted", rel)
		}
		if !tag.IsDir() {
			continue
		}
		if err := budget.checkFolder(filepath.Join(dir, tag.Name()), rel); err != nil {
			return err
		}
	}
	return nil
}

// checkFolder counts the catalogs of one locale folder, the only files
// exti18n.Open reads.
func (b *catalogBudget) checkFolder(dir, rel string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}
	for _, e := range entries {
		name := rel + "/" + e.Name()
		if e.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a link; links are not accepted", name)
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		b.files++
		b.bytes += info.Size()
		if b.files > maxCatalogFiles || b.bytes > maxCatalogsBytes {
			return fmt.Errorf("%s holds more than %d catalogs or %d MiB", exti18n.LocalesDir, maxCatalogFiles, maxCatalogsBytes>>20)
		}
	}
	return nil
}

// manifestText is the name, description and surface titles each catalog
// translates, leaving the fallback to English and the manifest to the
// window, which knows the language it shows.
func manifestText(b *exti18n.Bundle, m extension.Manifest) map[string]api.ExtensionText {
	var out map[string]api.ExtensionText
	for _, tag := range b.Tags() {
		t := api.ExtensionText{Name: ownText(b, tag, exti18n.ManifestNameKey), Description: ownText(b, tag, exti18n.ManifestDescriptionKey)}
		for _, s := range m.Surfaces {
			if title := ownText(b, tag, exti18n.SurfaceTitleKey(s.ID)); title != "" {
				if t.Surfaces == nil {
					t.Surfaces = map[string]string{}
				}
				t.Surfaces[s.ID] = title
			}
		}
		if t.Name == "" && t.Description == "" && t.Surfaces == nil {
			continue
		}
		if out == nil {
			out = map[string]api.ExtensionText{}
		}
		out[tag] = t
	}
	return out
}

// ownText is key's plain text in tag's own catalog, "" for a plural or a key
// it lacks.
func ownText(b *exti18n.Bundle, tag, key string) string {
	m, found, ok := b.Lookup(tag, key)
	if !ok || found != tag || m.Plural != nil {
		return ""
	}
	return m.Text
}
