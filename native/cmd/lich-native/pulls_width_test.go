package main

import (
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The list keeps pullsListWidth in English and grows only as far as the
// filter tabs need in a language whose labels run longer.
func TestPullsListGrowsForLongerFilterLabels(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(2000, 1000)}}
	p := new(pullsScreen)
	counts := [pullsFilterCount]int{12, 3, 1, 2}
	if w := p.listWidth(gtx, th, counts); w != int(pullsListWidth) {
		t.Errorf("English: %d, want %v", w, pullsListWidth)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	w := p.listWidth(gtx, th, counts)
	need := p.filterTabs(th, counts).NaturalWidth(gtx) + 2*int(filterTrackInset)
	if w <= int(pullsListWidth) || w != min(need, int(pullsListMaxWidth)) {
		t.Errorf("Portuguese: %d, want the %d the tabs need (bounded by %v)", w, need, pullsListMaxWidth)
	}
}
