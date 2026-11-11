// Package i18n is the native window's translations: the catalogs under
// locales/<tag>/<namespace>.json, one tree per namespace, read by a dotted
// key ("settings.language.uiTitle"). A message holds {name} placeholders or is
// a CLDR plural object, and it lands in every locale in the same change.
//
// Two languages never derive from each other: the interface's (Current, set
// by the window from the lich.appearance.language setting or the system's),
// read by T, and the prompt language, the one text typed into a session is
// written in, read by Prompt.
package i18n

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync/atomic"

	format "github.com/lichdotdev/lich/extension/i18n"
)

// Locale is a BCP 47 tag lich ships a catalog for: the same string the
// prompt.language setting and LICH_PROMPT_LANG use.
type Locale string

const (
	EN   Locale = "en"
	PtBR Locale = "pt-BR"
	ES   Locale = "es"
	ZhCN Locale = "zh-CN"
	JA   Locale = "ja"
	KO   Locale = "ko"
	RU   Locale = "ru"
)

// Locales are the languages the pickers offer, in their order.
var Locales = []Locale{EN, PtBR, ES, ZhCN, JA, KO, RU}

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
	case JA:
		return "日本語"
	case KO:
		return "한국어"
	case RU:
		return "Русский"
	}
	return "English"
}

// Match is the locale for a BCP 47 tag: an exact match, then a language
// match (any Portuguese reads pt-BR), then English.
func Match(tag string) Locale {
	tags := make([]string, len(Locales))
	for i, l := range Locales {
		tags[i] = string(l)
	}
	if m := format.Match(tag, tags); m != "" {
		return Locale(m)
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
	c, err := format.Load(fsys, path.Join("locales", dir))
	if err != nil {
		return nil, err
	}
	out := make(map[string]message, len(c))
	for key, m := range c {
		out[key] = message{text: m.Text, plural: m.Plural}
	}
	return out, nil
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
type Args = format.Args

// T is the message under key in the interface language, its placeholders
// filled from args.
func T(key string, args ...Args) string { return In(Current(), key, args...) }

// Prompt is the message under key in the prompt language: for text typed
// into a session, never for text the window shows.
func Prompt(key string, args ...Args) string { return In(PromptLanguage(), key, args...) }

// In is T in a named locale: the prompt language for text typed into a
// session. A key no catalog has, a plural without an int count or a
// placeholder without a value panics: each is a programming error the tests
// are there to catch.
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
	text, err := format.Message{Text: m.text, Plural: m.plural}.Template(string(l), values)
	if err != nil {
		panic(fmt.Sprintf("i18n: %q %v", key, err))
	}
	return text
}

var placeholder = format.Placeholder

func fill(text string, values Args) string {
	out, err := format.Fill(text, values)
	if err != nil {
		panic("i18n: " + err.Error())
	}
	return out
}

// Has reports whether the English catalog holds key, for a test that checks
// the keys code names.
func Has(key string) bool {
	_, ok := catalogs[EN][key]
	return ok
}
