package main

import (
	"image"
	"testing"
)

func TestPaneTargetRoundTrips(t *testing.T) {
	for _, edge := range []paneEdge{edgeLeft, edgeRight, edgeTop, edgeBottom, edgeCenter} {
		session, got, ok := parsePaneTarget(paneTargetID("s|1", edge))
		if !ok || session != "s|1" || got != edge {
			t.Errorf("edge %d: got %q %d %v", edge, session, got, ok)
		}
	}
	for _, id := range []string{"", "worktree-key", "pane|", "pane|s|9", "pane|s|x", "pane||1"} {
		if _, _, ok := parsePaneTarget(id); ok {
			t.Errorf("%q read as a drop zone", id)
		}
	}
}

func TestPaneZonesTileThePane(t *testing.T) {
	r := image.Rect(10, 20, 410, 220)
	z := paneZones(r)
	want := map[paneEdge]image.Rectangle{
		edgeLeft:   image.Rect(10, 20, 110, 220),
		edgeRight:  image.Rect(310, 20, 410, 220),
		edgeTop:    image.Rect(110, 20, 310, 70),
		edgeBottom: image.Rect(110, 170, 310, 220),
		edgeCenter: image.Rect(110, 70, 310, 170),
	}
	area := 0
	for edge, w := range want {
		if z[edge] != w {
			t.Errorf("edge %d: got %v, want %v", edge, z[edge], w)
		}
		area += w.Dx() * w.Dy()
	}
	if area != r.Dx()*r.Dy() {
		t.Fatalf("zones cover %d of %d", area, r.Dx()*r.Dy())
	}
	if got := dropPreview(r, edgeRight); got != image.Rect(210, 20, 410, 220) {
		t.Errorf("right preview %v", got)
	}
	if got := dropPreview(r, edgeTop); got != image.Rect(10, 20, 410, 120) {
		t.Errorf("top preview %v", got)
	}
}

// A pane of the layout carried over another swaps through its middle; a
// pane too narrow for two side by side offers only above and below; a card
// gets no middle; the pane carried is no target.
func TestZoneAtOffersOnlyWhatFits(t *testing.T) {
	d := dragPointer{
		last:  map[string]image.Rectangle{"wide": image.Rect(0, 0, 1000, 600), "narrow": image.Rect(1006, 0, 1506, 600)},
		was:   map[string]bool{},
		paneW: 480, paneH: 180, gap: 6,
	}
	cases := []struct {
		at      image.Point
		carried string
		want    string
	}{
		{image.Pt(500, 300), "narrow", paneTargetID("wide", edgeCenter)},
		{image.Pt(990, 300), "narrow", paneTargetID("wide", edgeRight)},
		{image.Pt(1490, 310), "wide", paneTargetID("narrow", edgeBottom)},
		{image.Pt(1256, 310), "card", paneTargetID("narrow", edgeBottom)},
		{image.Pt(500, 310), "card", paneTargetID("wide", edgeBottom)},
		{image.Pt(500, 300), "wide", ""},
		{image.Pt(1003, 300), "card", ""},
	}
	for _, c := range cases {
		if got := d.zoneAt(c.at, c.carried); got != c.want {
			t.Errorf("at %v carrying %q: got %q, want %q", c.at, c.carried, got, c.want)
		}
	}
	d.was["wide"] = true
	if got := d.zoneAt(image.Pt(500, 250), "narrow"); got != paneTargetID("wide", edgeTop) {
		t.Errorf("a session alone offered %q, want no middle", got)
	}
}

func TestPickZoneGivesALeftOutEdgeToTheOthers(t *testing.T) {
	r := image.Rect(0, 0, 400, 400)
	stackedOnly := [edgeCenter + 1]bool{edgeTop: true, edgeBottom: true}
	cases := []struct {
		at      image.Point
		allowed [edgeCenter + 1]bool
		want    paneEdge
		ok      bool
	}{
		{image.Pt(200, 300), stackedOnly, edgeBottom, true},
		{image.Pt(10, 150), stackedOnly, edgeTop, true},
		{image.Pt(200, 200), [edgeCenter + 1]bool{true, true, true, true, true}, edgeCenter, true},
		{image.Pt(395, 200), [edgeCenter + 1]bool{true, true, true, true, true}, edgeRight, true},
		{image.Pt(200, 200), [edgeCenter + 1]bool{}, 0, false},
	}
	for _, c := range cases {
		got, ok := pickZone(r, c.at, c.allowed)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("at %v: got %d %v, want %d %v", c.at, got, ok, c.want, c.ok)
		}
	}
}

func TestColumnRoomCountsThePanesThatFit(t *testing.T) {
	d := dragPointer{
		last:  map[string]image.Rectangle{"wide": image.Rect(0, 0, 1000, 600), "narrow": image.Rect(0, 0, 900, 600)},
		paneW: 480, paneH: 180, gap: 6,
	}
	cases := []struct {
		target string
		below  bool
		want   int
	}{
		{"wide", false, 3},
		{"narrow", false, 0},
		{"narrow", true, 2},
		{"missing", false, 0},
	}
	for _, c := range cases {
		if got := d.columnRoom(c.target, c.below); got != c.want {
			t.Errorf("columnRoom(%s, below %v) = %d, want %d", c.target, c.below, got, c.want)
		}
	}
}
