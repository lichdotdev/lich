package i18n

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lichdotdev/lich/extension"
)

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

func testBundle(t *testing.T) *Bundle {
	t.Helper()
	b, err := Open(fstest.MapFS{
		"locales/en/board.json": file(`{"title": "Board", "greet": "Hello, {name}", "only": "English only",
			"cards": {"one": "{count} card", "other": "{count} cards"}}`),
		"locales/pt-br/board.json": file(`{"title": "Quadro", "greet": "Olá, {name}",
			"cards": {"one": "{count} cartão", "other": "{count} cartões"}}`),
		"locales/ru/board.json": file(`{"cards": {"one": "{count} карточка", "few": "{count} карточки", "many": "{count} карточек", "other": "{count} карточки"}}`),
		"locales/ja/board.json": file(`{"cards": {"other": "{count} 枚"}}`),
		"locales/README.md":     file("not a catalog"),
		"locales/en/notes.txt":  file("{not json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOpenReadsEveryLocaleFolder(t *testing.T) {
	if got := strings.Join(testBundle(t).Tags(), " "); got != "en ja pt-br ru" {
		t.Fatalf("Tags = %q", got)
	}
}

func TestInLooksUpAndFillsInTheLanguageAsked(t *testing.T) {
	b := testBundle(t)
	cases := [][2]string{
		{b.In("pt-BR", "board.title"), "Quadro"},
		{b.In("pt-PT", "board.title"), "Quadro"},
		{b.In("en", "board.greet", Args{"name": "Ana"}), "Hello, Ana"},
		{b.In("pt-BR", "board.greet", Args{"name": "Ana"}), "Olá, Ana"},
		{b.In("pt-BR", "board.greet"), "Olá, {name}"},
		{b.In("pt-BR", "board.only"), "English only"},
		{b.In("fr", "board.title"), "Board"},
		{b.In("pt-BR", "board.nope"), "board.nope"},
		{b.In("en", "board.cards"), "{count} cards"},
		{b.In("en", "board.greet", Args{"name": "{x}", "x": 1}), "Hello, {x}"},
	}
	for i, c := range cases {
		if c[0] != c[1] {
			t.Errorf("case %d: got %q, want %q", i, c[0], c[1])
		}
	}
}

func TestInPicksEveryPluralFormLichHas(t *testing.T) {
	b := testBundle(t)
	cases := []struct {
		tag   string
		count int
		want  string
	}{
		{"en", 0, "0 cards"}, {"en", 1, "1 card"}, {"en", 2, "2 cards"},
		{"pt-BR", 0, "0 cartão"}, {"pt-BR", 1, "1 cartão"}, {"pt-BR", 2, "2 cartões"},
		{"ru", 1, "1 карточка"}, {"ru", 3, "3 карточки"}, {"ru", 5, "5 карточек"},
		{"ru", 11, "11 карточек"}, {"ru", 21, "21 карточка"}, {"ru", 22, "22 карточки"},
		{"ja", 1, "1 枚"}, {"ja", 2, "2 枚"},
		{"es", 1, "1 card"}, {"es", 4, "4 cards"},
	}
	for _, c := range cases {
		if got := b.In(c.tag, "board.cards", Args{"count": c.count}); got != c.want {
			t.Errorf("%s %d = %q, want %q", c.tag, c.count, got, c.want)
		}
	}
}

func TestPluralFormFollowsEachLanguagesRule(t *testing.T) {
	cases := []struct {
		tag  string
		n    int
		want string
	}{
		{"en", 1, "one"}, {"en", 0, "other"}, {"en-GB", 1, "one"},
		{"es", 1, "one"}, {"es", 2, "other"},
		{"pt-BR", 0, "one"}, {"pt-br", 1, "one"}, {"pt-BR", 2, "other"},
		{"ru", 1, "one"}, {"ru", 2, "few"}, {"ru", 12, "many"}, {"ru", 0, "many"}, {"ru", 112, "many"},
		{"zh-CN", 1, "other"}, {"ja", 1, "other"}, {"ko", 1, "other"},
	}
	for _, c := range cases {
		if got := PluralForm(c.tag, c.n); got != c.want {
			t.Errorf("PluralForm(%s, %d) = %q, want %q", c.tag, c.n, got, c.want)
		}
	}
}

func TestMatchIsExactThenLanguageThenNone(t *testing.T) {
	among := []string{"en", "pt-br", "zh-CN"}
	for tag, want := range map[string]string{"PT-BR": "pt-br", "pt": "pt-br", "zh-Hans-CN": "zh-CN", "en-US": "en", "fr": "", "": ""} {
		if got := Match(tag, among); got != want {
			t.Errorf("Match(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestTemplateAndFillReportWhatIsMissing(t *testing.T) {
	plural := Message{Plural: map[string]string{"other": "n"}}
	if _, err := plural.Template("en", Args{"count": 1.0}); err != ErrNoCount {
		t.Errorf("a float count: %v", err)
	}
	if got, err := Fill("{a} and {b}", Args{"a": 1}); err == nil || got != "1 and {b}" {
		t.Errorf("Fill = %q, %v", got, err)
	}
}

func TestOpenRefusesABadCatalog(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"not json":      {"locales/en/a.json": file(`{"title": `)},
		"a number leaf": {"locales/en/a.json": file(`{"title": 1}`)},
		"plural object": {"locales/en/a.json": file(`{"n": {"one": {}, "other": "x"}}`)},
	} {
		if _, err := Open(fsys); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if b, err := Open(fstest.MapFS{}); err != nil || b.In("en", "k") != "k" {
		t.Errorf("no locales folder: %v", err)
	}
}

func TestLanguageStartsFromTheEnvironmentAndFollowsSet(t *testing.T) {
	t.Setenv(extension.LanguageEnv, "")
	l := FromEnv()
	if l.Tag() != Source {
		t.Errorf("unset = %q", l.Tag())
	}
	t.Setenv(extension.LanguageEnv, "ja")
	l = FromEnv()
	l.Set("")
	if l.Tag() != "ja" {
		t.Errorf("from env = %q", l.Tag())
	}
	l.Set("pt-BR")
	if l.Tag() != "pt-BR" {
		t.Errorf("after Set = %q", l.Tag())
	}
}

func TestTagFromEnvReadsGlibcsOrder(t *testing.T) {
	env := map[string]string{"LANG": "en_US.UTF-8", "LC_MESSAGES": "pt_BR.UTF-8@euro"}
	if got := TagFromEnv(func(k string) string { return env[k] }); got != "pt-BR" {
		t.Errorf("TagFromEnv = %q", got)
	}
	if got := TagFromEnv(func(string) string { return "C.UTF-8" }); got != "" {
		t.Errorf("C = %q", got)
	}
}
