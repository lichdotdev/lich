package main

import (
	"strings"
	"testing"
)

func chunks(text string) []any { return []any{[]any{int64(0), text, int64(0)}} }

func TestVimUIFollowsTheModesCursorAndTheCommandLine(t *testing.T) {
	var u vimUI
	u.apply([]any{"mode_info_set", []any{true, []any{
		map[string]any{"name": "normal", "cursor_shape": "block", "cell_percentage": int64(0)},
		map[string]any{"name": "insert", "cursor_shape": "vertical", "cell_percentage": int64(25)},
	}}})
	u.apply([]any{"mode_change", []any{"insert", int64(1)}})
	if c := u.cursor(); c.shape != "vertical" || c.percent != 25 {
		t.Fatalf("insert mode's cursor = %+v; want the 'guicursor' ver25", c)
	}
	u.apply([]any{"cmdline_show", []any{chunks("s/a/b"), int64(5), ":", "", int64(0), int64(1)}})
	u.apply([]any{"cmdline_pos", []any{int64(2), int64(1)}})
	if u.cmdline == nil || u.cmdline.text != ":s/a/b" || u.cmdline.pos != 3 {
		t.Fatalf("command line = %+v; want :s/a/b with the caret after s/", u.cmdline)
	}
	u.apply([]any{"cmdline_hide", []any{int64(1), false}})
	if u.cmdline != nil {
		t.Fatal("the command line stayed after it closed")
	}
}

func TestVimUIKeepsTheLastMessageAndMarksErrors(t *testing.T) {
	var u vimUI
	u.apply([]any{"msg_show", []any{"emsg", chunks("E492: Not an editor command: foo"), false}})
	if u.msg != "E492: Not an editor command: foo" || !u.msgError {
		t.Fatalf("message = %q, error %v", u.msg, u.msgError)
	}
	u.apply([]any{"msg_showmode", []any{chunks("-- INSERT --")}})
	u.apply([]any{"msg_clear", []any{}})
	if u.msg != "" || u.showmode != "-- INSERT --" {
		t.Fatalf("message %q, mode %q after a clear", u.msg, u.showmode)
	}
}

func TestVimModeLabelNamesEveryMode(t *testing.T) {
	for mode, want := range map[string]string{
		"n": "NORMAL", "niI": "NORMAL", "no": "O-PENDING", "nov": "O-PENDING",
		"v": "VISUAL", "V": "V-LINE", "\x16": "V-BLOCK", "vs": "VISUAL",
		"i": "INSERT", "ic": "INSERT", "R": "REPLACE", "Rv": "V-REPLACE",
		"c": "COMMAND", "cv": "COMMAND", "s": "SELECT", "?": "",
	} {
		if got, _ := vimModeLabel(mode); got != want {
			t.Errorf("%q = %q, want %q", mode, got, want)
		}
	}
}

func TestVimShowmodeRestDropsWhatTheLabelSays(t *testing.T) {
	for in, want := range map[string]string{
		"-- INSERT --":             "",
		"-- VISUAL LINE --":        "",
		"-- INSERT --recording @q": "recording @q",
		"recording @q":             "recording @q",
		"":                         "",
	} {
		if got := vimShowmodeRest(in); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

func TestVimModeBadgesTellTheModesApart(t *testing.T) {
	lanes := map[int]string{}
	for _, mode := range []string{"n", "i", "v", "R", "c"} {
		name, lane := vimModeLabel(mode)
		if other, taken := lanes[lane]; taken {
			t.Fatalf("%s and %s wear the same lane %d", other, name, lane)
		}
		lanes[lane] = name
	}
	_, visual := vimModeLabel("v")
	for _, mode := range []string{"V", "\x16", "s"} {
		if _, lane := vimModeLabel(mode); lane != visual {
			t.Fatalf("%q wears lane %d, apart from VISUAL's %d", mode, lane, visual)
		}
	}
}

func TestVimUIStacksAListAndShowsItsTail(t *testing.T) {
	var u vimUI
	for _, line := range []string{"  1 %a \"a.go\"", "  2    \"b.go\""} {
		u.apply([]any{"msg_show", []any{"list_cmd", chunks(line), false}})
	}
	if u.msg != "  1 %a \"a.go\"\n  2    \"b.go\"" {
		t.Fatalf(":ls shows %q", u.msg)
	}
	u.apply([]any{"msg_show", []any{"", chunks("written"), true}})
	if u.msg != "written" {
		t.Fatalf("a replacing message left %q", u.msg)
	}
	u.apply([]any{"msg_history_show", []any{[]any{[]any{"", chunks("first")}, []any{"emsg", chunks("E492: x")}}}})
	if u.msg != "first\nE492: x" {
		t.Fatalf(":messages shows %q", u.msg)
	}
	long := strings.Repeat("line\n", vimMessageLines+5) + "last"
	if tail := vimMessageTail(long); strings.Count(tail, "\n") != vimMessageLines-1 || !strings.HasSuffix(tail, "last") {
		t.Fatalf("a long message kept %d lines", strings.Count(tail, "\n")+1)
	}
}

func TestVimUIFollowsTheCompletionMenu(t *testing.T) {
	var u vimUI
	items := []any{[]any{"Relaunch", "func", "", ""}, []any{"NoWindowArg", "const", "", ""}}
	u.apply([]any{"popupmenu_show", []any{items, int64(-1), int64(4), int64(8), int64(1)}})
	if u.pum == nil || len(u.pum.items) != 2 || u.pum.items[1].kind != "const" || u.pum.selected != -1 {
		t.Fatalf("menu = %+v", u.pum)
	}
	u.apply([]any{"popupmenu_select", []any{int64(1)}})
	if u.pum.selected != 1 {
		t.Fatalf("selected %d after a select", u.pum.selected)
	}
	u.apply([]any{"popupmenu_show", []any{items, int64(0), int64(0), int64(1), int64(-1)}})
	if u.pum != nil {
		t.Fatal("the command line's completion was taken for the editor's")
	}
}

func TestVimWordStartIsWhereTheCompletedWordBegins(t *testing.T) {
	st := vimState0(t, "  restart.Rela\n")
	if got := vimWordStart(st, 14); got != 10 {
		t.Fatalf("word starts at %d, want 10", got)
	}
}
