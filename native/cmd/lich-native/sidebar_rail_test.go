package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

func railShape(items []railItem) []string {
	var out []string
	for _, it := range items {
		s := [...]string{railCard: "card", railCheckout: "checkout", railHead: "head", railGap: "gap"}[it.kind]
		if it.sv != nil {
			s += ":" + it.sv.ID
		}
		if it.small {
			s += " small"
		}
		if it.up {
			s += " ^"
		}
		if it.down {
			s += " v"
		}
		if it.onTray {
			s += " tray"
		}
		out = append(out, s)
	}
	return out
}

func TestRailDrawsTheTreeInOneColumn(t *testing.T) {
	main := blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{
		treeSession("mother", "", "/repo"), treeSession("child", "mother", "/repo"), treeSession("solo", "", "/repo"),
	}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, byCheckout: true, color: "pink", sessions: []sessionView{
		treeSession("a", "", "/wt/a"), treeSession("b", "", "/wt/b"),
	}}
	got := railShape(railItems([]blockView{main, folder}))
	want := []string{
		"checkout v", "card:mother ^ v", "card:child small ^ v", "card:solo ^",
		"gap", "head tray", "checkout v tray", "card:a ^ tray", "checkout v tray", "card:b ^ tray",
	}
	if !slices.Equal(got, want) {
		t.Errorf("rail = %v\nwant   %v", got, want)
	}
}

func TestRailLeavesAFoldedBlockItsHead(t *testing.T) {
	b := blockView{key: "/repo", kind: blockCheckout, under: "/repo", folded: true, sessions: []sessionView{treeSession("s", "", "/repo")}}
	if got := railShape(railItems([]blockView{b})); !slices.Equal(got, []string{"checkout"}) {
		t.Errorf("rail = %v, want the checkout's node alone", got)
	}
}

func TestMorphNamesEveryNodeBothShapesDraw(t *testing.T) {
	main := blockView{key: "/repo", kind: blockCheckout, under: "/repo", sessions: []sessionView{treeSession("s", "", "/repo")}}
	folder := blockView{key: folderKey("Later"), kind: blockFolder, byCheckout: true, sessions: []sessionView{treeSession("a", "", "/wt/a")}}
	nodes := nodeGlyphs(&ui.Theme{Palette: ui.Dark}, []blockView{main, folder})
	for _, it := range railItems([]blockView{main, folder}) {
		if it.kind == railCheckout || it.kind == railHead {
			if _, ok := nodes[glyphNode+it.key]; !ok {
				t.Errorf("the rail's node %q has no flying glyph", it.key)
			}
		}
	}
}
