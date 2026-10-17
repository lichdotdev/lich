package prompt

import (
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// TestCatalogParity is the guard a struct cannot give: every locale fills every
// field, and fills it with the arguments English expects, so a translation can
// reorder its arguments but never drop or invent one.
func TestCatalogParity(t *testing.T) {
	source := reflect.ValueOf(en)
	for _, lang := range Langs {
		locale := reflect.ValueOf(*For(lang))
		for i := range source.NumField() {
			field := source.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			for suffix, form := range texts(source.Field(i), locale.Field(i)) {
				name := field.Name + suffix
				if form.locale == "" {
					if !form.optional {
						t.Errorf("%s: %s is missing", lang, name)
					}
					continue
				}
				want, got := verbs(form.source), verbs(form.locale)
				if extra := slices.DeleteFunc(slices.Clone(got), func(v string) bool {
					return slices.Contains(want, v) || slices.Contains(form.mayAdd, v)
				}); len(extra) > 0 || !isSubset(want, got) {
					t.Errorf("%s: %s takes %v, English takes %v", lang, name, got, want)
				}
			}
		}
	}
}

// form is one text of a locale against the English one it must consume the
// arguments of. mayAdd are arguments it may take beyond those: a language
// whose one form also covers 21 or 31 (Russian) has to print the count that
// English's one form, meaning exactly one, leaves out.
type form struct {
	source, locale string
	mayAdd         []string
	optional       bool
}

// texts pairs a field's English text with the locale's, one pair per plural
// form for a Plural. Few and Many, which English has no form of, answer to its
// Other, and are written only where the language's rule selects them:
// TestEveryFormALanguageSelectsIsWritten checks that side.
func texts(source, locale reflect.Value) map[string]form {
	if plural, ok := source.Interface().(Plural); ok {
		other := locale.Interface().(Plural)
		return map[string]form{
			".One":   {source: plural.One, locale: other.One, mayAdd: verbs(plural.Other)},
			".Few":   {source: plural.Other, locale: other.Few, optional: true},
			".Many":  {source: plural.Other, locale: other.Many, optional: true},
			".Other": {source: plural.Other, locale: other.Other},
		}
	}
	return map[string]form{"": {source: source.String(), locale: locale.String()}}
}

func isSubset(names, of []string) bool {
	for _, n := range names {
		if !slices.Contains(of, n) {
			return false
		}
	}
	return true
}

// TestEveryFormALanguageSelectsIsWritten: Pick never lands on an empty form,
// for any count up to past the hundreds where Russian's rule repeats.
func TestEveryFormALanguageSelectsIsWritten(t *testing.T) {
	for _, lang := range Langs {
		catalog := reflect.ValueOf(*For(lang))
		for i := range catalog.NumField() {
			if !catalog.Type().Field(i).IsExported() {
				continue
			}
			plural, ok := catalog.Field(i).Interface().(Plural)
			if !ok {
				continue
			}
			for n := range 200 {
				if For(lang).Pick(n, plural) == "" {
					t.Errorf("%s: %s has no form for %d", lang, catalog.Type().Field(i).Name, n)
					break
				}
			}
		}
	}
}

var verbPattern = regexp.MustCompile(`%(\[(\d+)\])?[-+# 0]*[0-9]*(\.[0-9]+)?([a-zA-Z%])`)

// verbs lists the arguments a format consumes as "index:verb", sorted, with
// implicit indexes resolved the way fmt resolves them.
func verbs(format string) []string {
	var found []string
	next := 1
	for _, m := range verbPattern.FindAllStringSubmatch(format, -1) {
		if m[4] == "%" {
			continue
		}
		if m[2] != "" {
			next, _ = strconv.Atoi(m[2])
		}
		found = append(found, strconv.Itoa(next)+":"+m[4])
		next++
	}
	slices.Sort(found)
	return slices.Compact(found)
}

func TestParse(t *testing.T) {
	for tag, want := range map[string]Lang{
		"en": English, "pt-BR": PortugueseBR, "": English, "fr": English, "pt-br": English, "zh-CN": ChineseCN, "ja": Japanese, "ja-JP": English,
	} {
		if got := Parse(tag); got != want {
			t.Errorf("Parse(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestForPanicsOnALangWithNoCatalog(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("For(\"xx\") did not panic")
		}
	}()
	For("xx")
}

func TestPickFollowsEachLanguagesPluralRule(t *testing.T) {
	forms := Plural{One: "one", Other: "other"}
	cases := []struct {
		lang Lang
		n    int
		want string
	}{
		{English, 0, "other"}, {English, 1, "one"}, {English, 2, "other"},
		{PortugueseBR, 0, "one"}, {PortugueseBR, 1, "one"}, {PortugueseBR, 2, "other"},
		{ChineseCN, 0, "other"}, {ChineseCN, 1, "other"}, {ChineseCN, 2, "other"},
		{Japanese, 0, "other"}, {Japanese, 1, "other"}, {Japanese, 2, "other"},
	}
	for _, tc := range cases {
		if got := For(tc.lang).Pick(tc.n, forms); got != tc.want {
			t.Errorf("%s Pick(%d) = %q, want %q", tc.lang, tc.n, got, tc.want)
		}
	}
}

func TestPickFollowsRussiansFourForms(t *testing.T) {
	forms := Plural{One: "one", Few: "few", Many: "many", Other: "other"}
	for n, want := range map[int]string{
		0: "many", 1: "one", 2: "few", 5: "many", 11: "many", 12: "many",
		21: "one", 22: "few", 25: "many", 111: "many", 112: "many",
	} {
		if got := For(Russian).Pick(n, forms); got != want {
			t.Errorf("Russian Pick(%d) = %q, want %q", n, got, want)
		}
	}
	if got := Parse("ru"); got != Russian {
		t.Errorf("Parse(\"ru\") = %q, want %q", got, Russian)
	}
	if got := For(Russian).OriginCLI; got == For(English).OriginCLI || got == "" {
		t.Errorf("Russian OriginCLI = %q, want its own text", got)
	}
}

func TestSpanishIsAPromptLanguage(t *testing.T) {
	if got := Parse("es"); got != Spanish {
		t.Errorf("Parse(\"es\") = %q, want %q", got, Spanish)
	}
	if got := Parse("es-MX"); got != English {
		t.Errorf("Parse(\"es-MX\") = %q, want English: only the exact tag is a language", got)
	}
	forms := Plural{One: "one", Other: "other"}
	for n, want := range map[int]string{0: "other", 1: "one", 2: "other"} {
		if got := For(Spanish).Pick(n, forms); got != want {
			t.Errorf("Spanish Pick(%d) = %q, want %q", n, got, want)
		}
	}
	if got := For(Spanish).OriginCLI; got == For(English).OriginCLI || got == "" {
		t.Errorf("Spanish OriginCLI = %q, want its own text", got)
	}
}

func TestKoreanIsAPromptLanguage(t *testing.T) {
	if got := Parse("ko"); got != Korean {
		t.Errorf("Parse(\"ko\") = %q, want %q", got, Korean)
	}
	if got := Parse("ko-KR"); got != English {
		t.Errorf("Parse(\"ko-KR\") = %q, want English: only the exact tag is a language", got)
	}
	forms := Plural{One: "one", Other: "other"}
	for _, n := range []int{0, 1, 2} {
		if got := For(Korean).Pick(n, forms); got != "other" {
			t.Errorf("Korean Pick(%d) = %q, want other: Korean has no plural", n, got)
		}
	}
	if got := For(Korean).OriginCLI; got == For(English).OriginCLI || got == "" {
		t.Errorf("Korean OriginCLI = %q, want its own text", got)
	}
}
