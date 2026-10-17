// Package i18n is the native window's translations: the catalogs under
// locales/<tag>/<namespace>.json, one tree per namespace, read by a dotted
// key ("settings.language.uiTitle"). They were seeded from the web window's
// TypeScript catalogs (frontend/src/lib/i18n) and keep its keys, its {name}
// placeholders and its CLDR plural objects; from then on these files are the
// native window's own, and a message lands in every locale in the same change.
//
// Two languages never derive from each other: the interface's (Current, set
// by the window from the lich.appearance.language setting or the system's),
// read by T, and the prompt language, the one text typed into a session is
// written in, read by Prompt.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"sync/atomic"
)

// Locale is a BCP 47 tag lich ships a catalog for: the same string the web
// window, the prompt.language setting and LICH_PROMPT_LANG use.
type Locale string

const (
	EN   Locale = "en"
	PtBR Locale = "pt-BR"
	ES   Locale = "es"
	ZhCN Locale = "zh-CN"
)

// Locales are the languages the pickers offer, in the web window's order.
var Locales = []Locale{EN, PtBR, ES, ZhCN}

// Name is the language named in itself, which is how someone who cannot read
// the current interface still finds their own.
func (l Locale) Name() string {
	switch l {
	case PtBR:
		return "Português (Brasil)"
	case ES:
		return "Español"
	case ZhCN:
		return "简体中文"
	}
	return "English"
}

// Match is the locale for a BCP 47 tag: an exact match, then a language
// match (any Portuguese reads pt-BR), then English. It is the web window's
// matchLocale, so both windows land on the same language for the same tag.
func Match(tag string) Locale {
	for _, l := range Locales {
		if strings.EqualFold(string(l), tag) {
			return l
		}
	}
	language, _, _ := strings.Cut(tag, "-")
	for _, l := range Locales {
		prefix, _, _ := strings.Cut(string(l), "-")
		if language != "" && strings.EqualFold(prefix, language) {
			return l
		}
	}
	return EN
}

//go:embed locales
var files embed.FS

// message is a leaf of a catalog: a text, or a plural's CLDR forms.
type message struct {
	text   string
	plural map[string]string
}

var catalogs = mustLoad()

func mustLoad() map[Locale]map[string]message {
	loaded := make(map[Locale]map[string]message, len(Locales))
	for _, l := range Locales {
		c, err := load(files, strings.ToLower(string(l)))
		if err != nil {
			panic(fmt.Sprintf("i18n: %s: %v", l, err))
		}
		loaded[l] = c
	}
	return loaded
}

// load reads every namespace file of the locale directory dir into one map
// from dotted key to message.
func load(fsys fs.FS, dir string) (map[string]message, error) {
	entries, err := fs.ReadDir(fsys, path.Join("locales", dir))
	if err != nil {
		return nil, err
	}
	out := map[string]message{}
	for _, e := range entries {
		data, err := fs.ReadFile(fsys, path.Join("locales", dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var tree map[string]any
		if err := json.Unmarshal(data, &tree); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := flatten(out, strings.TrimSuffix(e.Name(), ".json"), tree); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return out, nil
}

// flatten walks tree into out under prefix. An object holding "other" is a
// plural, as in the web's catalog type: every CLDR rule falls back to it.
func flatten(out map[string]message, prefix string, tree map[string]any) error {
	if _, ok := tree["other"]; ok {
		forms := make(map[string]string, len(tree))
		for form, v := range tree {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("%s.%s: a plural form must be a string", prefix, form)
			}
			forms[form] = s
		}
		out[prefix] = message{plural: forms}
		return nil
	}
	for k, v := range tree {
		key := prefix + "." + k
		switch v := v.(type) {
		case string:
			out[key] = message{text: v}
		case map[string]any:
			if err := flatten(out, key, v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: want a string or an object, got %T", key, v)
		}
	}
	return nil
}

var current atomic.Value // Locale

// Current is the interface language, English until the window sets one:
// tests, which never set it, read English.
func Current() Locale {
	if l, ok := current.Load().(Locale); ok {
		return l
	}
	return EN
}

// SetCurrent changes the interface language. Every label reads its text at
// layout, so the next frame draws in l.
func SetCurrent(l Locale) { current.Store(l) }

var promptLanguage atomic.Value // Locale

// PromptLanguage is the language of the text the window types into a
// session, the prompt.language setting; English until the window sets it, as
// the backend reads an absent value.
func PromptLanguage() Locale {
	if l, ok := promptLanguage.Load().(Locale); ok {
		return l
	}
	return EN
}

// SetPromptLanguage changes the language Prompt writes in.
func SetPromptLanguage(l Locale) { promptLanguage.Store(l) }

// Args are a message's placeholder values by name; "count" picks a plural's
// form and must be an int.
type Args map[string]any

// T is the message under key in the interface language, its placeholders
// filled from args.
func T(key string, args ...Args) string { return In(Current(), key, args...) }

// Prompt is the message under key in the prompt language: for text typed
// into a session, never for text the window shows.
func Prompt(key string, args ...Args) string { return In(PromptLanguage(), key, args...) }

// In is T in a named locale: the prompt language for text typed into a
// session. A key no catalog has, a plural without an int count or a
// placeholder without a value panics, as the web's t() throws: each is a
// programming error the tests are there to catch.
func In(l Locale, key string, args ...Args) string {
	var values Args
	if len(args) > 0 {
		values = args[0]
	}
	return fill(template(l, key, values), values)
}

func template(l Locale, key string, values Args) string {
	m, ok := catalogs[l][key]
	if !ok {
		panic(fmt.Sprintf("i18n: no message %q in %s", key, l))
	}
	if m.plural == nil {
		return m.text
	}
	count, ok := values["count"].(int)
	if !ok {
		panic(fmt.Sprintf("i18n: %q is a plural and needs an int count", key))
	}
	if form, ok := m.plural[pluralForm(l, count)]; ok {
		return form
	}
	return m.plural["other"]
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

func fill(text string, values Args) string {
	return placeholder.ReplaceAllStringFunc(text, func(match string) string {
		name := match[1 : len(match)-1]
		v, ok := values[name]
		if !ok {
			panic(fmt.Sprintf("i18n: no value for {%s} in %q", name, text))
		}
		return fmt.Sprint(v)
	})
}

// Has reports whether the English catalog holds key, for a test that checks
// the keys code names.
func Has(key string) bool {
	_, ok := catalogs[EN][key]
	return ok
}
