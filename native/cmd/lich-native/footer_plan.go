package main

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icGauge     = icons.Lucide("gauge")
	planMinWide = ui.Space(52)
	accountSize = ui.TextXS - 1
	// accountLeading is a 4/3 line height, at 11px.
	accountLeading unit.Sp = 14.67
)

// planDetails is the plan quota's tooltip body: the usage blocks, and the
// account the reading was taken against.
func planDetails(th *ui.Theme, plan backend.QuotaPlan, now time.Time) layout.Widget {
	return usageStack(planRows(th, plan, now, nil))
}

func planRows(th *ui.Theme, plan backend.QuotaPlan, now time.Time, use *resetUse) []layout.Widget {
	rows := usageBlocks(th, plan, now, use)
	if account := accountLine(plan); account != "" {
		rows = append(rows, accountRow(th, account))
	}
	return rows
}

// accountRow is the monospace line naming the account, under a separator.
func accountRow(th *ui.Theme, account string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Separator),
			ui.VGap(2),
			layout.Rigid(para(th.Text(accountSize, account).Leading(accountLeading).Muted().Mono()).Layout),
		)
	}
}
