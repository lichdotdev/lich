package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/lichdotdev/godemirror"
)

// patchFile is one file of a unified diff, read far enough to know what
// godemirror's FilePatch keeps to itself: how many rows it lays out (a card
// is as tall as its diff, and DiffView only fills what it is given), whether
// it is binary, and its hunks, which hiding whitespace rewrites.
type patchFile struct {
	path, oldPath string
	status        fileStatus
	binary        bool
	header        []string
	hunks         []patchHunk
}

type fileStatus uint8

const (
	statusModified fileStatus = iota
	statusAdded
	statusDeleted
	statusRenamed
)

// patchHunk is one "@@" stretch. Starts are numbered as godemirror numbers
// them: a side with no lines starts at the line the hunk sits before, where
// git prints the line before it.
type patchHunk struct {
	oldStart, oldCount, newStart, newCount int
	head                                   string
	lines                                  []string // with their +, - or space
}

// parsePatchFiles splits a git diff into its files, in order, as
// godemirror.ParsePatch does.
func parsePatchFiles(text string) ([]patchFile, error) {
	var files []patchFile
	for line := range strings.Lines(text) {
		line = strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(line, "diff --git ") {
			files = append(files, patchFile{path: newPathOf(line)})
		}
		if len(files) == 0 {
			continue
		}
		if err := files[len(files)-1].read(line); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func newPathOf(header string) string {
	if i := strings.LastIndex(header, " b/"); i >= 0 {
		return header[i+3:]
	}
	return strings.TrimPrefix(header, "diff --git ")
}

func (f *patchFile) read(line string) error {
	if len(f.hunks) == 0 && !strings.HasPrefix(line, "@@") {
		f.header = append(f.header, line)
		f.readHeader(line)
		return nil
	}
	if strings.HasPrefix(line, "@@") {
		h, err := parseHunkHead(line)
		if err != nil {
			return fmt.Errorf("%s: %w", f.path, err)
		}
		f.hunks = append(f.hunks, h)
		return nil
	}
	h := &f.hunks[len(f.hunks)-1]
	h.lines = append(h.lines, line)
	return nil
}

func (f *patchFile) readHeader(line string) {
	switch {
	case strings.HasPrefix(line, "new file mode"):
		f.status = statusAdded
	case strings.HasPrefix(line, "deleted file mode"):
		f.status = statusDeleted
	case strings.HasPrefix(line, "rename from "):
		f.status, f.oldPath = statusRenamed, strings.TrimPrefix(line, "rename from ")
	case strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch":
		f.binary = true
	case strings.HasPrefix(line, "--- a/"):
		f.oldPath = strings.TrimPrefix(line, "--- a/")
	case strings.HasPrefix(line, "+++ b/"):
		f.path = strings.TrimPrefix(line, "+++ b/")
	}
}

// parseHunkHead reads "@@ -a,b +c,d @@", a missing count being 1.
func parseHunkHead(head string) (patchHunk, error) {
	h := patchHunk{head: head, oldCount: 1, newCount: 1}
	old, rest, ok := strings.Cut(strings.TrimPrefix(head, "@@ -"), " +")
	if !ok {
		return h, fmt.Errorf("hunk header %q", head)
	}
	rest, _, _ = strings.Cut(rest, " ")
	if err := parseHunkRange(old, &h.oldStart, &h.oldCount); err != nil {
		return h, fmt.Errorf("hunk header %q: %w", head, err)
	}
	if err := parseHunkRange(rest, &h.newStart, &h.newCount); err != nil {
		return h, fmt.Errorf("hunk header %q: %w", head, err)
	}
	if h.oldCount == 0 {
		h.oldStart++
	}
	if h.newCount == 0 {
		h.newStart++
	}
	return h, nil
}

func parseHunkRange(r string, start, count *int) error {
	s, c, hasCount := strings.Cut(r, ",")
	var err error
	if *start, err = strconv.Atoi(s); err != nil {
		return err
	}
	if hasCount {
		*count, err = strconv.Atoi(c)
	}
	return err
}

// text is the file's patch as git would print it, for godemirror to parse.
func (f patchFile) text() string {
	var b strings.Builder
	for _, l := range f.header {
		b.WriteString(l + "\n")
	}
	for _, h := range f.hunks {
		b.WriteString(h.head + "\n")
		for _, l := range h.lines {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

// filePatch is f as godemirror reads it.
func (f patchFile) filePatch() (godemirror.FilePatch, error) {
	parsed, err := godemirror.ParsePatch(f.text())
	if err != nil || len(parsed) != 1 {
		return godemirror.FilePatch{Path: f.path}, err
	}
	return parsed[0], nil
}

// gapBefore is the unchanged run between prev (nil: the top of the file) and
// h, new-file lines from..to, as godemirror offers it; ok is false where it
// offers none, as when the two sides disagree on the run's length.
func gapBefore(prev *patchHunk, h patchHunk) (from, to int, ok bool) {
	from, oldFrom := 1, 1
	if prev != nil {
		from, oldFrom = prev.newStart+prev.newCount, prev.oldStart+prev.oldCount
	}
	to = h.newStart - 1
	return from, to, to >= from && h.oldStart-oldFrom == h.newStart-from
}

// diffRows is how many rows godemirror lays f out in, unified or side by
// side, with filled lines pulled into the gap ending at each key. It mirrors
// godemirror's diffSource.rows and split.
//
// ponytail: a copy of godemirror's layout rules; replace with a DiffView
// height once godemirror exposes one.
func diffRows(f patchFile, split bool, filled map[int]int) int {
	n := 0
	for i, h := range f.hunks {
		var prev *patchHunk
		if i > 0 {
			prev = &f.hunks[i-1]
		}
		from, to, ok := gapBefore(prev, h)
		switch {
		case ok:
			got := min(filled[to], to-from+1)
			n += got
			if got <= to-from {
				n++
			}
		case i > 0:
			n++
		}
		n += hunkRows(h.lines, split)
	}
	return n
}

// hunkRows counts a hunk's rows: one per line unified; side by side, a run
// of changes takes as many rows as its longer side.
func hunkRows(lines []string, split bool) int {
	n, dels, adds := 0, 0, 0
	flush := func() {
		if split {
			n += max(dels, adds)
		} else {
			n += dels + adds
		}
		dels, adds = 0, 0
	}
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, `\`):
		case strings.HasPrefix(l, "-"):
			dels++
		case strings.HasPrefix(l, "+"):
			adds++
		default:
			flush()
			n++
		}
	}
	flush()
	return n
}

// hideWhitespace is lib/git/whitespace.ts: a change block whose deletions
// and additions pair up one for one, each pair equal once whitespace is
// ignored, becomes unchanged lines showing the file as it is now; any other
// block is left whole. A hunk left with no change is dropped.
func hideWhitespace(f patchFile) patchFile {
	if f.binary {
		return f
	}
	out := f
	out.hunks = nil
	for _, h := range f.hunks {
		h.lines = foldHunk(h.lines)
		if hasChange(h.lines) {
			out.hunks = append(out.hunks, h)
		}
	}
	return out
}

// whitespaceOnly is a file every change of which was whitespace.
func whitespaceOnly(shown, original patchFile) bool {
	return len(shown.hunks) == 0 && len(original.hunks) > 0
}

func foldHunk(lines []string) []string {
	var out, block []string
	for _, l := range lines {
		if isChangeLine(l) {
			block = append(block, l)
			continue
		}
		out = append(out, foldBlock(block)...)
		block = nil
		out = append(out, l)
	}
	return append(out, foldBlock(block)...)
}

// foldBlock answers a block as unchanged lines when it only moved
// whitespace. git prints a block's deletions before its additions, so the
// i-th of each is the pair.
func foldBlock(block []string) []string {
	var dels, adds []string
	for _, l := range block {
		if l[0] == '-' {
			dels = append(dels, l)
		} else {
			adds = append(adds, l)
		}
	}
	if len(block) == 0 || len(dels) != len(adds) {
		return block
	}
	folded := make([]string, len(adds))
	for i := range adds {
		if squeeze(dels[i][1:]) != squeeze(adds[i][1:]) {
			return block
		}
		folded[i] = " " + adds[i][1:]
	}
	return folded
}

func squeeze(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func isChangeLine(l string) bool { return l != "" && (l[0] == '+' || l[0] == '-') }

func hasChange(lines []string) bool {
	for _, l := range lines {
		if isChangeLine(l) {
			return true
		}
	}
	return false
}

// discardTargets are the paths a discard reverts: the file, and for a rename
// the old path as well, which the discard restores.
func discardTargets(f patchFile) []string {
	if f.status == statusRenamed && f.oldPath != "" && f.oldPath != f.path {
		return []string{f.path, f.oldPath}
	}
	return []string{f.path}
}
