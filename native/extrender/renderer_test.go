package extrender

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func testTheme() *ui.Theme {
	return &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
}

// frame lays tree out once on a 400x300 window fed by router.
func frame(r *Renderer, router *input.Router, tree extension.Node) {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300)), Source: router.Source()}
	r.Layout(gtx, testTheme(), tree)
	router.Frame(&ops)
}

func TestAPressOnAButtonSendsItsHandler(t *testing.T) {
	var got []extension.EventData
	r := New(nil, func(e extension.EventData) { got = append(got, e) })
	tree := extension.Box.New(extension.BoxProps{}, extension.Button.New(extension.ButtonProps{Text: "Run", OnPress: "run"}))
	router := new(input.Router)
	frame(r, router, tree)
	router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(5, 5)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(5, 5)},
	)
	frame(r, router, tree)
	if len(got) != 1 || got[0].Handler != "run" {
		t.Fatalf("events = %+v, want one run", got)
	}
}

func TestStateOfANodeTheTreeDroppedIsDropped(t *testing.T) {
	r := New(nil, func(extension.EventData) {})
	router := new(input.Router)
	button := extension.Button.New(extension.ButtonProps{Text: "x"}).WithKey("x")
	frame(r, router, extension.Box.New(extension.BoxProps{}, button))
	if len(r.state) != 1 {
		t.Fatalf("state after drawing a button = %d entries, want 1", len(r.state))
	}
	frame(r, router, extension.Box.New(extension.BoxProps{}))
	if len(r.state) != 0 {
		t.Errorf("state after the button left = %d entries, want 0", len(r.state))
	}
}

func TestAnUnknownIconOrColorDoesNotPanic(t *testing.T) {
	r := New(nil, func(extension.EventData) {})
	tree := extension.Box.New(extension.BoxProps{Background: "no-such-token"},
		extension.Button.New(extension.ButtonProps{Text: "x", Icon: "no-such-icon"}))
	frame(r, new(input.Router), tree)
}
