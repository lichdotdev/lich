package main

// The conflict resolver's model: the result the middle pane starts from, where
// each chunk of the conflict sits in it as the user edits, and what taking a
// side writes there.

import (
	"strings"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
)

// resolveChunk is a chunk of the conflict and the bytes of the result it
// spans, kept current through every edit.
type resolveChunk struct {
	api.GitConflictChunk
	from, to int
	// oursOff and theirsOff are a side the user dismissed with ✕.
	oursOff, theirsOff bool
}

type resolveModel struct {
	base, ours, theirs []string
	chunks             []resolveChunk
}

// splitLines cuts text into lines that each keep their line break, the last
// one given one when the text lacks it, so lines join back without a seam.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	} else {
		lines[len(lines)-1] += "\n"
	}
	return lines
}

func joinLines(lines []string, r api.GitLines) string {
	return strings.Join(lines[r.From:r.To], "")
}

// newResolveModel lays the result out: the base, with each chunk only one
// side changed (or both alike) already taken from that side, and each
// conflict holding the base's lines until the user decides.
func newResolveModel(c api.GitConflict) (*resolveModel, string) {
	m := &resolveModel{base: splitLines(c.Base), ours: splitLines(c.Ours), theirs: splitLines(c.Theirs)}
	var doc strings.Builder
	at := 0
	for _, ch := range c.Chunks {
		doc.WriteString(joinLines(m.base, api.GitLines{From: at, To: ch.Base.From}))
		rc := resolveChunk{GitConflictChunk: ch, from: doc.Len()}
		doc.WriteString(m.initial(ch))
		rc.to, at = doc.Len(), ch.Base.To
		m.chunks = append(m.chunks, rc)
	}
	doc.WriteString(joinLines(m.base, api.GitLines{From: at, To: len(m.base)}))
	out := doc.String()
	if !strings.HasSuffix(c.Ours, "\n") && strings.HasSuffix(out, "\n") {
		out = strings.TrimSuffix(out, "\n")
	}
	return m, out
}

func (m *resolveModel) initial(ch api.GitConflictChunk) string {
	switch ch.Kind {
	case api.GitChunkOurs, api.GitChunkBoth:
		return joinLines(m.ours, ch.Ours)
	case api.GitChunkTheirs:
		return joinLines(m.theirs, ch.Theirs)
	}
	return joinLines(m.base, ch.Base)
}

func (m *resolveModel) sideText(i int, ours bool) string {
	if ours {
		return joinLines(m.ours, m.chunks[i].Ours)
	}
	return joinLines(m.theirs, m.chunks[i].Theirs)
}

// applied is whether a side's lines are in the chunk; a side that deleted
// them is applied once the chunk is empty.
func (m *resolveModel) applied(i int, ours bool, doc string) bool {
	cur, side := doc[m.chunks[i].from:m.chunks[i].to], m.sideText(i, ours)
	if side == "" {
		return cur == ""
	}
	return strings.Contains(cur, side)
}

// resolved is whether a chunk needs nothing more: one no conflict made, one
// whose lines are no longer the base's (a side taken, or typed over), or one
// whose two sides were both dismissed, keeping the base.
func (m *resolveModel) resolved(i int, doc string) bool {
	ch := m.chunks[i]
	if ch.Kind != api.GitChunkConflict {
		return true
	}
	return doc[ch.from:ch.to] != joinLines(m.base, ch.Base) || ch.oursOff && ch.theirsOff
}

func (m *resolveModel) unresolved(doc string) int {
	n := 0
	for i := range m.chunks {
		if !m.resolved(i, doc) {
			n++
		}
	}
	return n
}

// take writes a side into a chunk: in place of the base's lines while they
// are still there, else after what the chunk holds, so taking both sides
// stacks them in the order taken.
func (m *resolveModel) take(i int, ours bool, doc string) godemirror.ChangeSpec {
	ch := m.chunks[i]
	cur, side := doc[ch.from:ch.to], m.sideText(i, ours)
	if cur != joinLines(m.base, ch.Base) {
		side = cur + side
	}
	return godemirror.ChangeSpec{From: ch.from, To: ch.to, Insert: side}
}

// remap follows an edit: text typed at the start of a chunk's first line is
// the chunk's, at the start of the line after it is not, and an empty chunk
// takes what is typed into it.
func (m *resolveModel) remap(changes godemirror.ChangeSet) {
	for i := range m.chunks {
		ch := &m.chunks[i]
		empty := ch.from == ch.to
		ch.from = changes.MapPos(ch.from, -1)
		if empty {
			ch.to = changes.MapPos(ch.to, 1)
		} else {
			ch.to = max(changes.MapPos(ch.to, -1), ch.from)
		}
	}
}
