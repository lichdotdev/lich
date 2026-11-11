package extrender

import (
	"testing"

	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/extension"
)

func everyContent() extension.Node {
	lead := extension.Glyph.New(extension.GlyphProps{Name: "git-branch"})
	return column(
		extension.Notice.New(extension.TextProps{Text: "Nothing here"}),
		extension.Footnote.New(extension.TextProps{Text: "Two branches"}),
		extension.BodyLine.New(extension.TextProps{Text: "Whitespace only"}),
		extension.PanelNotice.New(extension.PanelNoticeProps{Text: "Could not read", PadX: 14}),
		extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow},
			extension.IconAction.New(extension.IconActionProps{Icon: "columns-2", Label: "Split", Pressed: true, OnPress: "split"}),
			extension.IconAction.New(extension.IconActionProps{Icon: "no-such-icon", Label: "Broken"}),
			extension.CornerButton.New(extension.IconToggleProps{Icon: "pin", On: true, OnPress: "pin"}),
			extension.CornerButton.New(extension.IconToggleProps{Icon: "pin"}),
			extension.BarIconButton.New(extension.IconToggleProps{Icon: "bell", On: true, OnPress: "bell"}),
			extension.BarIconButton.New(extension.IconToggleProps{Icon: "plus"}),
			extension.VerticalRule.New(extension.NoProps{}),
			extension.DiffStat.New(extension.DiffStatProps{Added: 12, Deleted: 3}),
		),
		extension.MetaGlyph.New(extension.MetaGlyphProps{Icon: "check", Text: "Resolved", Color: "tone-pass"}),
		extension.Pill.New(extension.PillProps{PadLeft: 7, PadRight: 9}, label("lich"), label("pill")),
		extension.KeyCap.New(extension.KeyCapProps{Keys: "Ctrl+K"}),
		extension.PopoverRow.New(extension.PopoverRowProps{OnPress: "row"}, label("Open in editor")),
		extension.SelectTrigger.New(extension.SelectTriggerProps{Value: "origin/main", Width: 240, Lead: &lead}),
		extension.SelectTrigger.New(extension.SelectTriggerProps{Value: "full width"}),
	)
}

func TestEveryContentDrawsWithEveryProp(t *testing.T) {
	drawsWithoutPanic(t, everyContent())
}

func TestAPressOnAContentButtonSendsItsHandler(t *testing.T) {
	for _, n := range []extension.Node{
		extension.IconAction.New(extension.IconActionProps{Icon: "columns-2", Label: "Split", OnPress: "pressed"}),
		extension.CornerButton.New(extension.IconToggleProps{Icon: "pin", OnPress: "pressed"}),
		extension.BarIconButton.New(extension.IconToggleProps{Icon: "bell", OnPress: "pressed"}),
		extension.PopoverRow.New(extension.PopoverRowProps{OnPress: "pressed"}, label("row")),
	} {
		g := newRig(t, column(n))
		g.click(pointer.ButtonPrimary, 5, 5)
		if got := g.fired("pressed"); len(got) != 1 {
			t.Errorf("%s: events = %+v, want one press", n.Type, got)
		}
	}
}

func TestAButtonWithNoHandlerTakesNoPress(t *testing.T) {
	for _, n := range []extension.Node{
		extension.CornerButton.New(extension.IconToggleProps{Icon: "pin"}),
		extension.BarIconButton.New(extension.IconToggleProps{Icon: "bell"}),
	} {
		g := newRig(t, column(n))
		g.click(pointer.ButtonPrimary, 5, 5)
		if len(g.got) != 0 {
			t.Errorf("%s: events = %+v, want none", n.Type, g.got)
		}
	}
}
