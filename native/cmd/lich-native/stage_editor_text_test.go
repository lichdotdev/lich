package main

import (
	"slices"
	"strings"
	"testing"
)

func TestChangedLinesMarkWhatTheUserTypedAndWhereTheyDeleted(t *testing.T) {
	base := "a\nb\nc\nd\ne\n"
	cases := []struct {
		name, doc string
		want      []int
	}{
		{"untouched", base, nil},
		{"one line edited", "a\nB\nc\nd\ne\n", []int{1}},
		{"two edits apart", "A\nb\nc\nd\nE\n", []int{0, 4}},
		{"a line added", "a\nb\nnew\nc\nd\ne\n", []int{2}},
		{"a line deleted", "a\nb\nd\ne\n", []int{2}},
		{"the last line deleted", "a\nb\nc\nd\n", []int{4}},
		{"everything replaced", "x\n", []int{0}},
	}
	for _, c := range cases {
		if got := changedLines(base, c.doc); !slices.Equal(got, c.want) {
			t.Errorf("%s: changedLines = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestChangedLinesMarkTheWholeStretchPastTheTable(t *testing.T) {
	var a, b []string
	for i := range 1100 {
		a = append(a, "same")
		b = append(b, "same")
		if i%2 == 0 {
			b[i] = "edited"
		}
	}
	got := changedLines("top\n"+strings.Join(a, "\n"), "top\n"+strings.Join(b, "\n"))
	if len(got) != 1099 || got[0] != 1 {
		t.Fatalf("got %d marks from %v, want the stretch from the first change to the last", len(got), got[:3])
	}
}

func TestADiskReadReloadsOnlyWhatTheUserLeftAlone(t *testing.T) {
	cases := []struct {
		name, base, doc, disk string
		want                  diskVerdict
	}{
		{"nothing new", "a", "a", "a", diskSame},
		{"nothing new under edits", "a", "b", "a", diskSame},
		{"the agent wrote, user idle", "a", "a", "c", diskReload},
		{"the agent wrote over edits", "a", "b", "c", diskAsk},
		{"both made the same change", "a", "b", "b", diskSame},
	}
	for _, c := range cases {
		if got := diskVerdictOf(c.base, c.doc, c.disk); got != c.want {
			t.Errorf("%s: verdict %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLineBreakKeepsACRLFFileCRLF(t *testing.T) {
	if lineBreak("a\r\nb") != "\r\n" || lineBreak("a\nb") != "" {
		t.Fatal("lineBreak missed the file's break")
	}
}
