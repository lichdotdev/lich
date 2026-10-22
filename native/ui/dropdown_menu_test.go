package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Dropdown geometry at 1px per dp: the panel opens 4px below the trigger,
// its rows start after p-1, and an item is py-2 around a 20px line.
const (
	dropdownTop   = 30 + 30 + 4
	dropdownRow   = 36
	dropdownWidth = 128 // min-w-32
)

func dropdownRowCenter(i int) image.Point {
	return image.Pt(triggerAt.X+20, dropdownTop+4+dropdownRow*i+dropdownRow/2)
}

func openDropdown(t *testing.T, entries []MenuEntry) (*menuRig, *MenuState, func()) {
	t.Helper()
	g, s, th := &menuRig{size: image.Pt(400, 400)}, new(MenuState), testTheme()
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D { return th.DropdownMenu(&g.root, s, entries).Layout(gtx, trigger) })
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	if !s.Opened() {
		t.Fatal("press on the trigger did not open the menu")
	}
	return g, s, frame
}

func TestDropdownItemClickRunsItAndCloses(t *testing.T) {
	var c chosen
	g, s, frame := openDropdown(t, []MenuEntry{c.item("Rename"), c.item("Pin")})
	g.click(pointer.ButtonPrimary, dropdownRowCenter(1))
	frame()
	c.only(t, "Pin")
	if s.Opened() {
		t.Error("menu still open after choosing an item")
	}
}

func TestDropdownPanelIsMinWidthWide(t *testing.T) {
	var c chosen
	g, _, frame := openDropdown(t, []MenuEntry{c.item("A")})
	g.click(pointer.ButtonPrimary, image.Pt(triggerAt.X+dropdownWidth-6, dropdownRowCenter(0).Y))
	frame()
	c.only(t, "A")
}

func TestDropdownOutsidePressClosesWithoutChoosing(t *testing.T) {
	var c chosen
	g, s, frame := openDropdown(t, []MenuEntry{c.item("A")})
	g.click(pointer.ButtonPrimary, image.Pt(triggerAt.X+dropdownWidth+20, dropdownRowCenter(0).Y))
	frame()
	if s.Opened() || len(c) > 0 {
		t.Errorf("open %v, chosen %q: want closed, nothing chosen", s.Opened(), []string(c))
	}
}

func TestDropdownEscapeCloses(t *testing.T) {
	var c chosen
	g, s, frame := openDropdown(t, []MenuEntry{c.item("A")})
	g.press(key.NameEscape)
	frame()
	if s.Opened() {
		t.Error("menu still open after Escape")
	}
}

func TestDropdownKeyboardNavigation(t *testing.T) {
	cases := []struct {
		keys []key.Name
		want string
	}{
		{[]key.Name{key.NameDownArrow, key.NameDownArrow}, "Copy"},
		{[]key.Name{key.NameUpArrow}, "Close"},
		{[]key.Name{key.NameEnd, key.NameDownArrow}, "Rename"},
		{[]key.Name{key.NameEnd, key.NameHome}, "Rename"},
	}
	for _, tc := range cases {
		var c chosen
		closeItem := c.item("Close")
		closeItem.Variant = VariantDestructive
		entries := []MenuEntry{c.item("Rename"), {Kind: MenuSeparator}, {Label: "Fork", Disabled: true}, c.item("Copy"), closeItem}
		g, s, frame := openDropdown(t, entries)
		g.press(append(tc.keys, key.NameReturn)...)
		frame()
		c.only(t, tc.want)
		if s.Opened() {
			t.Errorf("%v: menu still open after Enter", tc.keys)
		}
	}
}

func TestDropdownCheckboxTogglesAndStaysOpen(t *testing.T) {
	var got []bool
	entries := []MenuEntry{{Kind: MenuCheckboxItem, Label: "Confined", OnCheckedChange: func(v bool) { got = append(got, v) }}}
	g, s, frame := openDropdown(t, entries)
	g.press(key.NameDownArrow, key.NameSpace)
	frame()
	if len(got) != 1 || !got[0] || !s.Opened() {
		t.Errorf("got %v, open %v: want [true] and the menu open", got, s.Opened())
	}
}

func TestDropdownSubmenuFromKeyboard(t *testing.T) {
	var c chosen
	entries := []MenuEntry{c.item("Rename"), {Kind: MenuSubTrigger, Label: "Open in", Sub: []MenuEntry{c.item("Terminal"), c.item("Editor")}}}
	g, s, frame := openDropdown(t, entries)
	g.press(key.NameDownArrow, key.NameDownArrow, key.NameRightArrow, key.NameDownArrow, key.NameLeftArrow, key.NameRightArrow, key.NameReturn)
	frame()
	c.only(t, "Terminal")
	if s.Opened() {
		t.Error("menu still open after choosing a submenu item")
	}
}

func TestDropdownSubmenuOpensOnHover(t *testing.T) {
	var c chosen
	entries := []MenuEntry{{Kind: MenuSubTrigger, Label: "Open in", Sub: []MenuEntry{c.item("Terminal")}}}
	g, _, frame := openDropdown(t, entries)
	g.hover(dropdownRowCenter(0))
	frame()
	// The submenu sits right of the 128px panel, its first row 3px above the
	// trigger row's top (alignOffset -3) plus its own p-1.
	trigger := image.Pt(triggerAt.X+dropdownWidth+20, dropdownTop+4-3+4+16)
	g.click(pointer.ButtonPrimary, trigger)
	frame()
	c.only(t, "Terminal")
}

// A row that closes the panel does so as the panel is drawn, so the frame
// that took its click still draws it open; Gio draws no other until the next
// event, so that frame must ask for one.
func TestDropdownPanelRowClosingItAsksForTheNextFrame(t *testing.T) {
	g, th := &menuRig{size: image.Pt(400, 400)}, testTheme()
	var s DropdownPanelState
	var row widget.Clickable
	now := time.Unix(0, 0)
	frame := func() (redraw bool) {
		g.ops.Reset()
		gtx := layout.Context{Ops: &g.ops, Source: g.router.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(g.size)}
		g.root.Layout(gtx, func(gtx C) D {
			defer op.Offset(triggerAt).Push(gtx.Ops).Pop()
			return th.DropdownPanel(&g.root, &s, dropdownWidth, func(gtx C, th *Theme) D {
				if row.Clicked(gtx) {
					s.Close()
				}
				return row.Layout(gtx, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, dropdownRow)} })
			}).Layout(gtx, func(gtx C) D { return D{Size: triggerSize} })
		})
		g.router.Frame(&g.ops)
		at, wake := g.router.WakeupTime()
		return wake && !at.After(now)
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	for frame() {
		now = now.Add(time.Second)
	}
	g.click(pointer.ButtonPrimary, dropdownRowCenter(0))
	redraw := frame()
	if s.Opened() {
		t.Fatal("the row did not close the panel")
	}
	if !redraw {
		t.Error("the frame that closed the panel asked for no next frame")
	}
}
