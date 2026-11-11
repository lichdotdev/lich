package i18n

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"sync/atomic"

	"github.com/lichdotdev/lich/extension"
)

// LocalesDir is the folder of an extension's catalogs, beside its manifest.
const LocalesDir = "locales"

// Source is the language catalogs are written in first, and the one a
// message missing from a translation falls back to.
const Source = "en"

// Bundle is every catalog an extension ships, by tag as its folder names it.
type Bundle struct {
	catalogs map[string]Catalog
	tags     []string
}

// Open reads locales/<tag>/<namespace>.json under fsys. A missing locales
// folder is an empty bundle, which answers every key with itself.
func Open(fsys fs.FS) (*Bundle, error) {
	b := &Bundle{catalogs: map[string]Catalog{}}
	entries, err := fs.ReadDir(fsys, LocalesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := Load(fsys, path.Join(LocalesDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", LocalesDir, e.Name(), err)
		}
		b.catalogs[e.Name()] = c
		b.tags = append(b.tags, e.Name())
	}
	sort.Strings(b.tags)
	return b, nil
}

// OpenDir is Open on the folder dir, an extension's own: the folder its
// executable runs in.
func OpenDir(dir string) (*Bundle, error) { return Open(os.DirFS(dir)) }

// Tags are the languages the bundle has a catalog for, sorted.
func (b *Bundle) Tags() []string { return append([]string(nil), b.tags...) }

// Lookup is the message under key in the catalog tag reads, then in English,
// with the tag whose catalog had it.
func (b *Bundle) Lookup(tag, key string) (Message, string, bool) {
	for _, t := range []string{Match(tag, b.tags), Match(Source, b.tags)} {
		if m, ok := b.catalogs[t][key]; ok {
			return m, t, true
		}
	}
	return Message{}, "", false
}

// In is the message under key in the language tag, its placeholders filled
// from args: English when the tag's catalog lacks it, and the key itself
// when no catalog has it. A plural without an int count reads its "other"
// form, and a placeholder without a value stays as written.
func (b *Bundle) In(tag, key string, args ...Args) string {
	var values Args
	if len(args) > 0 {
		values = args[0]
	}
	m, found, ok := b.Lookup(tag, key)
	if !ok {
		return key
	}
	text, err := m.Template(found, values)
	if err != nil {
		text = m.Plural["other"]
	}
	text, _ = Fill(text, values)
	return text
}

// Language is the tag lich shows its interface in, as an extension learns
// it: the environment at start, then the language field of every
// surface.opened and every language message, which the extension passes to
// Set. It is safe to use from any goroutine.
type Language struct{ tag atomic.Value }

// FromEnv starts from the variable lich starts an extension with, English
// when it is unset.
func FromEnv() *Language {
	l := &Language{}
	l.Set(os.Getenv(extension.LanguageEnv))
	return l
}

// Set records tag; an empty one changes nothing.
func (l *Language) Set(tag string) {
	if tag != "" {
		l.tag.Store(tag)
	}
}

// Tag is the language last set, English before any.
func (l *Language) Tag() string {
	if t, ok := l.tag.Load().(string); ok {
		return t
	}
	return Source
}
