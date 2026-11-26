package webview

import (
	"image"
	"testing"
)

func TestVisibleIsThePageLessItsCuts(t *testing.T) {
	r := image.Rect(0, 0, 100, 50)
	for _, tc := range []struct {
		name string
		cuts []image.Rectangle
		want int
	}{
		{"no cut", nil, 100 * 50},
		{"a corner", []image.Rectangle{image.Rect(0, 0, 10, 10)}, 100*50 - 100},
		{"two that overlap", []image.Rectangle{image.Rect(0, 0, 20, 20), image.Rect(10, 10, 30, 30)}, 100*50 - 400 - 400 + 100},
		{"one past the page", []image.Rectangle{image.Rect(90, 40, 200, 200)}, 100*50 - 100},
		{"the whole page", []image.Rectangle{image.Rect(-5, -5, 200, 200)}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := visible(r, tc.cuts)
			area := 0
			for i, a := range got {
				area += a.Dx() * a.Dy()
				if !a.In(r) {
					t.Fatalf("%v lies outside the page", a)
				}
				for _, c := range tc.cuts {
					if !a.Intersect(c).Empty() {
						t.Fatalf("%v shows over the cut %v", a, c)
					}
				}
				for _, b := range got[i+1:] {
					if !a.Intersect(b).Empty() {
						t.Fatalf("%v and %v overlap", a, b)
					}
				}
			}
			if area != tc.want {
				t.Fatalf("visible area %d, want %d", area, tc.want)
			}
		})
	}
}
