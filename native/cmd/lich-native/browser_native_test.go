package main

import (
	"image"
	"slices"
	"testing"
)

func TestAPageIsCutWhereALayerCoversItAndNowhereElse(t *testing.T) {
	page := image.Rect(100, 50, 900, 650)
	own := []image.Rectangle{image.Rect(10, 10, 60, 30)}
	covers := []image.Rectangle{
		image.Rect(700, 0, 1000, 700), // a dock sliding in from the right, past the page's edge
		image.Rect(0, 0, 80, 700),     // a sidebar that stops short of the page
	}
	got := pageCuts(page, own, covers)
	want := []image.Rectangle{image.Rect(10, 10, 60, 30), image.Rect(600, 0, 800, 600)}
	if !slices.Equal(got, want) {
		t.Fatalf("cuts %v, want the page's own and the dock's part over the page, local to it: %v", got, want)
	}
}

func TestAPageWithNoCoverKeepsItsOwnCutsOnly(t *testing.T) {
	own := []image.Rectangle{image.Rect(0, 0, 5, 5)}
	if got := pageCuts(image.Rect(0, 0, 100, 100), own, nil); !slices.Equal(got, own) {
		t.Fatalf("cuts %v, want %v", got, own)
	}
}
