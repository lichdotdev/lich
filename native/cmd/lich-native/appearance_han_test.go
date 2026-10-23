package main

import (
	"testing"

	"gioui.org/app"
	"gioui.org/text"
	"gioui.org/widget/material"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"golang.org/x/image/math/fixed"
)

// An interface in a language that is not CJK leaves the CJK faces, tens of
// megabytes, out of its shaper until CJK text comes up on screen, and takes
// them in on the frame after it does.
func TestCJKFacesJoinTheInterfaceOnceCJKTextComesUp(t *testing.T) {
	u := &window{win: new(app.Window), th: material.NewTheme(), theme: &ui.Theme{}}
	if err := u.setInterfaceFonts(interfaceFonts{mono: fonts.SystemMono}); err != nil {
		t.Fatal(err)
	}
	draw := func(s string) {
		u.theme.Shaper.LayoutString(text.Parameters{PxPerEm: fixed.I(16), MaxWidth: 1 << 20}, s)
		for _, ok := u.theme.Shaper.NextGlyph(); ok; _, ok = u.theme.Shaper.NextGlyph() {
		}
	}
	draw("main · 3 sessions")
	u.followLanguage()
	if u.uiFonts.han {
		t.Fatal("the CJK faces were loaded with no CJK text on screen")
	}
	draw("feat/中文")
	u.followLanguage()
	if !u.uiFonts.han {
		t.Fatal("CJK text came up and the CJK faces were not loaded on the next frame")
	}
}
