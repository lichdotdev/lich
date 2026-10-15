package themes

import (
	"strings"
	"testing"
)

// withSyntax is customTheme's JSON with a "syntax" section spliced in, so
// the section reaches Import as an author wrote it, string and object forms
// alike.
func withSyntax(t *testing.T, id, syntax string) string {
	t.Helper()
	theme := customTheme(id)
	theme.Syntax = nil
	raw := rawTheme(t, theme)
	return strings.Replace(raw, `"app":`, `"syntax":`+syntax+`,"app":`, 1)
}

func TestImportKeepsTheSyntaxSection(t *testing.T) {
	s := NewInDir(t.TempDir())
	raw := withSyntax(t, "pine", `{
		"keyword": "#c4a7e7",
		"comment": {"color": "#6e6a86", "italic": true},
		"function(variableName)": "oklch(0.8 0.1 20)",
		"heading": {"bold": true}
	}`)
	if _, err := s.Import(rawThemeFile(t, raw), false); err != nil {
		t.Fatalf("Import: %v", err)
	}
	listed, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := listed[len(listed)-1].Syntax
	want := map[string]SyntaxStyle{
		"keyword":                {Color: "#c4a7e7"},
		"comment":                {Color: "#6e6a86", Italic: true},
		"function(variableName)": {Color: "oklch(0.8 0.1 20)"},
		"heading":                {Bold: true},
	}
	if len(got) != len(want) {
		t.Fatalf("syntax = %#v, want %#v", got, want)
	}
	for tag, style := range want {
		if got[tag] != style {
			t.Errorf("syntax[%q] = %#v, want %#v", tag, got[tag], style)
		}
	}
}

func TestImportRejectsABadSyntaxEntry(t *testing.T) {
	cases := map[string]struct{ syntax, reason string }{
		"unknown tag":      {`{"keywords": "#fff"}`, `"keywords" is not a highlighting tag`},
		"unknown modifier": {`{"bold(keyword)": "#fff"}`, `"bold" in "bold(keyword)" is not a tag modifier`},
		"unsafe color":     {`{"keyword": "url(x)"}`, `syntax.keyword is not an accepted color value`},
		"style of nothing": {`{"keyword": {}}`, `syntax.keyword sets no color and no font style`},
		"neither form":     {`{"keyword": 3}`, `syntax.keyword must be a color or a style object`},
	}
	for name, tc := range cases {
		s := NewInDir(t.TempDir())
		_, err := s.Import(rawThemeFile(t, withSyntax(t, "pine", tc.syntax)), false)
		if err == nil || !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: Import error = %v, want one naming %s", name, err, tc.reason)
		}
	}
}
