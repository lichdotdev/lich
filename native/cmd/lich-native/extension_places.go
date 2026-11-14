package main

import (
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// placedSurface is a dock or panel surface of an extension that can run,
// with what its tab, footer item and panel header show.
type placedSurface struct {
	key   extensionKey
	place extension.Place
	// title is the surface's, name its extension's, both translated.
	title, name string
	glyph       icons.Glyph
}

// label is the footer item's tooltip and the editor's chip: the surface,
// then the extension it comes from.
func (p placedSurface) label() string { return p.title + " · " + p.name }

// placedSurfaces is every dock and panel surface, in the list's order. An
// invalid extension never started, so it has none.
func placedSurfaces(list []backend.Extension, images map[extensionKey]*icons.Bitmap) []placedSurface {
	var out []placedSurface
	for _, e := range list {
		if e.Status == api.ExtensionInvalid {
			continue
		}
		for _, s := range e.Surfaces {
			place := extension.Place(s.Place)
			if place == extension.PlaceScreen {
				continue
			}
			k := extensionKey{e.ID, s.ID}
			var glyph icons.Glyph = surfaceGlyph(s.Icon)
			if img := images[k]; img != nil {
				glyph = img
			}
			out = append(out, placedSurface{key: k, place: place, title: surfaceTitle(e, s), name: extensionName(e), glyph: glyph})
		}
	}
	return out
}

// placedIn is the surfaces of list that live in place.
func placedIn(list []placedSurface, place extension.Place) []placedSurface {
	var out []placedSurface
	for _, p := range list {
		if p.place == place {
			out = append(out, p)
		}
	}
	return out
}

// findPlaced is surface k among list.
func findPlaced(list []placedSurface, k extensionKey) (placedSurface, bool) {
	for _, p := range list {
		if p.key == k {
			return p, true
		}
	}
	return placedSurface{}, false
}

var surfaceGlyphs = struct {
	sync.Mutex
	parsed map[string]*icons.Icon
}{parsed: map[string]*icons.Icon{}}

// surfaceGlyph is the Lucide glyph a surface names, the puzzle for none or one
// the set lacks, parsed once per name.
func surfaceGlyph(name string) *icons.Icon {
	surfaceGlyphs.Lock()
	defer surfaceGlyphs.Unlock()
	ic, ok := surfaceGlyphs.parsed[name]
	if !ok {
		ic, ok = icons.LookupLucide(name)
		if !ok {
			ic = icExtension
		}
		surfaceGlyphs.parsed[name] = ic
	}
	return ic
}

// extensionItemPrefix starts the footer item id of a placed surface,
// "ext:<extension>/<surface>". An extension id has no slash, so the first one
// ends it.
const extensionItemPrefix = "ext:"

func extensionItemID(k extensionKey) string { return extensionItemPrefix + k.ext + "/" + k.surface }

// extensionItemKey is the surface a footer item id names, false for a
// built-in item.
func extensionItemKey(id string) (extensionKey, bool) {
	rest, ok := strings.CutPrefix(id, extensionItemPrefix)
	if !ok {
		return extensionKey{}, false
	}
	ext, surface, ok := strings.Cut(rest, "/")
	return extensionKey{ext, surface}, ok && ext != "" && surface != ""
}

// activeContext is what the placed surfaces follow: the active session in its
// checkout, else the project's own folder.
func activeContext(v stageView) extension.ContextData {
	if v.home {
		return extension.ContextData{}
	}
	c := extension.ContextData{Session: v.in.sessionID, Project: v.project, Path: v.in.checkout}
	if c.Session == "" {
		c.Path = v.in.path
	}
	return c
}
