// Package prompt holds the natural-language text lich types into an agent
// session, in every language a user can pick for it. The UI has its own
// language and its own catalogs (native/i18n); the two settings are
// independent because people often run the interface in one language and talk
// to their agents in another.
//
// A catalog is a struct rather than a map of string keys so that a wrong name
// is a compile error at the call site, and a locale that misses a field is
// caught by TestCatalogParity. Each field is a fmt format: use explicit
// argument indexes (%[2]s) when a translation needs the arguments in another
// order.
package prompt

// Lang is a prompt language, spelled as a BCP 47 tag: the same value is stored
// in the settings table, exported to every session as EnvVar, and offered by
// the window's select.
type Lang string

const (
	English      Lang = "en"
	PortugueseBR Lang = "pt-BR"
	Spanish      Lang = "es"
	ChineseCN    Lang = "zh-CN"
	Japanese     Lang = "ja"
	Korean       Lang = "ko"
	Russian      Lang = "ru"
)

// Langs is every language a catalog exists for, English first as the default.
var Langs = []Lang{English, PortugueseBR, Spanish, ChineseCN, Japanese, Korean, Russian}

// EnvVar is the variable lich exports into every session PTY with the prompt
// language at spawn time. Text composed outside the lich process (the lich CLI,
// its MCP server, the companion plugin's hooks) reads the language from here,
// because it has no way to read the setting itself.
const EnvVar = "LICH_PROMPT_LANG"

// Parse reads a stored or exported language tag. An empty or unknown tag is
// English: absent is the default by contract, and a tag from a newer build
// must not stop an older one from talking to its agents.
func Parse(tag string) Lang {
	for _, lang := range Langs {
		if string(lang) == tag {
			return lang
		}
	}
	return English
}

var catalogs = map[Lang]*Catalog{
	English:      &en,
	PortugueseBR: &ptBR,
	Spanish:      &es,
	ChineseCN:    &zhCN,
	Japanese:     &ja,
	Korean:       &ko,
	Russian:      &ru,
}

// For is the catalog of a language. A Lang that came through Parse always has
// one; a hand-built Lang with no catalog panics, which is a programming error.
func For(lang Lang) *Catalog {
	catalog, ok := catalogs[lang]
	if !ok {
		panic("prompt: no catalog for language " + string(lang))
	}
	return catalog
}

// Plural is a text that changes with a count, one field per CLDR form. Few
// and Many are written only by a language whose rule selects them (Russian)
// and stay empty elsewhere.
type Plural struct {
	One   string
	Few   string
	Many  string
	Other string
}

// Pick is the form of p this catalog's language uses for n.
func (c *Catalog) Pick(n int, p Plural) string {
	switch {
	case c.isOne(n):
		return p.One
	case c.isFew != nil && c.isFew(n):
		return p.Few
	case c.isMany != nil && c.isMany(n):
		return p.Many
	}
	return p.Other
}

// isOneEnglish, isOnePortuguese and isOneSpanish are the CLDR "one" rules for
// integers: Portuguese counts zero as singular, English and Spanish do not.
// Chinese, Japanese and Korean have no plural: isOneChinese, isOneJapanese and
// isOneKorean are never true, so Pick always returns Other.
func isOneEnglish(n int) bool    { return n == 1 }
func isOnePortuguese(n int) bool { return n == 0 || n == 1 }
func isOneSpanish(n int) bool    { return n == 1 }
func isOneChinese(int) bool      { return false }
func isOneJapanese(int) bool     { return false }
func isOneKorean(int) bool       { return false }

// isOneRussian, isFewRussian and isManyRussian are Russian's CLDR rules for
// integers: one is 1, 21, 31... but not 11; few is 2-4, 22-24... but not
// 12-14; many is every other integer, 0 and 11-14 included. CLDR's other is
// left to fractions, which no count here is.
func isOneRussian(n int) bool { return n%10 == 1 && n%100 != 11 }
func isFewRussian(n int) bool {
	return n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14)
}
func isManyRussian(n int) bool { return !isOneRussian(n) && !isFewRussian(n) }
