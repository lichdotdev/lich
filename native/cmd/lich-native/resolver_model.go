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

// newResolveModel lays the result out as the base: nothing is taken until the
// user takes it, a change only one side made included.
func newResolveModel(c api.GitConflict) (*resolveModel, string) {
	m := &resolveModel{base: splitLines(c.Base), ours: splitLines(c.Ours), theirs: splitLines(c.Theirs)}
	var doc strings.Builder
	at := 0
	for _, ch := range c.Chunks {
		doc.WriteString(joinLines(m.base, api.GitLines{From: at, To: ch.Base.From}))
		rc := resolveChunk{GitConflictChunk: ch, from: doc.Len()}
		doc.WriteString(joinLines(m.base, ch.Base))
		rc.to, at = doc.Len(), ch.Base.To
		m.chunks = append(m.chunks, rc)
	}
	doc.WriteString(joinLines(m.base, api.GitLines{From: at, To: len(m.base)}))
	out := doc.String()
	if !strings.HasSuffix(c.Base, "\n") {
		out = strings.TrimSuffix(out, "\n")
	}
	return m, out
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

// chunkState is how far the user has taken a chunk.
type chunkState uint8

const (
	// chunkPending holds the base's lines, undecided.
	chunkPending chunkState = iota
	// chunkApplied no longer holds them: a side taken, or typed over.
	chunkApplied
	// chunkDismissed keeps them on purpose, every side that changed them
	// left out.
	chunkDismissed
)

func (m *resolveModel) state(i int, doc string) chunkState {
	ch := m.chunks[i]
	switch {
	case doc[ch.from:ch.to] != joinLines(m.base, ch.Base):
		return chunkApplied
	case (ch.oursOff || ch.Kind == api.GitChunkTheirs) && (ch.theirsOff || ch.Kind == api.GitChunkOurs):
		return chunkDismissed
	}
	return chunkPending
}

// resolved is whether a chunk needs nothing more.
func (m *resolveModel) resolved(i int, doc string) bool { return m.state(i, doc) != chunkPending }

// unresolved counts the conflicts still open, which Apply waits on, and
// changes the changes only one side made that are neither taken nor left out.
func (m *resolveModel) unresolved(doc string) (conflicts, changes int) {
	for i, ch := range m.chunks {
		switch {
		case m.resolved(i, doc):
		case ch.Kind == api.GitChunkConflict:
			conflicts++
		default:
			changes++
		}
	}
	return conflicts, changes
}

// neighbor is the pending chunk nearest pos on one side of it: the first
// that starts after it, or with dir below zero the last that starts before.
func (m *resolveModel) neighbor(pos, dir int, doc string) (int, bool) {
	found, ok := 0, false
	for i, ch := range m.chunks {
		if m.resolved(i, doc) {
			continue
		}
		switch {
		case dir > 0 && ch.from > pos:
			return i, true
		case dir < 0 && ch.from < pos:
			found, ok = i, true
		}
	}
	return found, ok
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
