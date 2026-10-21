// Package site checks the website under docs/, which has no build step: each page is written out whole, so
// nothing but these tests keeps the pages a visitor moves between in step.
package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	changelogLink = regexp.MustCompile(`changelog((?:\.[\w-]+)?)\.html`)
	landingAnchor = regexp.MustCompile(`href="index(?:\.[\w-]+)?\.html(#?)`)
)

// sameSite rewrites the links a changelog page points back at its landing so both pages read alike: a link to
// either page in any language becomes a same-page link, as the landing writes its own anchors.
func sameSite(block string) string {
	return landingAnchor.ReplaceAllString(changelogLink.ReplaceAllString(block, "index$1.html"), `href="#`)
}

func element(t *testing.T, page, tag string) string {
	t.Helper()
	raw, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	start := strings.Index(s, "<"+tag)
	end := strings.Index(s, "</"+tag+">")
	if start < 0 || end < start {
		t.Fatalf("%s has no <%s>", page, tag)
	}
	return s[start : end+len(tag)+3]
}

// TestChangelogPagesShareTheLandingChrome: the changelog in each language carries its landing's header and footer,
// so moving between them never looks like another site.
func TestChangelogPagesShareTheLandingChrome(t *testing.T) {
	landings, err := filepath.Glob("../../docs/index*.html")
	if err != nil || len(landings) == 0 {
		t.Fatalf("no landing pages under docs/: %v", err)
	}
	for _, landing := range landings {
		changelog := filepath.Join(filepath.Dir(landing), "changelog"+strings.TrimPrefix(filepath.Base(landing), "index"))
		for _, tag := range []string{"header", "footer"} {
			if sameSite(element(t, landing, tag)) != sameSite(element(t, changelog, tag)) {
				t.Errorf("%s does not carry the <%s> of %s: copy it over, with its links pointed back at the landing",
					filepath.Base(changelog), tag, filepath.Base(landing))
			}
		}
	}
}
