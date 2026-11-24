package main

import (
	"testing"

	"gioui.org/io/key"
)

func TestVimKeyOfSpellsKeysAsNvimInputTakesThem(t *testing.T) {
	cases := []struct {
		name key.Name
		mods key.Modifiers
		want string
	}{
		{key.NameEscape, 0, "<Esc>"},
		{key.NameReturn, 0, "<CR>"},
		{key.NameTab, key.ModShift, "<S-Tab>"},
		{key.NameDeleteBackward, 0, "<BS>"},
		{key.NameLeftArrow, key.ModCtrl, "<C-Left>"},
		{"R", key.ModCtrl, "<C-r>"},
		{"V", key.ModCtrl | key.ModShift, "<C-S-v>"},
		{"[", key.ModCtrl, "<C-[>"},
		{"<", key.ModAlt, "<M-lt>"},
		{key.NameSpace, key.ModCtrl, "<C-Space>"},
	}
	for _, c := range cases {
		got, ok := vimKeyOf(key.Event{Name: c.name, Modifiers: c.mods, State: key.Press})
		if !ok || got != c.want {
			t.Errorf("%q %v = %q, %v; want %q", c.name, c.mods, got, ok, c.want)
		}
	}
}

func TestVimKeyOfLeavesTextToTheEditEvent(t *testing.T) {
	for _, ev := range []key.Event{
		{Name: "A", State: key.Press},
		{Name: "A", Modifiers: key.ModShift, State: key.Press},
		{Name: key.NameSpace, State: key.Press},
		{Name: "Q", Modifiers: key.ModCtrl | key.ModAlt, State: key.Press},
		{Name: key.NameEscape, State: key.Release},
	} {
		if got, ok := vimKeyOf(ev); ok {
			t.Errorf("%+v sent %q; want it left to the text it types", ev, got)
		}
	}
}

func TestVimTextOfEscapesTheKeyNotationOpener(t *testing.T) {
	if got := vimTextOf("a<b"); got != "a<lt>b" {
		t.Fatalf("got %q", got)
	}
}
