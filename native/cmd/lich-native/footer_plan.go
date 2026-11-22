package main

import (
	"gioui.org/unit"

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
