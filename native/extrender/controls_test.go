package extrender

import (
	"slices"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func everyControl() extension.Node {
	return column(
		extension.Scroll.New(extension.ScrollProps{Direction: extension.DirectionRow}, label("a"), label("b")),
		extension.Scroll.New(extension.ScrollProps{}, label("c")),
		extension.Wrap.New(extension.WrapProps{GapX: 4, GapY: 2, Justify: extension.JustifyBetween, ContentStart: true},
			extension.WrapItem.New(extension.WrapItemProps{MarginAuto: true}, label("d")),
			extension.WrapItem.New(extension.WrapItemProps{}, label("e"), label("f")),
			label("g")),
		extension.Wrap.New(extension.WrapProps{Justify: extension.JustifyEnd}, label("h")),
		extension.Separator.New(extension.NoProps{}),
		extension.FormLabel.New(extension.FormLabelProps{Text: "Name", Size: 13, Weight: extension.FontSemibold, Uppercase: true, Disabled: true}),
		glyphs(),
		extension.Skeleton.New(extension.SkeletonProps{Width: 40, Height: 10}),
		extension.Progress.New(extension.ProgressProps{Percent: 40}),
		extension.Spinner.New(extension.SpinnerProps{Size: 12, Color: "muted-foreground"}),
		extension.BusyRing.New(extension.BusyRingProps{Size: 12, Color: "tone-pass", Track: "border"}),
		extension.Chip.New(extension.ChipProps{Label: "Open", Count: 3, On: true, OnPress: "chip"}),
		extension.Checkbox.New(extension.CheckboxProps{Checked: true, Disabled: true, OnChange: "check"}),
		extension.Switch.New(extension.SwitchProps{Checked: true, Size: extension.SwitchSizeSM, Disabled: true, OnChange: "switch"}),
		extension.Toggle.New(extension.ToggleProps{Pressed: true, Text: "Bold", Icon: "bold", Variant: extension.ToggleVariantOutline,
			Size: extension.ToggleSizeLG, Height: 30, PadX: 6, TextSize: 12, Disabled: true, OnChange: "toggle"}),
		extension.ToggleGroup.New(extension.ToggleGroupProps{Value: "b", Variant: extension.ToggleVariantOutline, Size: extension.ToggleSizeSM,
			ItemHeight: 28, ItemPadX: 6, ItemText: 12, OnChange: "group",
			Items: []extension.ToggleItem{{Value: "a", Text: "A", Icon: "list"}, {Value: "b", Text: "B", Disabled: true}}}),
		everyTabs(),
		extension.Collapse.New(extension.CollapseProps{Open: true}, label("i")),
		extension.Presence.New(extension.NoProps{}, label("j").WithKey("j"), label("k")),
		extension.SortableList.New(extension.SortableListProps{Gap: 2, Horizontal: true, Within: true, Leaves: true, OnReorder: "sort"},
			label("l").WithKey("l"), label("m").WithKey("m")),
		extension.ResizeHandle.New(extension.ResizeHandleProps{Edge: extension.PanelEdgeLeft, Width: 80, MinWidth: 40, MaxWidth: 200, OnResize: "resize"},
			label("n")),
		extension.ResizeHandle.New(extension.ResizeHandleProps{Width: 80}, label("o"), label("p")),
	)
}

func glyphs() extension.Node {
	glyph := func(name extension.Icon) extension.Node {
		return extension.Glyph.New(extension.GlyphProps{Name: name, Size: 14, Color: "primary"})
	}
	return extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow},
		glyph("git-branch"), glyph("lobe:claude"), glyph("lobe:nope"), glyph("devicon:go"), glyph("devicon:nope"),
		glyph("no-such-icon"), extension.Glyph.New(extension.GlyphProps{}))
}

func everyTabs() extension.Node {
	count := label("2")
	return extension.Tabs.New(extension.TabsProps{Selected: 1, Variant: extension.TabsVariantGhost, Orientation: extension.TabsVertical, OnChange: "tab",
		Triggers: []extension.TabsTrigger{{Text: "One", Icon: "list"}, {Text: "Two", Trailing: &count}, {Text: "Three", Disabled: true}}},
		label("one"), label("two"), label("three"))
}

func TestEveryControlDrawsWithEveryProp(t *testing.T) {
	g := drawsWithoutPanic(t, everyControl())
	g.tree = column()
	g.frame()
	g.wait(time.Second)
	for _, v := range []extension.TabsVariant{extension.TabsVariantLine, extension.TabsVariantDefault} {
		drawsWithoutPanic(t, extension.Tabs.New(extension.TabsProps{Variant: v, Triggers: []extension.TabsTrigger{{Text: "x"}}}))
	}
}

func TestAPressOnAChipSendsItsHandler(t *testing.T) {
	g := newRig(t, column(extension.Chip.New(extension.ChipProps{Label: "Open", OnPress: "chip"})))
	g.click(pointer.ButtonPrimary, 5, 5)
	g.once("chip")
}

