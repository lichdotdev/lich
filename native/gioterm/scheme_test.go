package gioterm

import (
	"image/color"
	"testing"
)

var (
	darkColors  = Colors{Foreground: color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}, Background: color.NRGBA{R: 0x10, G: 0x10, B: 0x14, A: 0xff}}
	lightColors = Colors{Foreground: color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff}, Background: color.NRGBA{R: 0xf4, G: 0xf4, B: 0xf6, A: 0xff}}
)

func replyingTerminal(t *testing.T) (*Terminal, *[]string) {
	t.Helper()
	var replies []string
	v, err := New(80, 24, testCellW, testCellH, func(b []byte) { replies = append(replies, string(b)) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Close)
	return v, &replies
}

// A program asks the terminal's color scheme with DSR 996 and is told the
// one its background is in.
func TestColorSchemeQueryAnswersTheBackgroundsScheme(t *testing.T) {
	v, replies := replyingTerminal(t)
	if err := v.SetColors(darkColors); err != nil {
		t.Fatal(err)
	}
	v.Write([]byte("\x1b[?996n"))
	if err := v.SetColors(lightColors); err != nil {
		t.Fatal(err)
	}
	v.Write([]byte("\x1b[?996n"))
	want := []string{"\x1b[?997;1n", "\x1b[?997;2n"}
	if len(*replies) != 2 || (*replies)[0] != want[0] || (*replies)[1] != want[1] {
		t.Fatalf("replies %q, want %q", *replies, want)
	}
}

// A program that turned mode 2031 on is told whenever the scheme changes,
// and only then; one that did not, or turned it off, is told nothing.
func TestColorSchemeChangeIsReportedUnderMode2031(t *testing.T) {
	v, replies := replyingTerminal(t)
	if err := v.SetColors(darkColors); err != nil {
		t.Fatal(err)
	}
	if err := v.SetColors(lightColors); err != nil {
		t.Fatal(err)
	}
	if len(*replies) != 0 {
		t.Fatalf("reported %q without mode 2031", *replies)
	}
	v.Write([]byte("\x1b[?2031h"))
	for _, c := range []Colors{darkColors, darkColors, lightColors} {
		if err := v.SetColors(c); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"\x1b[?997;1n", "\x1b[?997;2n"}
	if len(*replies) != 2 || (*replies)[0] != want[0] || (*replies)[1] != want[1] {
		t.Fatalf("reports %q, want %q (a theme of the same scheme reports nothing)", *replies, want)
	}
	v.Write([]byte("\x1b[?2031l"))
	if err := v.SetColors(darkColors); err != nil {
		t.Fatal(err)
	}
	if len(*replies) != 2 {
		t.Fatalf("reported %q after mode 2031 was turned off", (*replies)[2:])
	}
}
