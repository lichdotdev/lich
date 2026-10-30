package main

import "time"

// blockMove is a checkout block coming in or leaving by the tree's motion:
// coming in, its header row opens, and its rows follow down the line once
// the header's ring is drawing; leaving, its rows go first, then its header.
type blockMove struct {
	start   time.Time
	leaving bool
	rows    int // the rows under the header
}

const (
	// blockRowsAfter is how long a block's rows wait on its header coming in.
	blockRowsAfter = 300 * time.Millisecond
	// blockHeadAfter is how far into its rows leaving the header starts to go.
	blockHeadAfter = 0.85
)

// headRun is the header's run, one root row.
func (m blockMove) headRun() treeRun {
	head := treeRun{rows: []sessionRow{{}}, start: m.start, closing: m.leaving}
	if m.leaving {
		head.start = m.start.Add(time.Duration(float64(m.rowsRun().length()) * blockHeadAfter))
	}
	return head
}

// rowsRun is the rows' run, as long as theirs.
func (m blockMove) rowsRun() treeRun {
	return treeRun{rows: make([]sessionRow, max(m.rows, 1)), start: m.start, closing: m.leaving}
}

// end is when the block's motion has played.
func (m blockMove) end() time.Time {
	head := m.headRun()
	if m.leaving {
		return head.start.Add(head.length())
	}
	return m.start.Add(max(head.length(), blockRowsAfter+m.rowsRun().length()))
}

// head is the header's share of the motion at now.
func (m blockMove) head(now time.Time) rowMotion { return m.headRun().frame(now, 0) }
