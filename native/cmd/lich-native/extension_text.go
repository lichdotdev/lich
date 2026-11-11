package main

import (
	"maps"
	"slices"

	exti18n "github.com/lichdotdev/lich/extension/i18n"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// extensionName is e's name in the interface language: its catalog's, then
// its English catalog's, then the manifest's.
func extensionName(e backend.Extension) string {
	return extensionText(e, func(t backend.ExtensionText) string { return t.Name }, e.Name)
}

// surfaceTitle is surface s's title the way extensionName reads the name.
func surfaceTitle(e backend.Extension, s backend.ExtensionSurface) string {
	return extensionText(e, func(t backend.ExtensionText) string { return t.Surfaces[s.ID] }, s.Title)
}

func extensionText(e backend.Extension, pick func(backend.ExtensionText) string, manifest string) string {
	tags := slices.Collect(maps.Keys(e.Text))
	for _, want := range []string{string(i18n.Current()), exti18n.Source} {
		if text := pick(e.Text[exti18n.Match(want, tags)]); text != "" {
			return text
		}
	}
	return manifest
}
