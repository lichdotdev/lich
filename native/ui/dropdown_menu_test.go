package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
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
	var row Clickable
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

func TestOverlayCountSeesAnOpenMenu(t *testing.T) {
	var c chosen
	g, s, th := &menuRig{size: image.Pt(400, 400)}, new(MenuState), testTheme()
	th.Overlays = &OverlayCount{}
	frame := func() bool {
		th.Overlays.Frame()
		g.frame(func(gtx C, trigger layout.Widget) D {
			return th.DropdownMenu(&g.root, s, []MenuEntry{c.item("A")}).Layout(gtx, trigger)
		})
		th.Overlays.Frame()
		return th.Overlays.Open()
	}
	if frame() {
		t.Fatal("a closed menu counted as open")
	}
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	if !frame() {
		t.Fatal("an open menu did not count")
	}
	g.press(key.NameEscape)
	frame()
	if frame() {
		t.Fatal("a closed menu still counts")
	}
}

func TestMenuSpotGetsTheRowAtItsSize(t *testing.T) {
	var c chosen
	var got image.Point
	b := c.item("B")
	b.Spot = func(gtx C, row layout.Widget) D {
		d := row(gtx)
		got = d.Size
		return d
	}
	_, _, frame := openDropdown(t, []MenuEntry{c.item("A"), b})
	frame()
	if got.X < dropdownWidth-8 || got.Y != dropdownRow {
		t.Fatalf("spot got a row of %v, want about %dx%d", got, dropdownWidth-8, dropdownRow)
	}
}

// At the window's right edge a submenu flips to the left of its parent and
// sits beside it, mirroring where it opens on the right: never over it.
func TestDropdownSubmenuFlippedLeftSitsBesideItsParent(t *testing.T) {
	defer func(at image.Point) { triggerAt = at }(triggerAt)
	triggerAt = image.Pt(400-triggerSize.X, 30)
	entries := []MenuEntry{{Label: "Settings"}, {Kind: MenuSubTrigger, Label: "Guides", Sub: []MenuEntry{{Label: "The basics"}, {Label: "Worktrees", Disabled: true, Shortcut: "Open a git project first"}}}}
	g, s, frame := openDropdown(t, entries)
	parent := s.levels[0].rows[1]
	g.hover(parent.Min.Add(image.Pt(20, 10)))
	frame()
	frame()
	if len(s.levels) != 2 {
		t.Fatalf("hovering the trigger row opened %d levels, want 2", len(s.levels))
	}
	parent = s.levels[0].rows[1]
	// A panel's edge is p-1 outside its rows.
	parentLeft, subRight := parent.Min.X-4, s.levels[1].rows[0].Max.X+4
	if subRight != parentLeft-4 {
		t.Errorf("submenu ends at x %d, want %d: 4px left of its parent's edge at %d", subRight, parentLeft-4, parentLeft)
	}
	if top := s.levels[1].rows[0].Min.Y; top != parent.Min.Y-3+4 {
		t.Errorf("submenu's first row at y %d, want %d: alignOffset -3 from the trigger row plus p-1", top, parent.Min.Y-3+4)
	}
}

// openPanel opens a DropdownPanel of the given content in a window h tall,
// its trigger at triggerAt.
func openPanel(t *testing.T, h int, above bool, content func(gtx C, th *Theme) D) (*menuRig, *DropdownPanelState, func()) {
	t.Helper()
	g, th := &menuRig{size: image.Pt(400, h)}, testTheme()
	var s DropdownPanelState
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D {
			p := th.DropdownPanel(&g.root, &s, dropdownWidth, content)
			if above {
				p = p.Above()
			}
			return p.Layout(gtx, trigger)
		})
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	frame()
	if !s.Opened() {
		t.Fatal("press on the trigger did not open the panel")
	}
	return g, &s, frame
}

