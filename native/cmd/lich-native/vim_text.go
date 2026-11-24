package main

// The editor's text as Neovim's buffer holds it, and back: a buffer is lines
// with no break after the last, the file's last "\n" being its 'eol'; rows
// count from 1 and columns are bytes from 0, as the editor's positions are.

import (
	"strings"
	"unicode/utf8"

	"github.com/lichdotdev/godemirror"
)

// vimLinesOf is doc as buffer lines, and whether it ends in a line break.
func vimLinesOf(doc string) ([]string, bool) {
	lines := strings.Split(doc, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1], true
	}
	return lines, false
}

// vimSplice is doc with its buffer lines first to last replaced by lines, as
// nvim_buf_lines_event reports a change; last -1 is the buffer's end.
func vimSplice(doc string, first, last int, lines []string) string {
	old, eol := vimLinesOf(doc)
	if last < 0 || last > len(old) {
		last = len(old)
	}
	first = min(max(first, 0), last)
	next := make([]string, 0, len(old)-(last-first)+len(lines))
	next = append(next, old[:first]...)
	next = append(next, lines...)
	next = append(next, old[last:]...)
	if len(next) == 0 {
		next = []string{""}
	}
	text := strings.Join(next, "\n")
	if eol {
		text += "\n"
	}
	return text
}

// vimChange is the one change taking from to to: what lies between their
// common start and common end.
func vimChange(from, to string) (godemirror.ChangeSpec, bool) {
	if from == to {
		return godemirror.ChangeSpec{}, false
	}
	pre := 0
	for pre < len(from) && pre < len(to) && from[pre] == to[pre] {
		pre++
	}
	for pre > 0 && pre < len(from) && !utf8.RuneStart(from[pre]) {
		pre--
	}
	suf := 0
	for suf < len(from)-pre && suf < len(to)-pre && from[len(from)-1-suf] == to[len(to)-1-suf] {
		suf++
	}
	for suf > 0 && !utf8.RuneStart(from[len(from)-suf]) {
		suf--
	}
	return godemirror.ChangeSpec{From: pre, To: len(from) - suf, Insert: to[pre : len(to)-suf]}, true
}

// vimPos is row, 1-based, at byte column col in st, held inside the text.
func vimPos(st godemirror.EditorState, row, col int) int {
	line := st.Line(min(max(row, 1), st.Lines()))
	return line.From + min(max(col, 0), len(line.Text))
}

// vimRowCol is pos as a buffer row, 1-based, and byte column.
func vimRowCol(st godemirror.EditorState, pos int) (int, int) {
	line := st.LineAt(pos)
	return line.Number, pos - line.From
}

// vimSelection is what the editor selects for s: the visual region, one
// range per row of a block, with its head where Vim's cursor is; outside
// Visual mode, the cursor alone. getregionpos reports a region a row at a
// time, which only a block keeps apart.
func vimSelection(st godemirror.EditorState, s vimState) godemirror.EditorSelection {
	cursor := vimPos(st, s.row, s.col)
	if len(s.regions) == 0 {
		return godemirror.SingleSelection(cursor, cursor)
	}
	if len(s.regions) > 1 && !strings.HasPrefix(s.mode, "\x16") {
		first, last := s.regions[0], s.regions[len(s.regions)-1]
		s.regions = [][4]int{{first[0], first[1], last[2], last[3]}}
	}
	ranges := make([]godemirror.SelectionRange, 0, len(s.regions))
	for _, r := range s.regions {
		from, to := vimPos(st, r[0], r[1]-1), vimPos(st, r[2], r[3])
		if strings.HasPrefix(s.mode, "V") {
			from, to = st.Line(r[0]).From, min(st.Line(min(r[2], st.Lines())).To+1, len(st.Doc()))
		}
		to = max(to, from)
		if cursor <= from && cursor < to {
			ranges = append(ranges, godemirror.Range(to, from))
		} else {
			ranges = append(ranges, godemirror.Range(from, to))
		}
	}
	return godemirror.NewSelection(ranges, len(ranges)-1)
}

// vimClickCol is where Vim's cursor goes for a selection's head: on the last
// character selected, which Vim's cursor covers where the editor's caret
// stands after it.
func vimClickCol(st godemirror.EditorState, anchor, head int) (int, int) {
	if head > anchor {
		_, size := utf8.DecodeLastRuneInString(st.Doc()[:head])
		head -= size
	}
	return vimRowCol(st, head)
}
