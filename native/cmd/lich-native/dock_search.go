package main

// The Code tab in text mode (SearchResults.tsx and lib/git/file-search.ts):
// the lines holding the query, one group per file, each opening the preview
// on its line.

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/omartelo/lich/native/i18n"
)

// searchSettle is how long typing has to pause before the checkout is read:
// one search reads every file the tree lists, and a word typed at speed is
// not five searches.
const searchSettle = 250 * time.Millisecond

// searchMatch and searchResult mirror internal/project's SearchMatch and
// SearchResult.
type searchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type searchResult struct {
	Matches  []searchMatch `json:"matches"`
	Cut      bool          `json:"cut"`
	TooLarge int           `json:"tooLarge"`
}

type searchGroup struct {
	path string
	hits []searchMatch
}

// groupMatches folds the backend's list, already file by file, into one group
// per file.
func groupMatches(matches []searchMatch) []searchGroup {
	var groups []searchGroup
	for _, m := range matches {
		if n := len(groups); n > 0 && groups[n-1].path == m.Path {
			groups[n-1].hits = append(groups[n-1].hits, m)
			continue
		}
		groups = append(groups, searchGroup{path: m.Path, hits: []searchMatch{m}})
	}
	return groups
}

type segment struct {
	text string
	hit  bool
}

// highlightSegments splits a hit's line into the runs matching query and the
// runs between, case-insensitively and as a literal, the way the backend
// matched. A line whose lowercase changes length is shown plain rather than
// cut at the wrong offsets.
func highlightSegments(text, query string) []segment {
	needle := strings.ToLower(query)
	hay := strings.ToLower(text)
	if needle == "" || len(hay) != len(text) {
		return []segment{{text: text}}
	}
	var out []segment
	from := 0
	for {
		at := strings.Index(hay[from:], needle)
		if at < 0 {
			break
		}
		at += from
		if at > from {
			out = append(out, segment{text: text[from:at]})
		}
		out = append(out, segment{text: text[at : at+len(needle)], hit: true})
		from = at + len(needle)
	}
	if from < len(text) {
		out = append(out, segment{text: text[from:]})
	}
	return out
}

// searchSummary is the label over the list. A cut answer counts the lines it
// stopped at, not files: the files past the cap were never read.
func searchSummary(r searchResult, groups int) string {
	lines := len(r.Matches)
	if r.Cut {
		return i18n.T("git.fileSearch.cutMatches", i18n.Args{"count": lines})
	}
	return i18n.T("git.fileSearch.summary", i18n.Args{
		"matches": i18n.T("git.fileSearch.matches", i18n.Args{"count": lines}),
		"files":   i18n.T("common.count.file", i18n.Args{"count": groups}),
	})
}

// searchFootnote is what the answer did not read: the lines past the cap and
// the files skipped for size.
func searchFootnote(r searchResult) string {
	var parts []string
	if r.Cut {
		parts = append(parts, i18n.T("git.fileSearch.cutNote", i18n.Args{"count": len(r.Matches)}))
	}
	if r.TooLarge > 0 {
		parts = append(parts, i18n.T("git.fileSearch.tooLarge", i18n.Args{"count": r.TooLarge}))
	}
	return strings.Join(parts, " ")
}

// settler is SearchResults' useSettled: the query once it has held still for
// searchSettle; clearing the box settles at once.
type settler struct {
	typed   string
	since   time.Time
	settled string
}

// at feeds the query typed now and answers the settled one, plus when to look
// again while a pause is still running (zero for never).
func (s *settler) at(query string, now time.Time) (string, time.Time) {
	if query != s.typed {
		s.typed, s.since = query, now
	}
	if query == "" || !now.Before(s.since.Add(searchSettle)) {
		s.settled = query
		return s.settled, time.Time{}
	}
	return s.settled, s.since.Add(searchSettle)
}

// splitPath is lang-badge.ts splitPath: the directory and the base name.
func splitPath(p string) (dir, base string) {
	slash := strings.LastIndexByte(p, '/')
	if slash < 0 {
		return "", p
	}
	return p[:slash], p[slash+1:]
}

// lineNumberCells is the hit's w-[4ch] line-number column.
const lineNumberCells = 4

func padLineNumber(n int) string {
	s := fmt.Sprint(n)
	return strings.Repeat(" ", max(lineNumberCells-utf8.RuneCountInString(s), 0)) + s
}
