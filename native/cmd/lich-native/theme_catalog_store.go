package main

import (
	"context"
	"image/color"
	"log"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// themeCatalogStore is the theme catalog as the backend holds it, read with
// themes.Catalog the first time a screen asks and kept by theme-catalog, each
// listed theme read once into the colors its previews are drawn in.
type themeCatalogStore struct {
	client     *backend.Client
	invalidate func()

	mu      sync.Mutex
	cat     *themeCatalog
	err     string
	reading bool
}

// themeCatalog is the listed packs this window can draw, in the catalog's
// order.
type themeCatalog struct{ packs []themePack }

// themePack is one listed pack and those of its themes whose colors parse.
type themePack struct {
	def    backend.ThemeCatalogPack
	themes []themeLook
}

// themeLook is a listed theme in the colors its previews are drawn in.
type themeLook struct {
	themeEntry
	ink themeInk
}

// themeInk is what a preview paints past the palette: the code's tokens, each
// the theme's syntax color or the foreground for a theme without one.
type themeInk struct {
	keyword, number, str, comment, fn, variable, null color.NRGBA
}

func newThemeCatalogStore(client *backend.Client, invalidate func()) *themeCatalogStore {
	return &themeCatalogStore{client: client, invalidate: invalidate}
}

// read is the catalog, or why there is none: both empty while the read runs.
// The first call starts it.
func (s *themeCatalogStore) read() (*themeCatalog, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil && s.err == "" && !s.reading {
		s.fetch()
	}
	return s.cat, s.err
}

// retry is Try again: one more read while there is no catalog.
func (s *themeCatalogStore) retry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil && !s.reading {
		s.err = ""
		s.fetch()
	}
}

// fetch reads the catalog. A copy theme-catalog brought while it ran is as new
// as the answer or newer, and wins. Called with mu held.
func (s *themeCatalogStore) fetch() {
	s.reading = true
	go func() {
		cat, err := s.client.ThemeCatalog(context.Background())
		s.mu.Lock()
		s.reading = false
		switch {
		case s.cat != nil:
		case err != nil:
			s.err = backend.ErrorText(err)
		default:
			s.cat = readThemeCatalog(cat)
		}
		s.mu.Unlock()
		s.invalidate()
	}()
}

// set takes the copy theme-catalog carries.
func (s *themeCatalogStore) set(cat backend.ThemeCatalog) {
	read := readThemeCatalog(cat)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cat, s.err = read, ""
}

// readThemeCatalog reads every listed theme; one whose colors do not parse is
// left out, and a pack with none left is too.
func readThemeCatalog(cat backend.ThemeCatalog) *themeCatalog {
	out := &themeCatalog{}
	for _, p := range cat.Packs {
		pack := themePack{def: p}
		for _, def := range p.Themes {
			look, err := readLook(def)
			if err != nil {
				log.Printf("theme catalog: %s/%s can't be drawn: %v", p.ID, def.ID, err)
				continue
			}
			pack.themes = append(pack.themes, look)
		}
		if len(pack.themes) > 0 {
			out.packs = append(out.packs, pack)
		}
	}
	return out
}

func readLook(def backend.Theme) (themeLook, error) {
	e, err := readTheme(def)
	if err != nil {
		return themeLook{}, err
	}
	fg := e.palette.Foreground
	at := func(key string) color.NRGBA { return syntaxColor(def.Syntax, key, fg) }
	return themeLook{themeEntry: e, ink: themeInk{
		keyword: at("keyword"), number: at("number"), str: at("string"), comment: at("comment"),
		fn: at("function(variableName)"), variable: at("variableName"), null: at("null"),
	}}, nil
}

// syntaxColor is the color the theme gives key, else fallback.
func syntaxColor(syntax map[string]backend.SyntaxStyle, key string, fallback color.NRGBA) color.NRGBA {
	if c, err := ui.ParseColor(syntax[key].Color); err == nil {
		return c
	}
	return fallback
}

func (c *themeCatalog) pack(id string) (themePack, bool) {
	i := slices.IndexFunc(c.packs, func(p themePack) bool { return p.def.ID == id })
	if i < 0 {
		return themePack{}, false
	}
	return c.packs[i], true
}

// cover is the theme a pack's card is drawn in: its first dark one, else its
// first.
func (p themePack) cover() themeLook {
	if i := slices.IndexFunc(p.themes, func(t themeLook) bool { return !t.light() }); i >= 0 {
		return p.themes[i]
	}
	return p.themes[0]
}

// schemes is whether the pack holds a dark theme and a light one.
func (p themePack) schemes() (dark, light bool) {
	for _, t := range p.themes {
		if t.light() {
			light = true
		} else {
			dark = true
		}
	}
	return dark, light
}

// holds is whether one of the pack's themes has id.
func (p themePack) holds(id string) bool {
	return slices.ContainsFunc(p.themes, func(t themeLook) bool { return t.def.ID == id })
}

// variantName is a theme's name less its pack's, the name itself when that
// leaves nothing: Rosé Pine's Dawn, Moon and Rosé Pine.
func (p themePack) variantName(t themeLook) string {
	if rest := strings.TrimSpace(strings.TrimPrefix(t.def.Name, p.def.Name)); rest != "" && rest != t.def.Name {
		return rest
	}
	return t.def.Name
}

// fromRepo is the pack whose repository a theme was installed from.
func (c *themeCatalog) fromRepo(src *backend.ThemeSource) (themePack, bool) {
	if c == nil || src == nil {
		return themePack{}, false
	}
	i := slices.IndexFunc(c.packs, func(p themePack) bool { return repoLabel(p.def.Repo) == repoLabel(src.URL) })
	if i < 0 {
		return themePack{}, false
	}
	return c.packs[i], true
}

// olderRelease is whether lich, a MAJOR.MINOR.PATCH release, is older than
// minimum. A build that is no release (a dev build, a pre-release) is never
// older, as the backend's own check has it (internal/themes, checkMinLichVersion).
func olderRelease(lich, minimum string) bool {
	have, ok := releaseParts(lich)
	want, okWant := releaseParts(minimum)
	if !ok || !okWant {
		return false
	}
	return slices.Compare(have, want) < 0
}

func releaseParts(v string) ([]int, bool) {
	fields := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(fields) != 3 {
		return nil, false
	}
	parts := make([]int, len(fields))
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return nil, false
		}
		parts[i] = n
	}
	return parts, true
}
