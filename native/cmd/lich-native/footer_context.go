package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// contextSegments is the bar's parts, the sikemux meter's twenty.
	contextSegments = 20
	// contextWaitAbove and contextAlarmAbove are where the bar leaves its plain
	// color for ToneWait and for Destructive, in percent of the window. They are
	// the bar's own, higher than the ring's (toneOf): the bar says how much
	// room is left, the ring warns.
	contextWaitAbove  = 70
	contextAlarmAbove = 90
)

// contextBarTone is the tone of the bar's lit segments at percent of the
// window: muted up to contextWaitAbove, wait up to contextAlarmAbove,
// destructive past it.
func contextBarTone(percent int) usageTone {
	switch {
	case percent > contextAlarmAbove:
		return toneDestructive
	case percent > contextWaitAbove:
		return toneWait
	}
	return toneMuted
}

// compactCount is a token count in the short form of a reading: 585653 is
// "585.6K", 1000000 "1M", with the locale's decimal mark. Past a thousand it
// keeps one decimal and drops a zero one.
func compactCount(n int, l i18n.Locale) string {
	if n < 1000 {
		return strconv.Itoa(max(n, 0))
	}
	units := []struct {
		size   float64
		suffix string
	}{{1e3, "K"}, {1e6, "M"}, {1e9, "B"}}
	i := 0
	for i+1 < len(units) && float64(n) >= units[i+1].size {
		i++
	}
	v := float64(n) / units[i].size
	// 999,950 rounds to 1000.0K: it reads in the next unit.
	if s := fmt.Sprintf("%.1f", v); s == "1000.0" && i+1 < len(units) {
		i++
		v = float64(n) / units[i].size
	}
	s := strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0")
	if decimalComma(l) {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s + units[i].suffix
}

// decimalComma is whether l writes its decimals with a comma.
func decimalComma(l i18n.Locale) bool {
	switch l {
	case i18n.PtBR, i18n.ES, i18n.RU:
		return true
	}
	return false
}

// contextRow is one line of the context tooltip: its label muted at the left,
// its value at the right, and the part of the value that is only background to
// it (the window a token count is against) muted after it.
type contextRow struct {
	label, value, rest string
	tone               *color.NRGBA
}

// contextRows is the tooltip's lines for a report: tokens against the window,
// the window, the cache's hit rate and the session's cost, the last two only
// when the report has them. The number of times the session was compacted is
// not here: lich does not keep it.
func contextRows(u sessionUsage, l i18n.Locale) []contextRow {
	rows := []contextRow{
		{label: i18n.T("shell.footerSession.tokens"), value: compactCount(u.Tokens, l), rest: " / " + compactCount(u.Window, l)},
		{label: i18n.T("shell.footerSession.modelWindow"), value: compactCount(u.Window, l)},
	}
	if hit, ok := cacheHit(u.Cache); ok {
		rows = append(rows, contextRow{label: i18n.T("shell.footerSession.cache"), value: i18n.T("shell.footerSession.cacheHitShort", i18n.Args{"percent": hit})})
	}
	if u.CostUSD != nil {
		rows = append(rows, contextRow{label: i18n.T("shell.footerSession.sessionCost"), value: formatCost(*u.CostUSD)})
	}
	return rows
}

// contextBarColor is the color of the bar's lit segments at percent: the
// foreground up to contextWaitAbove, then the wait and destructive tones.
func contextBarColor(th *ui.Theme, percent int) color.NRGBA {
	if t := contextBarTone(percent); t != toneMuted {
		return toneColor(th, t)
	}
	return th.Foreground
}

// tipHead is a tooltip block's title with its reading at the right, both medium.
func tipHead(th *ui.Theme, title, reading string) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, title).Weight(font.Medium).Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(th.Text(ui.TextXS, reading).Weight(font.Medium).Layout),
		)
	}
}

// tipRow is a tooltip line: the label muted at the left, the value at the
// right with its muted rest after it.
func tipRow(th *ui.Theme, r contextRow) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		value := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, r.value).Layout)}
		if r.rest != "" {
			value = append(value, layout.Rigid(th.Text(ui.TextXS, r.rest).Muted().Layout))
		}
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, r.label).Muted().Layout),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, value...) }),
		)
	}
}

// contextTip is the body of the context tooltip: its title with the share used
// at the right, the segmented bar in the tone of how full it is, then a line
// per reading.
func contextTip(th *ui.Theme, u sessionUsage) layout.Widget {
	bar := func(gtx C) D { return th.SegmentedBar(gtx, u.Percent, contextSegments, contextBarColor(th, u.Percent)) }
	widgets := []layout.Widget{tipHead(th, i18n.T("shell.footerSession.context"), fmt.Sprintf("%d%%", u.Percent)), bar}
	for _, r := range contextRows(u, i18n.Current()) {
		widgets = append(widgets, tipRow(th, r))
	}
	return func(gtx C) D {
		gtx.Constraints.Min.X = min(gtx.Dp(contextTipWide), gtx.Constraints.Max.X)
		return ui.Stack(2, widgets...)(gtx)
	}
}

// contextTipWide is the tooltip's width: room for twenty segments and the
// longest reading beside its label.
var contextTipWide = ui.Space(60)
