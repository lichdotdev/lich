package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// hovered is the cursor after the mouse moves to p over w.
func hovered(w layout.Widget, p image.Point) pointer.Cursor {
	var r motionRig
	r.frame(w)
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(p)})
	r.frame(w)
	return r.router.Cursor()
}

// Everything that answers a click shows the hand over its box, and the arrow
// when it is disabled.
func TestClickablesShowTheHand(t *testing.T) {
	th, ic := testTheme(), icons.Lucide("plus")
	body := func(gtx C, th *Theme) D { return D{Size: image.Pt(60, 24)} }
	var (
		click, click2, click3 Clickable
		bar, tab, enum        = new(Clickable), new(TabsState), new(Enum)
		checked, pressed      Bool
		sw                    SwitchState
		root                  Root
		tip                   Tooltip
	)
	cases := []struct {
		name     string
		enabled  layout.Widget
		disabled layout.Widget // nil when the control has no disabled state
	}{
		{"Button", th.Button(&click, "Save").Layout, ButtonStyle{Click: &click, Text: "Save", Disabled: true, theme: th}.Layout},
		{"GhostTextButton", func(gtx C) D { return th.GhostTextButton(gtx, &click2, "Undo", th.Foreground) }, nil},
		{"CardIconButton", func(gtx C) D { return th.CardIconButton(gtx, &click3, ic, 20, 0) }, nil},
		{"GhostIconButton", func(gtx C) D { return th.GhostIconButton(gtx, &root, &Clickable{}, &tip, ic, "Add", false) },
			func(gtx C) D { return th.GhostIconButton(gtx, &root, &Clickable{}, &tip, ic, "Add", true) }},
		{"CornerButton", th.CornerButton(&Clickable{}, ic, false).Layout, nil},
		{"BarIconButton", th.BarIconButton(bar, ic, false).Layout, th.BarIconButton(nil, ic, false).Layout},
		{"Chip", th.Chip(&Clickable{}, "Working", 3, false).Layout, nil},
		{"NavRow", func(gtx C) D { return th.NavRow(gtx, &Clickable{}, false, 24, 4, body) }, nil},
		{"PanelRow", func(gtx C) D { return th.PanelRow(gtx, &Clickable{}, false, 0, 4, body) }, nil},
		{"PopoverRow", th.PopoverRow(&Clickable{}, body).Layout, nil},
		{"HoverLink", func(gtx C) D { return th.HoverLink(gtx, &Clickable{}, "link") }, nil},
		{"TrailingIconLink", func(gtx C) D { return th.TrailingIconLink(gtx, &Clickable{}, "link", ic) }, nil},
		{"InputGroupButton", th.InputGroupButton(&Clickable{}, "Go").Layout, nil},
		{"Tabs", func(gtx C) D { return th.Tabs(tab, TabsTrigger{Text: "One"}, TabsTrigger{Text: "Two"}).Layout(gtx) },
			func(gtx C) D { return th.Tabs(&TabsState{}, TabsTrigger{Text: "One", Disabled: true}).Layout(gtx) }},
		{"ToggleGroup", th.ToggleGroup(enum, ToggleItem{Value: "a", Text: "A"}, ToggleItem{Value: "b", Text: "B"}).Layout,
			th.ToggleGroup(&Enum{}, ToggleItem{Value: "a", Text: "A", Disabled: true}).Layout},
		{"Toggle", th.Toggle(&pressed, "Bold").Layout, ToggleStyle{Pressed: &pressed, Text: "Bold", Disabled: true, theme: th}.Layout},
		{"Checkbox", th.Checkbox(&checked).Layout, CheckboxStyle{Checked: &checked, Disabled: true, theme: th}.Layout},
		{"Switch", th.Switch(&sw).Layout, SwitchStyle{Checked: &sw, Disabled: true, theme: th}.Layout},
	}
	inside := image.Pt(4, 4)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hovered(c.enabled, inside); got != pointer.CursorPointer {
				t.Errorf("enabled: cursor %v, want the hand", got)
			}
			if c.disabled != nil {
				if got := hovered(c.disabled, inside); got != pointer.CursorDefault {
					t.Errorf("disabled: cursor %v, want the arrow", got)
				}
			}
		})
	}
}

