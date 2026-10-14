package gioterm

import (
	"image"
	"strings"
	"testing"
)

func TestURLAroundFindsTheURLUnderTheCell(t *testing.T) {
	line := []rune("see (https://github.com/omartelo/lich/pull/12) now.")
	at := strings.Index(string(line), "omartelo")
	start, end, ok := urlAround(line, at)
	if got := string(line[start:end]); !ok || got != "https://github.com/omartelo/lich/pull/12" {
		t.Fatalf("urlAround = %q, %v", got, ok)
	}
	if _, _, ok := urlAround(line, 0); ok {
		t.Fatal("urlAround off the URL found one")
	}
	dot := strings.LastIndex(string(line), ".")
	if _, _, ok := urlAround(line, dot); ok {
		t.Fatal("urlAround on the sentence's period found one")
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
	printed := Link{URL: "https://example.com/x", From: image.Pt(6, 0), To: image.Pt(26, 0)}
	if got, ok := v.LinkAt(10*10+5, 5); !ok || got != printed {
		t.Fatalf("printed URL: LinkAt = %+v, %v; want %+v", got, ok, printed)
	}
	osc8 := Link{URL: "https://lich.dev/docs", From: image.Pt(0, 1), To: image.Pt(3, 1)}
	if got, ok := v.LinkAt(1*10+5, 20+5); !ok || got != osc8 {
		t.Fatalf("OSC 8: LinkAt = %+v, %v; want %+v", got, ok, osc8)
	}
	if got, ok := v.LinkAt(7*10+5, 20+5); ok {
		t.Fatalf("past the OSC 8 link: LinkAt = %+v, want none", got)
	}
}

func TestLinkAtSpansAURLWrappedOverRows(t *testing.T) {
	v, err := New(20, 4, 10, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	v.Write([]byte("see https://example.com/a/very/long/path end"))
	want := Link{URL: "https://example.com/a/very/long/path", From: image.Pt(4, 0), To: image.Pt(19, 1)}
	if got, ok := v.LinkAt(2*10+5, 20+5); !ok || got != want {
		t.Fatalf("wrapped URL from its second row: LinkAt = %+v, %v; want %+v", got, ok, want)
	}
}
