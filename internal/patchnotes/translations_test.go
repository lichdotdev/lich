package patchnotes

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
)

// The translations live beside CHANGELOG.md at the module root, where main.go
// embeds them.
const repoRoot = "../.."

var codeSpan = regexp.MustCompile("`[^`]+`")

// TestTranslations holds every changelog/<locale>.md to CHANGELOG.md: its
// releases are the newest ones there, latest first and none skipped, each under
// the same heading, with the same groups, item counts, alert kinds, code spans
// and bold lead-ins. A release cut without running the lich-changelog-i18n
// skill fails here.
func TestTranslations(t *testing.T) {
	english := readRepoFile(t, "CHANGELOG.md")
	released := releaseHeadings(t, english)
	for _, lang := range prompt.Langs {
		if lang == prompt.English {
			continue
		}
		t.Run(string(lang), func(t *testing.T) {
			translated := readRepoFile(t, filepath.Join("changelog", string(lang)+".md"))
			headings := versionHeadings(translated)
			if len(headings) == 0 || len(headings) > len(released) || !reflect.DeepEqual(headings, released[:len(headings)]) {
				t.Fatalf("version headings = %q, want the newest releases of CHANGELOG.md in order, starting at %q", headings, released[0])
			}
			for _, heading := range headings {
				version := heading[len("## ["):strings.Index(heading, "]")]
				t.Run(version, func(t *testing.T) {
					assertSameShape(t, Section(english, version), Section(translated, version))
					assertSameHighlights(t, Highlights(english, version), Highlights(translated, version))
				})
			}
		})
	}
}

func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func versionHeadings(changelog string) []string {
	var out []string
	for _, line := range strings.Split(changelog, "\n") {
		if strings.HasPrefix(line, "## [") {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func releaseHeadings(t *testing.T, changelog string) []string {
	t.Helper()
	var out []string
	for _, heading := range versionHeadings(changelog) {
		if !strings.HasPrefix(heading, "## [Unreleased]") {
			out = append(out, heading)
		}
	}
	if len(out) == 0 {
		t.Fatal("CHANGELOG.md has no released version")
	}
	return out
}

func assertSameShape(t *testing.T, english, translated []Group) {
	t.Helper()
	if len(english) != len(translated) {
		t.Fatalf("%d groups, want %d", len(translated), len(english))
	}
	for i, group := range english {
		got := translated[i]
		if got.Label != group.Label {
			t.Fatalf("group %d is %q, want %q left in English", i, got.Label, group.Label)
		}
		if len(got.Items) != len(group.Items) {
			t.Fatalf("%s has %d items, want %d", group.Label, len(got.Items), len(group.Items))
		}
		for j, item := range group.Items {
			assertSameMarkup(t, item, got.Items[j])
		}
	}
}

func assertSameHighlights(t *testing.T, english, translated []Highlight) {
	t.Helper()
	if len(english) != len(translated) {
		t.Fatalf("%d highlights, want %d", len(translated), len(english))
	}
	for i, highlight := range english {
		if translated[i].Kind != highlight.Kind {
			t.Fatalf("highlight %d is %q, want %q", i, translated[i].Kind, highlight.Kind)
		}
		assertSameMarkup(t, highlight.Text, translated[i].Text)
	}
}

// assertSameMarkup checks what a translation must not touch: code spans stay
// literal, in whatever order the language's grammar puts them, and the bold
// lead-in keeps its markers.
func assertSameMarkup(t *testing.T, english, translated string) {
	t.Helper()
	if got, want := sortedCodeSpans(translated), sortedCodeSpans(english); !reflect.DeepEqual(got, want) {
		t.Fatalf("code spans %q, want %q in:\n%s", got, want, translated)
	}
	if got, want := strings.Count(translated, "**"), strings.Count(english, "**"); got != want {
		t.Fatalf("%d bold markers, want %d in:\n%s", got, want, translated)
	}
}

func sortedCodeSpans(text string) []string {
	spans := codeSpan.FindAllString(text, -1)
	slices.Sort(spans)
	return spans
}
