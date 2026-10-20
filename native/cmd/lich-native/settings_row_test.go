package main

import (
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// A row keeps its control beside the text while the text has room, and puts
// it under the text when the control would squeeze the text column.
func TestSettingRowStacksControlWhenNarrow(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Canvas}
	const controlW, controlH = 224, 36
	control := func(gtx C, th *ui.Theme) D { return D{Size: image.Pt(controlW, controlH)} }
	height := func(width int) int {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(width, 1000)},
		}
		return settingRow{title: "Profile"}.layout(gtx, th, true, control).Size.Y
	}
	pads := 2 * int(rowPadY)
	if got := height(1000); got != pads+controlH {
		t.Errorf("wide row height %d, want %d: the control beside the title", got, pads+controlH)
	}
	if got, least := height(300), pads+controlH+int(ui.Space(3)); got <= least {
		t.Errorf("narrow row height %d, want more than %d: the control under the title", got, least)
	}
}