func TestAToggledSwitchSendsItsNewState(t *testing.T) {
	sw := extension.Switch.New(extension.SwitchProps{OnChange: "switch"})
	g := newRig(t, column(sw))
	g.click(pointer.ButtonPrimary, 5, 5)
	if e := g.once("switch"); !e.Checked {
		t.Errorf("switch event = %+v, want checked", e)
	}
	g.frame()
	if s := stateOf[switchState](g.r, "/0"); !s.value.Value {
		t.Error("a render that still says unchecked turned the switch back off")
	}
}

func TestACheckboxAndAToggleSendTheirNewState(t *testing.T) {
	for _, n := range []extension.Node{
		extension.Checkbox.New(extension.CheckboxProps{Checked: true, OnChange: "changed"}),
		extension.Toggle.New(extension.ToggleProps{Text: "Bold", OnChange: "changed"}),
	} {
		g := newRig(t, column(n))
		g.click(pointer.ButtonPrimary, 5, 5)
		want := n.Type == extension.Toggle.Name
		if e := g.once("changed"); e.Checked != want {
			t.Errorf("%s event = %+v, want checked %v", n.Type, e, want)
		}
	}
}

func TestAPickedToggleGroupItemSendsItsValue(t *testing.T) {
	g := newRig(t, column(extension.ToggleGroup.New(extension.ToggleGroupProps{Value: "a", OnChange: "group",
		Items: []extension.ToggleItem{{Value: "a", Text: "Alpha"}, {Value: "b", Text: "Beta"}}})))
	probe(g, "group", func(x float32) { g.click(pointer.ButtonPrimary, x, 12) })
	if e := g.once("group"); e.Text != "b" {
		t.Errorf("group event = %+v, want b", e)
	}
}

func TestASelectedTabSendsItsIndex(t *testing.T) {
	g := newRig(t, column(extension.Tabs.New(extension.TabsProps{OnChange: "tab",
		Triggers: []extension.TabsTrigger{{Text: "Files"}, {Text: "Changes"}}}, label("files"), label("changes"))))
	probe(g, "tab", func(x float32) { g.click(pointer.ButtonPrimary, x, 15) })
	if e := g.once("tab"); e.Index != 1 {
		t.Errorf("tab event = %+v, want index 1", e)
	}
}

// probe clicks along the row at y until h fires: where a control's later
// items sit depends on the shaped text before them.
func probe(g *rig, h extension.Handler, click func(x float32)) {
	g.t.Helper()
	for x := float32(4); x < float32(rigWindow.X) && len(g.fired(h)) == 0; x += 6 {
		click(x)
	}
}

func TestADraggedResizeHandleSendsTheWidthOnRelease(t *testing.T) {
	panel := extension.Box.New(extension.BoxProps{Height: 100}, label("panel"))
	g := newRig(t, column(extension.ResizeHandle.New(extension.ResizeHandleProps{Width: 200, MinWidth: 100, MaxWidth: 300, OnResize: "resize"}, panel)))
	at := f32.Pt(197, 50)
	g.router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
	)
	g.frame()
	g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at.Add(f32.Pt(50, 0))})
	g.frame()
	if len(g.fired("resize")) != 0 {
		t.Fatal("the handle sent a width before the drag ended")
	}
	g.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at.Add(f32.Pt(50, 0))})
	g.frame()
	if e := g.once("resize"); e.Width != 250 {
		t.Errorf("resize event = %+v, want width 250", e)
	}
}

func TestReorderMovesTheKeyIntoTheSlotItWasDroppedOver(t *testing.T) {
	keys := []string{"a", "b", "c"}
	cases := []struct {
		drop ui.SortDrop
		want []string
	}{
		{ui.SortDrop{ID: "a", Over: "c"}, []string{"b", "c", "a"}},
		{ui.SortDrop{ID: "c", Over: "a"}, []string{"c", "a", "b"}},
		{ui.SortDrop{ID: "b", Over: "b"}, nil},
		{ui.SortDrop{ID: "b", Over: "elsewhere"}, nil},
	}
	for _, tc := range cases {
		got, moved := reorder(keys, tc.drop)
		if moved != (tc.want != nil) || !slices.Equal(got, tc.want) {
			t.Errorf("reorder(%+v) = %v, %v; want %v", tc.drop, got, moved, tc.want)
		}
	}
	if !slices.Equal(keys, []string{"a", "b", "c"}) {
		t.Errorf("reorder changed its input: %v", keys)
	}
}

func TestAPresenceKeepsALeavingChildUntilItsExitEnds(t *testing.T) {
	g := newRig(t, extension.Presence.New(extension.NoProps{}, label("a").WithKey("a"), label("b").WithKey("b")))
	g.tree = extension.Presence.New(extension.NoProps{}, label("a").WithKey("a"))
	g.frame()
	s := stateOf[presenceState](g.r, "")
	if _, ok := s.kids["b"]; !ok {
		t.Fatal("the leaving child was dropped before its exit played")
	}
	g.wait(ui.PresenceExit)
	g.frame()
	if _, ok := s.kids["b"]; ok {
		t.Error("the leaving child is still kept after its exit")
	}
}