// The raw kit types, laid out by a screen of its own, carry the hand the same
// way, and a disabled gtx takes it off.
func TestKitStateTypesShowTheHand(t *testing.T) {
	var click Clickable
	var on Bool
	var enum Enum
	box := func(gtx C) D { return D{Size: image.Pt(40, 40)} }
	for name, w := range map[string]func(C, bool) D{
		"Clickable": func(gtx C, _ bool) D { return click.Layout(gtx, box) },
		"Bool":      func(gtx C, _ bool) D { return on.Layout(gtx, box) },
		"Enum":      func(gtx C, _ bool) D { return enum.Layout(gtx, "a", box) },
	} {
		t.Run(name, func(t *testing.T) {
			if got := hovered(func(gtx C) D { return w(gtx, true) }, image.Pt(4, 4)); got != pointer.CursorPointer {
				t.Errorf("enabled: cursor %v, want the hand", got)
			}
			if got := hovered(func(gtx C) D { return w(gtx.Disabled(), false) }, image.Pt(4, 4)); got != pointer.CursorDefault {
				t.Errorf("disabled: cursor %v, want the arrow", got)
			}
			if got := hovered(func(gtx C) D { return w(gtx, true) }, image.Pt(90, 90)); got != pointer.CursorDefault {
				t.Errorf("off the box: cursor %v, want the arrow", got)
			}
		})
	}
}

// A menu's entries show the hand where a click chooses them, and the arrow on
// a label, a separator or a disabled entry.
func TestMenuEntriesShowTheHandWhereAClickChoosesThem(t *testing.T) {
	entries := []MenuEntry{
		{Kind: MenuLabel, Label: "Label"},
		{Label: "Rename"},
		{Kind: MenuSeparator},
		{Label: "Delete", Disabled: true},
	}
	g, s, th := &menuRig{size: image.Pt(400, 400)}, new(MenuState), testTheme()
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D { return th.ContextMenu(&g.root, s, entries).Layout(gtx, trigger) })
	}
	frame()
	s.OpenAt(image.Pt(60, 60))
	frame()
	frame()
	if len(s.levels) == 0 || len(s.levels[0].rows) != len(entries) {
		t.Fatalf("open levels %d, want the menu with %d rows", len(s.levels), len(entries))
	}
	// The test's clock never moves, so the panel would stay mid-way through its
	// animate-in, scaled off the rows it records.
	s.levels[0].shown = time.Time{}.Add(-time.Second)
	frame()
	// The rig draws the overlay under its trigger's offset, so the rows are that far off the window.
	for i, e := range entries {
		row := s.levels[0].rows[i]
		g.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: pt(row.Min.Add(row.Size().Div(2)).Sub(triggerAt))})
		frame()
		want := pointer.CursorDefault
		if e.actionable() {
			want = pointer.CursorPointer
		}
		if got := g.router.Cursor(); got != want {
			t.Errorf("row %d (%q): cursor %v, want %v", i, e.Label, got, want)
		}
	}
}

// Only the kit's own Clickable, Bool and Enum lay a click area out, so none
// of the window's code reaches for gio's, which shows no hand. tour.go is
// exempt: gio-driver hands it its own Clickables, and it passes their clicks on.
func TestNoGioClickableOutsideTheKit(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || filepath.Base(path) == "clickable.go" || filepath.Base(path) == "tour.go" {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "widget" && slices.Contains([]string{"Clickable", "Bool", "Enum"}, sel.Sel.Name) {
					t.Errorf("%s: widget.%s; use the kit's own, which shows the hand", path, sel.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
