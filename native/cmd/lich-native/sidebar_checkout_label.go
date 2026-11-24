package main

import (
	"gioui.org/layout"
	"github.com/lichdotdev/lich/native/ui"
)

// checkoutLabelRow is the label over the sessions of a block that share a
// checkout: its branch in the node column, then a rule to the row's end. It
// is not a row to act on: no hover, no fold, no menu.
func checkoutLabelRow(gtx C, th *ui.Theme, r sessionRow) D {
	name := r.sv.git.Branch
	if name == "" {
		name = baseName(r.path)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(1), Left: rowInset, Right: rowInset}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				l := th.Text(ui.TextXS, name).Muted().Mono()
				l.MaxLines = 1
				return l.Layout(gtx)
			}),
			ui.Gap(1.5),
			layout.Flexed(1, th.Separator),
		)
	})
}
