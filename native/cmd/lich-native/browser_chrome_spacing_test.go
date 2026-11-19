package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

// A page pane's chrome keeps one gap, Space(2), between the pane's top edge and
// the tabs, the tabs and the address, the address and the page's card, and
// from the card to the pane's sides and foot. The bounds are what the pane drew:
// the front tab's and the address pill's, as the frame's semantic tree reports
// them, and the card's, which browserPaneAt lays the page out in.
func TestPageChromeKeepsOneGapBetweenItsRows(t *testing.T) {
	st := api.BrowserState{Open: true, Tabs: []api.BrowserTab{{ID: "a", Title: "Alpha", Front: true}}}
	u := &window{browserZoom: defaultZoom}
	p := &browserPaneUI{queue: newBrowserQueue(), copied: make(chan string, 1), notes: newBrowserNotes()}
	u.browserPanes = map[string]*browserPaneUI{"s": p}
	size := image.Pt(700, 400)
	var card image.Rectangle
	rig := newFrameRig(size, func(gtx C, th *ui.Theme) {
		u.theme = th
		var head, bar int
		head, bar, card = browserChrome(gtx, size)
		header := gtx
		header.Constraints = layout.Exact(image.Pt(size.X, head))
		u.browserHeader(header, p, "s", st, true)
		toolbar := gtx
		toolbar.Constraints = layout.Exact(image.Pt(size.X, bar))
		defer op.Offset(image.Pt(0, head)).Push(gtx.Ops).Pop()
		u.browserToolbar(toolbar, p, "s", st)
	})
	rig.settle(t)

	tab, ok := rig.smallestClickAt(rig.glyph(t, "Alpha"))
	if !ok {
		t.Fatal("the front tab is not clickable")
	}
	pill := rig.boxAround(t, "Search or enter address", size)

	const gap = 8 // Space(2) at one pixel per dp
	for name, got := range map[string]int{
		"pane top to the tabs":    tab.Min.Y,
		"pane side to the tabs":   tab.Min.X,
		"tabs to the address":     pill.Min.Y - tab.Max.Y,
		"address to the card":     card.Min.Y - pill.Max.Y,
		"pane side to the card":   card.Min.X,
		"card to the pane's side": size.X - card.Max.X,
		"card to the pane's foot": size.Y - card.Max.Y,
	} {
		if got != gap {
			t.Errorf("%s is %dpx, want %d", name, got, gap)
		}
	}
}
