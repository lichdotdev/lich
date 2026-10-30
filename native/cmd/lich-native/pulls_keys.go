package main

import (
	"slices"

	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/native/backend"
)

// keys carries out the screen's keys: on the index, j/k or the arrows move
// the cursor, Enter opens its pull request and / goes to the field; on a pull
// request, Escape goes back to the index and Alt with an arrow steps along it.
// They are read only while nothing holds the keyboard and no dialog is up, so
// a field, a menu or a dialog keeps its own keys.
func (p *pullsScreen) keys(gtx C, rows []backend.PRSummary) {
	if !gtx.Source.Focused(nil) || p.view.dialogUp() {
		return
	}
	filters := []event.Filter{
		key.Filter{Name: key.NameEscape},
		key.Filter{Name: key.NameUpArrow, Required: key.ModAlt},
		key.Filter{Name: key.NameDownArrow, Required: key.ModAlt},
	}
	if p.selected == 0 {
		filters = []event.Filter{
			key.Filter{Name: "J"}, key.Filter{Name: "K"},
			key.Filter{Name: key.NameDownArrow}, key.Filter{Name: key.NameUpArrow},
			key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameEnter},
			key.Filter{Name: "/"},
		}
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			p.key(gtx, e.Name, p.visible(rows))
		}
	}
}

func (p *pullsScreen) key(gtx C, name key.Name, rows []backend.PRSummary) {
	if p.selected != 0 {
		switch name {
		case key.NameEscape:
			p.backToIndex()
		case key.NameUpArrow:
			p.stepPR(rows, -1)
		case key.NameDownArrow:
			p.stepPR(rows, 1)
		}
		return
	}
	switch name {
	case "J", key.NameDownArrow:
		p.moveCursor(rows, 1)
	case "K", key.NameUpArrow:
		p.moveCursor(rows, -1)
	case key.NameReturn, key.NameEnter:
		if p.cursorAt(rows) >= 0 {
			p.openPR(p.cursor)
		}
	case "/":
		gtx.Execute(key.FocusCmd{Tag: &p.query})
	}
}

// moveCursor moves the index's cursor delta rows and keeps its row in view.
func (p *pullsScreen) moveCursor(rows []backend.PRSummary, delta int) {
	p.cursor = stepAlong(rows, p.cursor, delta)
	i := p.cursorAt(rows)
	if pos := p.rows.Position; i >= 0 && (i <= pos.First || i >= pos.First+pos.Count-1) {
		p.rows.ScrollTo(max(i-delta, 0))
	}
}

// stepPR opens the pull request delta rows from the one in view, along the
// index's filter; one the index does not show has nowhere to step from.
func (p *pullsScreen) stepPR(rows []backend.PRSummary, delta int) {
	if position(rows, p.selected) < 0 {
		return
	}
	if n := stepAlong(rows, p.selected, delta); n != p.selected {
		p.openPR(n)
	}
}

// position is pull request n's place in rows, -1 when it is not there.
func position(rows []backend.PRSummary, n int) int {
	return slices.IndexFunc(rows, func(r backend.PRSummary) bool { return r.Number == n })
}

// stepAlong is the number of the row delta rows from n, held at the ends; the
// first row when n is not among them, 0 when there are none.
func stepAlong(rows []backend.PRSummary, n, delta int) int {
	if len(rows) == 0 {
		return 0
	}
	i := position(rows, n)
	if i < 0 {
		return rows[0].Number
	}
	return rows[min(max(i+delta, 0), len(rows)-1)].Number
}

// dialogUp is whether one of the screen's dialogs is open, which takes
// Escape for itself.
func (v *pullView) dialogUp() bool {
	return v.submit.verdict != "" || v.mergeEdit.open || v.talk.deleting != nil
}
