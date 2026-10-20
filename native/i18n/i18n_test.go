package i18n

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestMatchPicksExactThenLanguageThenEnglish(t *testing.T) {
	cases := map[string]Locale{
		"pt-BR": PtBR, "pt-br": PtBR, "pt": PtBR, "pt-PT": PtBR,
		"en-GB": EN, "es-MX": ES, "zh-CN": ZhCN, "zh-Hans-CN": ZhCN,
		"ja": JA, "ja-JP": JA,
		"fr-FR": EN, "": EN,
		"ko": KO, "ko-KR": KO,
		"ru": RU, "ru-RU": RU,
	}
	for tag, want := range cases {
		if got := Match(tag); got != want {
			t.Errorf("Match(%q) = %s, want %s", tag, got, want)
		}
	}
}

func TestEveryLocaleIsNamedInItself(t *testing.T) {
	want := []string{"English", "Português (Brasil)", "Español", "简体中文", "日本語", "한국어", "Русский"}
	for i, l := range Locales {
		if got := l.Name(); got != want[i] {
			t.Errorf("%s.Name() = %q, want %q", l, got, want[i])
		}
	}
}

func TestTIsEnglishUntilALanguageIsSet(t *testing.T) {
	if got := T("settings.language.uiTitle"); got != "Interface language" {
		t.Fatalf("T = %q", got)
	}
}

func TestSetCurrentChangesWhatTReads(t *testing.T) {
	SetCurrent(PtBR)
	defer SetCurrent(EN)
	if got := T("settings.language.uiTitle"); got != "Idioma da interface" {
		t.Fatalf("T = %q", got)
	}
}

func TestPromptFollowsThePromptLanguageNotTheInterfaces(t *testing.T) {
	SetCurrent(PtBR)
	SetPromptLanguage(ES)
	defer SetCurrent(EN)
	defer SetPromptLanguage(EN)
	if got := Prompt("prompts.reviewComments.header"); got != In(ES, "prompts.reviewComments.header") {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestInFillsPlaceholdersByName(t *testing.T) {
	got := In(EN, "settings.language.promptSaveFailed", Args{"error": "boom"})
	if got != "Could not save the prompt language: boom" {
		t.Fatalf("In = %q", got)
	}
}

func TestInPicksTheFormEachLanguageSelects(t *testing.T) {
	cases := []struct {
		l     Locale
		count int
		want  string
	}{
		{EN, 0, "0 files"}, {EN, 1, "1 file"}, {EN, 2, "2 files"},
		{PtBR, 0, "0 arquivo"}, {PtBR, 1, "1 arquivo"}, {PtBR, 2, "2 arquivos"},
		{ES, 1, "1 archivo"}, {ES, 3, "3 archivos"},
		{ZhCN, 1, "1 个文件"}, {ZhCN, 2, "2 个文件"},
		{JA, 1, "1 個のファイル"}, {JA, 2, "2 個のファイル"},
		{KO, 1, "파일 1개"}, {KO, 2, "파일 2개"},
	}
	for _, c := range cases {
		if got := In(c.l, "common.count.file", Args{"count": c.count}); got != c.want {
			t.Errorf("%s count %d = %q, want %q", c.l, c.count, got, c.want)
		}
	}
}

func TestRussianFormFollowsCLDR(t *testing.T) {
	for n, want := range map[int]string{
		0: "many", 1: "one", 2: "few", 5: "many", 11: "many", 12: "many",
		21: "one", 22: "few", 25: "many", 111: "many", 112: "many",
	} {
		if got := pluralForm(RU, n); got != want {
			t.Errorf("pluralForm(ru, %d) = %q, want %q", n, got, want)
		}
	}
}

// A form the language's rule selects but the catalog lacks would silently
// read "other", which in Russian is the fractions' form.
func TestEveryPluralHasTheFormsItsLanguageSelects(t *testing.T) {
	for _, l := range Locales {
		for key, m := range catalogs[l] {
			if m.plural == nil {
				continue
			}
			for n := range 200 {
				if form := pluralForm(l, n); m.plural[form] == "" {
					t.Errorf("%s: %s has no %q form (count %d)", l, key, form, n)
					break
				}
			}
		}
	}
}

func TestInPanicsOnAProgrammingError(t *testing.T) {
	cases := map[string]func(){
		"unknown key":        func() { In(EN, "settings.nope") },
		"unfilled":           func() { In(EN, "settings.language.promptSaveFailed") },
		"plural no count":    func() { In(EN, "common.count.file") },
		"plural float count": func() { In(EN, "common.count.file", Args{"count": 1.0}) },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("did not panic")
				}
			}()
			call()
		})
	}
}

func TestTagFromEnvReadsGlibcsOrder(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "pt_BR.UTF-8"}, "pt-BR"},
		{map[string]string{"LANG": "de_DE.UTF-8@euro"}, "de-DE"},
		{map[string]string{"LANG": "en_US.UTF-8", "LC_MESSAGES": "zh_CN.UTF-8"}, "zh-CN"},
		{map[string]string{"LC_ALL": "es_ES", "LC_MESSAGES": "zh_CN"}, "es-ES"},
		{map[string]string{"LANG": "C.UTF-8"}, ""},
		{map[string]string{"LC_ALL": "POSIX", "LANG": "pt_BR"}, ""},
		{map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := tagFromEnv(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("tagFromEnv(%v) = %q, want %q", c.env, got, c.want)
		}
	}
}

func placeholders(text string) []string {
	var names []string
	for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	sort.Strings(names)
	return slices.Compact(names)
}

// texts are every text of m, a plural's forms included.
func (m message) texts() []string {
	if m.plural == nil {
		return []string{m.text}
	}
	var out []string
	for _, s := range m.plural {
		out = append(out, s)
	}
	return out
}

// Over the JSON, every locale has English's keys and no other, each a plural
// where English's is, and uses the placeholders English uses (count aside,
// which a form may leave out).
func TestEveryLocaleMatchesEnglish(t *testing.T) {
	source := catalogs[EN]
	for _, l := range Locales[1:] {
		translated := catalogs[l]
		for key, en := range source {
			m, ok := translated[key]
			if !ok {
				t.Errorf("%s: missing %s", l, key)
				continue
			}
			if (m.plural == nil) != (en.plural == nil) {
				t.Errorf("%s: %s is a plural in one locale only", l, key)
				continue
			}
			if m.plural != nil && m.plural["other"] == "" {
				t.Errorf("%s: %s has no other form", l, key)
			}
			want := withoutCount(placeholders(strings.Join(en.texts(), " ")))
			for _, text := range m.texts() {
				if got := withoutCount(placeholders(text)); !subset(got, want) {
					t.Errorf("%s: %s uses %v, English %v", l, key, got, want)
				}
			}
			if got := withoutCount(placeholders(strings.Join(m.texts(), " "))); !slices.Equal(got, want) {
				t.Errorf("%s: %s uses %v, English %v", l, key, got, want)
			}
		}
		for key := range translated {
			if _, ok := source[key]; !ok {
				t.Errorf("%s: %s is not in English", l, key)
			}
		}
	}
}

func withoutCount(names []string) []string {
	return slices.DeleteFunc(names, func(n string) bool { return n == "count" })
}

func subset(names, of []string) bool {
	for _, n := range names {
		if !slices.Contains(of, n) {
			return false
		}
	}
	return true
}
