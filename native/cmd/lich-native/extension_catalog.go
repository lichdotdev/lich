package main

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/extension"
	exti18n "github.com/lichdotdev/lich/extension/i18n"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// catalogSort is the order of the catalog's grid, the site's three.
type catalogSort int

const (
	sortRecent catalogSort = iota
	sortStars
	sortName
)

func (o catalogSort) label() string {
	switch o {
	case sortStars:
		return i18n.T("settings.extensionsSettings.catalog.sortStars")
	case sortName:
		return i18n.T("settings.extensionsSettings.catalog.sortName")
	}
	return i18n.T("settings.extensionsSettings.catalog.sortRecent")
}

// catalogPlatforms are the builds a listing can carry, in the order the
// detail lists them. The names are the systems' own, left untranslated like a
// provider's.
var catalogPlatforms = []struct{ key, name string }{
	{"linux_amd64", "Linux x64"},
	{"linux_arm64", "Linux ARM"},
	{"darwin_arm64", "macOS Apple Silicon"},
	{"darwin_amd64", "macOS Intel"},
	{"windows_amd64", "Windows x64"},
	{"windows_arm64", "Windows ARM"},
}

// platformName is a build key's name, the key itself for one not listed.
func platformName(key string) string {
	for _, p := range catalogPlatforms {
		if p.key == key {
			return p.name
		}
	}
	return key
}

// systemName is the operating system of a build key, the card's Not for.
func systemName(key string) string {
	os, _, _ := strings.Cut(key, "_")
	switch os {
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	}
	return os
}

// catalogText is a translation the catalog carries, by language tag, in the
// interface language, then English, then the entry's own.
func catalogText[T any](byTag map[string]T, pick func(T) string, own string) string {
	tags := slices.Collect(maps.Keys(byTag))
	for _, want := range []string{string(i18n.Current()), exti18n.Source} {
		if tag := exti18n.Match(want, tags); tag != "" {
			if text := pick(byTag[tag]); text != "" {
				return text
			}
		}
	}
	return own
}

func entryName(e extension.CatalogEntry) string {
	return catalogText(e.Translations, func(t extension.CatalogText) string { return t.Name }, e.Name)
}

func entryDescription(e extension.CatalogEntry) string {
	return catalogText(e.Translations, func(t extension.CatalogText) string { return t.Description }, e.Description)
}

func categoryName(c extension.CatalogCategory) string {
	return catalogText(c.Names, func(name string) string { return name }, c.ID)
}

// entryCategories is the entry's categories by name, in the order it gives.
func entryCategories(cat extension.Catalog, e extension.CatalogEntry) string {
	names := make([]string, 0, len(e.Categories))
	for _, id := range e.Categories {
		if i := slices.IndexFunc(cat.Categories, func(c extension.CatalogCategory) bool { return c.ID == id }); i >= 0 {
			names = append(names, categoryName(cat.Categories[i]))
		}
	}
	return strings.Join(names, ", ")
}

// catalogCount is a category button: the id ("" for All) and how many
// extensions it holds.
type catalogCount struct {
	id, name string
	n        int
}

// catalogCounts is All, then every category holding an extension, in the
// catalog's order: a category nobody listed in would only ever empty the grid.
func catalogCounts(cat extension.Catalog) []catalogCount {
	out := []catalogCount{{name: i18n.T("settings.extensionsSettings.catalog.all"), n: len(cat.Extensions)}}
	for _, c := range cat.Categories {
		n := 0
		for _, e := range cat.Extensions {
			if slices.Contains(e.Categories, c.ID) {
				n++
			}
		}
		if n > 0 {
			out = append(out, catalogCount{id: c.ID, name: categoryName(c), n: n})
		}
	}
	return out
}

// catalogFilter is what narrows and orders the grid.
type catalogFilter struct {
	category, query string
	sort            catalogSort
}

// entries is the catalog's extensions in category, matching the query in
// name, description, publisher or id, in the chosen order.
func (f catalogFilter) entries(cat extension.Catalog) []extension.CatalogEntry {
	query := strings.ToLower(strings.TrimSpace(f.query))
	var out []extension.CatalogEntry
	for _, e := range cat.Extensions {
		if f.category != "" && !slices.Contains(e.Categories, f.category) {
			continue
		}
		if query != "" && !entryMatches(e, query) {
			continue
		}
		out = append(out, e)
	}
	slices.SortStableFunc(out, f.compare)
	return out
}

func entryMatches(e extension.CatalogEntry, query string) bool {
	for _, field := range []string{entryName(e), entryDescription(e), e.Publisher, e.ID} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func (f catalogFilter) compare(a, b extension.CatalogEntry) int {
	byName := strings.Compare(strings.ToLower(entryName(a)), strings.ToLower(entryName(b)))
	switch f.sort {
	case sortStars:
		return cmp.Or(cmp.Compare(b.Stars, a.Stars), byName)
	case sortName:
		return byName
	}
	return cmp.Or(strings.Compare(b.Released, a.Released), byName)
}

// releasedDate is an entry's release day, as the history graph writes one.
func releasedDate(released string) string {
	t, err := time.Parse(time.RFC3339, released)
	if err != nil {
		return released
	}
	return t.Local().Format("Jan 2, 2006")
}

// installedAs is the installed extension with the entry's id.
func installedAs(list []backend.Extension, id string) (backend.Extension, bool) {
	i := slices.IndexFunc(list, func(x backend.Extension) bool { return x.ID == id })
	if i < 0 {
		return backend.Extension{}, false
	}
	return list[i], true
}
