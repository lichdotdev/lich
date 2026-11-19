package main

import (
	"cmp"
	"slices"
	"time"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// usageBlockGap is the room between two blocks of the usage tooltip, more
	// than the one inside a block so each reads as a unit.
	usageBlockGap = 3
	usageLineGap  = 1.5
)

// windowRank orders the windows a tooltip lists: the session's, the weekly
// one, then the rest in the order the provider sent them (a model's own cap).
func windowRank(w backend.QuotaWindow) int {
	switch w.Kind {
	case api.QuotaWindowSession:
		return 0
	case api.QuotaWindowWeekly:
		return 1
	}
	return 2
}

// usageBlocks are the plan's tooltip blocks: a bar per window, then the extra
// spend and the reset credits when the account has them.
func usageBlocks(th *ui.Theme, plan backend.QuotaPlan, now time.Time) []layout.Widget {
	plan.Windows = slices.SortedStableFunc(slices.Values(plan.Windows), func(a, b backend.QuotaWindow) int {
		return cmp.Compare(windowRank(a), windowRank(b))
	})
	var blocks []layout.Widget
	for _, g := range quotaGauges(plan, now) {
		blocks = append(blocks, windowBlock(th, g))
	}
	if plan.Extra != nil {
		blocks = append(blocks, extraBlock(th, *plan.Extra, i18n.Current()))
	}
	if plan.ResetCredits > 0 {
		blocks = append(blocks, tipRow(th, contextRow{
			label: i18n.T("shell.planQuota.resetCredits"),
			value: i18n.T("shell.planQuota.resetCreditsAvailable", i18n.Args{"count": plan.ResetCredits}),
		}))
	}
	return blocks
}

// usageStack lays the blocks out at the width of the context tooltip, whose
// bar they share.
func usageStack(blocks []layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = min(gtx.Dp(contextTipWide), gtx.Constraints.Max.X)
		return ui.Stack(usageBlockGap, blocks...)(gtx)
	}
}

// windowBlock is one window: its name with the share spent at the right, the
// segmented bar, the provider's lock reason when it gave one, when it resets
// and, when the pace so far empties it first, when it runs out.
func windowBlock(th *ui.Theme, g quotaGauge) layout.Widget {
	bar := contextBarColor(th, g.Percent)
	if g.Locked != "" {
		bar = th.Destructive
	}
	parts := []layout.Widget{
		tipHead(th, g.Label, g.Reading),
		func(gtx C) D { return th.SegmentedBar(gtx, g.Percent, contextSegments, bar) },
	}
	if g.Locked != "" {
		parts = append(parts, para(th.Text(ui.TextXS, g.Locked).In(th.Destructive)).Layout)
	}
	if g.Left != "" {
		parts = append(parts, para(th.Text(ui.TextXS, i18n.T("shell.planQuota.resetsIn", i18n.Args{"time": g.Left})).Muted()).Layout)
	}
	if g.RunsOut != "" {
		parts = append(parts, para(th.Text(ui.TextXS, i18n.T("shell.planQuota.runsOut", i18n.Args{"time": g.RunsOut})).In(toneColor(th, toneWait))).Layout)
	}
	return ui.Stack(usageLineGap, parts...)
}

// extraBlock is the spend past the plan: what is spent against the monthly cap
// at the right of its name, and a bar only when there is a cap to fill.
func extraBlock(th *ui.Theme, e backend.QuotaExtra, l i18n.Locale) layout.Widget {
	used := formatMoney(e.Used, e.Decimals, e.Currency, l)
	reading := i18n.T("shell.planQuota.extraNoLimit", i18n.Args{"used": used})
	parts := []layout.Widget{nil}
	if e.Limit != nil {
		reading = i18n.T("shell.planQuota.extraOf", i18n.Args{"used": used, "limit": formatMoney(*e.Limit, e.Decimals, e.Currency, l)})
	}
	parts[0] = tipHead(th, i18n.T("shell.planQuota.extraUsage"), reading)
	if e.Limit != nil && *e.Limit > 0 {
		percent := int(min((e.Used*100+*e.Limit/2) / *e.Limit, 100))
		parts = append(parts, func(gtx C) D { return th.SegmentedBar(gtx, percent, contextSegments, contextBarColor(th, percent)) })
	}
	return ui.Stack(usageLineGap, parts...)
}
