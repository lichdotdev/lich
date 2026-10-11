package main

import (
	"reflect"
	"testing"
	"time"
)

func TestGroupMatchesFoldsConsecutiveHitsOfAFile(t *testing.T) {
	groups := groupMatches([]searchMatch{{Path: "a.go", Line: 1}, {Path: "a.go", Line: 9}, {Path: "b.go", Line: 2}})
	if len(groups) != 2 || groups[0].path != "a.go" || len(groups[0].hits) != 2 || groups[1].path != "b.go" {
		t.Errorf("groups = %+v", groups)
	}
}

func TestHighlightSegmentsMarksEveryCaseInsensitiveOccurrence(t *testing.T) {
	got := highlightSegments("Foo bar foo", "FOO")
	want := []segment{{"Foo", true}, {" bar ", false}, {"foo", true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("segments = %+v, want %+v", got, want)
	}
}

func TestHighlightSegmentsShowsALineWhoseLowercaseResizesPlain(t *testing.T) {
	got := highlightSegments("İstanbul", "stan")
	if !reflect.DeepEqual(got, []segment{{"İstanbul", false}}) {
		t.Errorf("segments = %+v", got)
	}
}

func TestSearchSummaryCountsLinesAndFilesOrStopsAtTheCut(t *testing.T) {
	r := searchResult{Matches: make([]searchMatch, 3)}
	if got := searchSummary(r, 1); got != "3 matches in 1 file" {
		t.Errorf("summary = %q", got)
	}
	r.Cut = true
	if got := searchSummary(r, 2); got != "3+ matches" {
		t.Errorf("cut summary = %q", got)
	}
}

func TestSearchFootnoteSaysWhatWasNotRead(t *testing.T) {
	r := searchResult{Matches: make([]searchMatch, 2), Cut: true, TooLarge: 1}
	want := "Showing the first 2 matching lines. Narrow the search. 1 file over 1 MB not searched."
	if got := searchFootnote(r); got != want {
		t.Errorf("footnote = %q, want %q", got, want)
	}
	if got := searchFootnote(searchResult{}); got != "" {
		t.Errorf("a whole answer owes %q", got)
	}
}

func TestSettlerWaitsForTypingToPauseButClearsAtOnce(t *testing.T) {
	var s settler
	t0 := time.Unix(100, 0)
	if got, next := s.at("ab", t0); got != "" || !next.Equal(t0.Add(searchSettle)) {
		t.Errorf("while typing = %q next %v", got, next)
	}
	if got, _ := s.at("abc", t0.Add(200*time.Millisecond)); got != "" {
		t.Errorf("a keystroke restarted nothing: %q", got)
	}
	if got, next := s.at("abc", t0.Add(450*time.Millisecond)); got != "abc" || !next.IsZero() {
		t.Errorf("after the pause = %q next %v", got, next)
	}
	if got, _ := s.at("", t0.Add(460*time.Millisecond)); got != "" {
		t.Errorf("cleared = %q, want at once", got)
	}
}

func TestPadLineNumberRightAlignsInFourCells(t *testing.T) {
	if got := padLineNumber(42); got != "  42" {
		t.Errorf("pad = %q", got)
	}
	if got := padLineNumber(12345); got != "12345" {
		t.Errorf("a wider number = %q, want it whole", got)
	}
}
