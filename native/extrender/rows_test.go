package extrender

import (
	"testing"

	"gioui.org/io/input"

	"github.com/lichdotdev/lich/extension"
)

func TestEveryRowComponentDrawsWithEveryPropSet(t *testing.T) {
	text := extension.Label.New(extension.LabelProps{Text: "x"})
	icon := extension.Glyph.New(extension.GlyphProps{Name: "folder"})
	sized := func(n extension.Node) extension.Node {
		return extension.Box.New(extension.BoxProps{Width: 22, Height: 22}, n)
	}
	tree := extension.Stack.New(extension.GapProps{},
		extension.IconLine.New(extension.IconColorProps{Icon: "git-branch", Color: "muted-foreground"}, text),
		sized(extension.BoxIcon.New(extension.IconColorProps{Icon: "folder", Color: "foreground"})),
		extension.IconLabel.New(extension.IconLabelProps{Icon: "hourglass", Glyph: "tone-wait", Label: extension.LabelProps{
			Text: "Waiting", Size: 12, Color: "tone-wait", Weight: extension.FontMedium, Mono: true, MaxLines: 1, LineHeight: 16, Align: extension.TextAlignEnd,
		}}),
		extension.IconLabel.New(extension.IconLabelProps{Icon: "hourglass", Label: extension.LabelProps{Text: "c", Align: extension.TextAlignCenter}}),
		extension.MonoGlyph.New(extension.MetaGlyphProps{Icon: "git-branch", Text: "main", Color: "foreground"}),
		extension.MutedGlyph.New(extension.MutedGlyphProps{Icon: "folder", Size: 12}),
		sized(extension.CenteredIcon.New(extension.GlyphProps{Name: "folder", Size: 13, Color: "primary"})),
		extension.TriggerChevron.New(extension.TriggerChevronProps{Open: true}),
		extension.TinyChip.New(extension.TinyChipProps{Word: "waiting", Tone: "tone-wait"}),
		extension.HoverLink.New(extension.LinkProps{Text: "Pulls", OnPress: "pulls"}),
		extension.TrailingIconLink.New(extension.TrailingIconLinkProps{Label: "Docs", Icon: "external-link", OnPress: "docs"}),
		extension.TwoLines.New(extension.TwoLinesProps{Top: "a", Under: "b"}),
		extension.SpinnerLine.New(extension.TextProps{Text: "Waiting"}),
		extension.SpinnerBeside.New(extension.NoProps{}, text),
		extension.CheckRow.New(extension.CheckRowProps{Checked: true, Label: "Run", Note: "note", OnChange: "check"}),
		extension.NavRow.New(extension.NavRowProps{On: true, Height: 30, PadX: 4, OnPress: "nav"}, text),
		extension.PanelRow.New(extension.PanelRowProps{Live: true, Edge: 6, PadX: 4, OnPress: "panel"}, text),
		extension.FillRow.New(extension.FillRowProps{Width: 100, Height: 20, Alpha: 0.4}),
		extension.OrRule.New(extension.WordProps{Word: "or"}),
		extension.MenuHeader.New(extension.MenuHeaderProps{Title: "Recent", Count: "3"}),
		extension.MenuDivider.New(extension.NoProps{}),
		extension.RailHairline.New(extension.NoProps{}),
		extension.CaptionDivider.New(extension.CaptionDividerProps{Inset: 6, Height: 20}),
		extension.GhostTextButton.New(extension.GhostTextButtonProps{Label: "Remove", Color: "destructive", OnPress: "remove"}),
		extension.CardIconButton.New(extension.CardIconButtonProps{Icon: "refresh-cw", Size: 20, Turn: 1.2, OnPress: "refresh"}),
		extension.CardIconButton.New(extension.CardIconButtonProps{Icon: "refresh-cw"}),
		extension.GhostIconButton.New(extension.GhostIconButtonProps{Icon: "rotate-ccw", Hint: "Reset", Disabled: true, OnPress: "reset"}),
		extension.SquareFace.New(extension.FaceProps{Background: "accent"}, icon),
		extension.LabelFace.New(extension.LabelFaceProps{Background: "accent", Label: "New"}, icon),
	)
	r := New(nil, func(extension.EventData) {})
	router := new(input.Router)
	frame(r, router, tree)
	frame(r, router, tree)
}

func TestARowWithAnUnknownIconDrawsNothing(t *testing.T) {
	const none = "no-such-icon"
	for _, n := range []extension.Node{
		extension.IconLine.New(extension.IconColorProps{Icon: none}),
		extension.BoxIcon.New(extension.IconColorProps{Icon: none}),
		extension.IconLabel.New(extension.IconLabelProps{Icon: none}),
		extension.MonoGlyph.New(extension.MetaGlyphProps{Icon: none, Text: "x"}),
		extension.MutedGlyph.New(extension.MutedGlyphProps{Icon: none}),
		extension.CenteredIcon.New(extension.GlyphProps{Name: none}),
		extension.TrailingIconLink.New(extension.TrailingIconLinkProps{Icon: none, Label: "x"}),
		extension.CardIconButton.New(extension.CardIconButtonProps{Icon: none}),
		extension.GhostIconButton.New(extension.GhostIconButtonProps{Icon: none}),
		extension.DropHint.New(extension.DropHintProps{Icon: none, Label: "x"}),
	} {
		r := New(nil, func(extension.EventData) {})
		frame(r, new(input.Router), n)
	}
}

func TestAPressOnANavRowSendsItsHandler(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	tree := extension.NavRow.New(extension.NavRowProps{OnPress: "nav"}, extension.Label.New(extension.LabelProps{Text: "General"}))
	tap(r, new(input.Router), tree, 10, 10)
	onlyEvent(t, got, extension.EventData{Handler: "nav"})
}

func TestAPressOnAPanelRowSendsItsHandler(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	tree := extension.PanelRow.New(extension.PanelRowProps{Live: true, OnPress: "panel"}, extension.Label.New(extension.LabelProps{Text: "Lint"}))
	tap(r, new(input.Router), tree, 20, 8)
	onlyEvent(t, got, extension.EventData{Handler: "panel"})
}

func TestAFlippedCheckRowSendsTheNewValue(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	tree := extension.CheckRow.New(extension.CheckRowProps{Label: "Run confined", OnChange: "confine"})
	tap(r, new(input.Router), tree, 8, 8)
	if len(got) != 1 || got[0].Handler != "confine" || !got[0].Checked {
		t.Fatalf("events = %+v, want one confine, checked", got)
	}
}

func TestADisabledGhostIconButtonSendsNothing(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	tree := extension.GhostIconButton.New(extension.GhostIconButtonProps{Icon: "rotate-ccw", Disabled: true, OnPress: "reset"})
	tap(r, new(input.Router), tree, 8, 8)
	if len(got) != 0 {
		t.Fatalf("events = %+v, want none", got)
	}
}

func TestAPressOnAGhostTextButtonSendsItsHandler(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	tree := extension.GhostTextButton.New(extension.GhostTextButtonProps{Label: "Remove", OnPress: "remove"})
	tap(r, new(input.Router), tree, 8, 8)
	onlyEvent(t, got, extension.EventData{Handler: "remove"})
}
