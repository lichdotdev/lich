package extrender

import (
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/extension"
)

func trigger(text string) extension.Node {
	return extension.Button.New(extension.ButtonProps{Text: text})
}

func everyEntry() []extension.MenuEntry {
	avatar := extension.Glyph.New(extension.GlyphProps{Name: "user"})
	return []extension.MenuEntry{
		{Kind: extension.MenuItem, Label: "Rename", Icon: "pencil", Shortcut: "F2", Lead: &avatar, Inset: true, OnSelect: "rename"},
		{Kind: extension.MenuCheckboxItem, Label: "Pin", Checked: true, OnCheckedChange: "pin"},
		{Kind: extension.MenuRadioItem, Label: "Dark", Checked: true, OnSelect: "dark"},
		{Kind: extension.MenuSubTrigger, Label: "Color", Columns: 2, Sub: []extension.MenuEntry{
			{Kind: extension.MenuSwatch, Label: "Red", Swatch: "#ff0000", OnSelect: "red"},
			{Kind: extension.MenuSwatch, Label: "Theme"},
		}},
		{Kind: extension.MenuLabel, Label: "Danger"},
		{Kind: extension.MenuSeparator},
		{Label: "Delete", Shortcut: "No undo", ShortcutDestructive: true, Variant: extension.VariantDestructive, Disabled: true},
	}
}

func everyOverlay() extension.Node {
	card := label("card body")
	return column(
		extension.DropdownMenu.New(extension.DropdownMenuProps{Entries: everyEntry()}, trigger("Menu")),
		extension.Tooltip.New(extension.TooltipProps{Text: "Tip", Side: extension.SideBottom}, trigger("Tip")),
		extension.Tooltip.New(extension.TooltipProps{Content: &card, Variant: extension.TooltipCard, Side: extension.SideLeft}, trigger("Card")),
		extension.Tooltip.New(extension.TooltipProps{Text: "Card text", Variant: extension.TooltipCard, Side: extension.SideRight}, trigger("A"), trigger("B")),
		extension.ContextMenu.New(extension.ContextMenuProps{Entries: everyEntry()}, label("area")),
		extension.DropdownPanel.New(extension.DropdownPanelProps{Trigger: trigger("Panel"), Width: 200, AlignEnd: true}, label("row")),
		extension.DropdownPanel.New(extension.DropdownPanelProps{Trigger: trigger("Default")}),
		extension.Dialog.New(extension.DialogProps{Open: true, Title: "Delete?", Description: "No undo.", Actions: []extension.Node{trigger("Cancel")},
			MaxWidth: 300, HideClose: true, OnClose: "close"}, label("body")),
		extension.Dialog.New(extension.DialogProps{Open: true, Raw: true, Top: 0.2, Scrim: 0.5}, label("raw")),
	)
}

func TestEveryOverlayDrawsWithEveryProp(t *testing.T) {
	g := drawsWithoutPanic(t, everyOverlay())
	g.click(pointer.ButtonPrimary, 5, 5)
	g.wait(time.Second)
}

func TestAChosenMenuItemSendsItsHandler(t *testing.T) {
	g := newRig(t, column(extension.DropdownMenu.New(extension.DropdownMenuProps{Entries: everyEntry()}, trigger("Menu"))))
	g.click(pointer.ButtonPrimary, 5, 5)
	g.wait(time.Second)
	g.press(key.NameDownArrow, key.NameReturn)
	g.once("rename")

	g.click(pointer.ButtonPrimary, 5, 5)
	g.press(key.NameDownArrow, key.NameDownArrow, key.NameReturn)
	if e := g.once("pin"); e.Checked {
		t.Errorf("pin event = %+v, want unchecked", e)
	}
}

func TestAContextMenuOpensOnASecondaryPress(t *testing.T) {
	g := newRig(t, column(extension.ContextMenu.New(extension.ContextMenuProps{Entries: everyEntry()}, label("area"))))
	g.click(pointer.ButtonSecondary, 5, 5)
	g.wait(time.Second)
	g.press(key.NameDownArrow, key.NameReturn)
	g.once("rename")
}

func TestADismissedDialogSendsOnCloseAndStaysShut(t *testing.T) {
	dialog := extension.Dialog.New(extension.DialogProps{Open: true, Title: "Delete?", OnClose: "close"})
	g := newRig(t, column(dialog))
	g.wait(time.Second)
	g.press(key.NameEscape)
	g.once("close")
	g.frame()
	if s := stateOf[dialogState](g.r, "/0"); !s.dismissed {
		t.Error("the dialog opened again before the extension closed it")
	}
	g.tree = column(extension.Dialog.New(extension.DialogProps{Title: "Delete?", OnClose: "close"}))
	g.frame()
	g.tree = column(dialog)
	g.frame()
	if s := stateOf[dialogState](g.r, "/0"); s.dismissed {
		t.Error("the extension opened the dialog again and it stayed shut")
	}
}

func TestADropdownPanelOpensItsChildren(t *testing.T) {
	panel := extension.DropdownPanel.New(extension.DropdownPanelProps{Trigger: trigger("Panel")},
		extension.PopoverRow.New(extension.PopoverRowProps{OnPress: "row"}, label("row")))
	g := newRig(t, column(panel))
	g.click(pointer.ButtonPrimary, 5, 5)
	g.wait(time.Second)
	if _, ok := g.r.state["/0/0"]; !ok {
		t.Error("the open panel did not draw its row")
	}
}
