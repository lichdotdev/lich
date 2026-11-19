package main

// The stage editor's text against the disk: the line break a file keeps,
// which of its lines the user changed since it was read, and what a newer
// read of the disk means for what the user holds.

import (
	"slices"
	"strings"
)

// maxChangedCells bounds the table changedLines compares two stretches in:
// past it the stretch between the first and the last change is marked whole.
const maxChangedCells = 1 << 20

// lineBreak is the break a file's text uses, "" for "\n": a file read with
// CRLF is written back with it.
func lineBreak(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	return ""
}

// normalizedText is text in the lines the editor holds it in, as setText
// splits it: on CRLF alone in a CRLF file, on any break in another.
func normalizedText(text string) string {
	if lineBreak(text) == "\r\n" {
		return strings.ReplaceAll(text, "\r\n", "\n")
	}
	return strings.ReplaceAll(text, "\r", "\n")
}

// changedLines are the lines of doc, counted from 0, that are not base's:
// typed, pasted or replaced since base was read. Where lines were only
// deleted, the line that took their place is marked, so a deletion shows too.
func changedLines(base, doc string) []int {
	if base == doc {
		return nil
	}
	a, b := strings.Split(base, "\n"), strings.Split(doc, "\n")
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	marks := middleMarks(a[pre:len(a)-suf], b[pre:len(b)-suf])
	out := make([]int, 0, len(marks))
	for _, i := range marks {
		out = append(out, min(pre+i, len(b)-1))
	}
	return slices.Compact(out)
}

// middleMarks marks the lines of b, from 0, that a longest common
// subsequence with a leaves out, and the place of each run of a's lines b
// lacks.
func middleMarks(a, b []string) []int {
	n, m := len(a), len(b)
	if n*m > maxChangedCells || n == 0 || m == 0 {
		if m == 0 {
			return []int{0}
		}
		marks := make([]int, m)
		for i := range marks {
			marks[i] = i
		}
		return marks
	}
	// lcs[i][j] is the longest common subsequence of a[i:] and b[j:].
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var marks []int
	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && a[i] == b[j] {
			i, j = i+1, j+1
			continue
		}
		// A run of changes: the lines it adds, or the place of the lines it
		// only deletes.
		at, added := j, false
		for (i < n || j < m) && !(i < n && j < m && a[i] == b[j]) {
			if j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]) {
				marks, added = append(marks, j), true
				j++
				continue
			}
			i++
		}
		if !added {
			marks = append(marks, at)
		}
	}
	return slices.Compact(marks)
}

// diskVerdict is what a newer read of the disk does to the editor.
type diskVerdict int

const (
	// diskSame is the file as the editor read it, or as the editor holds it.
	diskSame diskVerdict = iota
	// diskReload replaces the text: the user changed nothing since.
	diskReload
	// diskAsk warns over the text: the user's changes and the disk's differ.
	diskAsk
)

// diskVerdictOf weighs disk against the text the editor read, base, and the one
// it holds, doc.
func diskVerdictOf(base, doc, disk string) diskVerdict {
	switch {
	case disk == base, disk == doc:
		return diskSame
	case doc == base:
		return diskReload
	}
	return diskAsk
}
