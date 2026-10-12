package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/unit"
)

func TestOpenAtShowsASubmenuAndAMarkInTheIconsPlace(t *testing.T) {
	var marked unit.Dp
	sub := []MenuEntry{
		{Kind: MenuLabel, Label: "Same conversation"},
		{Label: "Work", Shortcut: "12% used", Lead: func(gtx C, size unit.Dp) D {
			marked = size
			return D{Size: image.Pt(gtx.Dp(size), gtx.Dp(size))}
		}},
	}
	entries := []MenuEntry{{Label: "Rename"}, {Kind: MenuSubTrigger, Label: "Continue in", Sub: sub}}
	g, s, th := &menuRig{size: image.Pt(400, 400)}, new(MenuState), testTheme()
	frame := func() {
		g.frame(func(gtx C, trigger layout.Widget) D { return th.ContextMenu(&g.root, s, entries).Layout(gtx, trigger) })
	}
	frame()
	s.OpenAt(image.Pt(60, 60), 1, 1)
	frame()
	frame()
	if !s.Opened() || len(s.levels) != 2 {
		t.Fatalf("opened %v with %d levels, want the menu and its submenu", s.Opened(), len(s.levels))
	}
	if s.levels[1].highlight != 1 {
		t.Errorf("submenu highlight = %d, want the row walked to", s.levels[1].highlight)
	}
	if marked != menuIconSize {
		t.Errorf("Lead was drawn at %v, want the icon's size %v", marked, menuIconSize)
	}
}

func TestShortcutDestructiveIsTheWarningColorWhateverTheRow(t *testing.T) {
	th := testTheme()
	core := menuCore{th: th, panel: th}
	spent := MenuEntry{Shortcut: "limit", ShortcutDestructive: true}
	for _, highlighted := range []bool{false, true} {
		if got := core.rowColors(spent, highlighted).shortcut; got != th.Palette.Destructive {
			t.Errorf("highlighted=%v: shortcut = %v, want destructive %v", highlighted, got, th.Palette.Destructive)
		}
	}
	if got := core.rowColors(MenuEntry{Shortcut: "12%"}, false).shortcut; got != th.Palette.MutedForeground {
		t.Errorf("plain shortcut = %v, want muted", got)
	}
}
