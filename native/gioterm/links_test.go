package gioterm

import (
	"strings"
	"testing"
)

func TestURLAroundFindsTheURLUnderTheCell(t *testing.T) {
	line := []rune("see (https://github.com/omartelo/lich/pull/12) now.")
	at := strings.Index(string(line), "omartelo")
	if got := urlAround(line, at); got != "https://github.com/omartelo/lich/pull/12" {
		t.Fatalf("urlAround = %q", got)
	}
	if got := urlAround(line, 0); got != "" {
		t.Fatalf("urlAround off the URL = %q, want none", got)
	}
	dot := strings.LastIndex(string(line), ".")
	if got := urlAround(line, dot); got != "" {
		t.Fatalf("urlAround on the sentence's period = %q, want none", got)
	}
}

func TestTrimURLKeepsBracketsTheURLOpened(t *testing.T) {
	cases := map[string]string{
		"https://en.wikipedia.org/wiki/Go_(language)": "https://en.wikipedia.org/wiki/Go_(language)",
		"https://x.dev/a).":                           "https://x.dev/a",
		"https://x.dev/a,":                            "https://x.dev/a",
		"https://x.dev/q?a=1&b=[2]":                   "https://x.dev/q?a=1&b=[2]",
		"https://localhost:8080/path/":                "https://localhost:8080/path/",
	}
	for in, want := range cases {
		if got := trimURL(in); got != want {
			t.Errorf("trimURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLinkAtReadsAPrintedURLAndAnOSC8Link(t *testing.T) {
	v, err := New(40, 4, 10, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	v.Write([]byte("go to https://example.com/x ok\r\n"))
	v.Write([]byte("\x1b]8;;https://lich.dev/docs\x1b\\docs\x1b]8;;\x1b\\ here"))
	if got := v.LinkAt(10*10+5, 5); got != "https://example.com/x" {
		t.Fatalf("printed URL: LinkAt = %q", got)
	}
	if got := v.LinkAt(1*10+5, 20+5); got != "https://lich.dev/docs" {
		t.Fatalf("OSC 8: LinkAt = %q", got)
	}
	if got := v.LinkAt(7*10+5, 20+5); got != "" {
		t.Fatalf("past the OSC 8 link: LinkAt = %q, want none", got)
	}
}