// A panel that asks for more than the room on the side it opens on is laid
// out in that room and cut to it: it is no taller than the room, and what it
// asks for beyond that scrolls. It does not move over its trigger.
func TestDropdownPanelIsCutToTheRoomOnItsSide(t *testing.T) {
	var gotMax int
	// Below the trigger (30..60) of a 100px window: 100 less the trigger's
	// bottom, its 4px gap and the 5px window padding is 31px of room.
	g, s, frame := openPanel(t, 100, false, func(gtx C, th *Theme) D {
		// The first sight of it each frame, before it is put in a scroll area.
		if gotMax == 0 || gtx.Constraints.Max.Y < gotMax {
			gotMax = gtx.Constraints.Max.Y
		}
		return D{Size: image.Pt(gtx.Constraints.Max.X, 500)}
	})
	if gotMax != 31-8 {
		t.Errorf("the content was given %dpx, want the room less the panel's padding (23)", gotMax)
	}
	g.click(pointer.ButtonPrimary, image.Pt(triggerAt.X+10, dropdownTop+30))
	frame()
	if !s.Opened() {
		t.Error("a press inside the cut panel closed it")
	}
	g.click(pointer.ButtonPrimary, image.Pt(triggerAt.X+10, dropdownTop+34))
	frame()
	if s.Opened() {
		t.Error("the panel runs past the room it was cut to: a press below it did not close it")
	}
}

// Content that keeps to the height it is given is not wrapped in a scroll
// area, and a panel that opens above stands its bottom edge a gap over the
// trigger.
func TestDropdownPanelAboveStandsAGapOverItsTrigger(t *testing.T) {
	g, s, frame := openPanel(t, 400, true, func(gtx C, th *Theme) D {
		return D{Size: image.Pt(gtx.Constraints.Max.X, 20)}
	})
	// The trigger is high in the window, with no room over it: the panel
	// takes the room below, and does not cover the trigger.
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	if s.Opened() {
		t.Error("a press on the trigger reached the panel: it covers the trigger")
	}
}

// A menu with more rows than the room has is cut to the room, and the wheel
// scrolls its rows under the pointer.
func TestDropdownMenuTallerThanTheRoomScrolls(t *testing.T) {
	var c chosen
	var entries []MenuEntry
	for _, name := range []string{"R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9", "R10", "R11"} {
		entries = append(entries, c.item(name))
	}
	g, s, th := &menuRig{size: image.Pt(400, 200)}, new(MenuState), testTheme()
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D { return th.DropdownMenu(&g.root, s, entries).Layout(gtx, trigger) })
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	frame()
	// The room under the trigger is 200-64-5 = 131px: three rows and a bit.
	g.click(pointer.ButtonPrimary, image.Pt(triggerAt.X+20, dropdownTop+131-2))
	frame()
	if !s.Opened() {
		t.Fatal("a press on the last pixel of the cut menu closed it")
	}
	g.click(pointer.ButtonPrimary, image.Pt(triggerAt.X+20, dropdownTop+131+3))
	frame()
	if s.Opened() {
		t.Fatal("the menu runs past the room it was cut to")
	}
	// Open it again, wheel two rows, and the first row in view is the third.
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	frame()
	g.hover(image.Pt(triggerAt.X+20, dropdownTop+20))
	frame()
	g.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(float32(triggerAt.X+20), float32(dropdownTop+20)), Scroll: f32.Pt(0, 2*dropdownRow)})
	frame()
	frame()
	g.click(pointer.ButtonPrimary, dropdownRowCenter(0))
	frame()
	c.only(t, "R2")
}

// A panel that spans its trigger stands as wide as it, whatever width it was
// given, so a trigger that resizes (the sidebar's account row) keeps its
// panel on the same left and right edges.
func TestDropdownPanelSpanningStandsAsWideAsItsTrigger(t *testing.T) {
	g, th := &menuRig{size: image.Pt(400, 400)}, testTheme()
	var s DropdownPanelState
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D {
			return th.DropdownPanel(&g.root, &s, dropdownWidth, func(gtx C, th *Theme) D {
				return D{Size: image.Pt(gtx.Constraints.Max.X, 20)}
			}).Spanning().Layout(gtx, trigger)
		})
	}
	frame()
	g.click(pointer.ButtonPrimary, triggerAt.Add(image.Pt(10, 10)))
	frame()
	frame()
	if !s.Opened() {
		t.Fatal("press on the trigger did not open the panel")
	}
	if box := s.Box(); box.Min.X != triggerAt.X || box.Dx() != triggerSize.X {
		t.Errorf("panel spans x %d..%d, want the trigger's %d..%d", box.Min.X, box.Max.X, triggerAt.X, triggerAt.X+triggerSize.X)
	}
}
