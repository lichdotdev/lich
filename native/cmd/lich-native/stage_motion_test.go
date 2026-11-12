package main

import (
	"image"
	"testing"
	"time"
)

func drawPanes(m *paneMotion, at time.Time, zoomed bool, panes map[string]image.Rectangle) (map[string]image.Rectangle, []image.Rectangle) {
	ids := make([]string, 0, len(panes))
	for id := range panes {
		ids = append(ids, id)
	}
	m.begin(at, zoomed, ids)
	frames := map[string]image.Rectangle{}
	for id, r := range panes {
		frames[id] = m.frame(id, r)
	}
	return frames, m.end()
}

var (
	whole = image.Rect(0, 0, 100, 50)
	left  = image.Rect(0, 0, 47, 50)
	right = image.Rect(53, 0, 100, 50)
)

func TestAPaneNewToTheStageGrowsOutOfTheEdgeItWasSplitOn(t *testing.T) {
	var m paneMotion
	t0 := time.Unix(100, 0)
	drawPanes(&m, t0, false, map[string]image.Rectangle{"a": whole})
	two := map[string]image.Rectangle{"a": left, "b": right}
	frames, ghosts := drawPanes(&m, t0, false, two)
	if frames["a"] != whole || frames["b"] != image.Rect(100, 0, 100, 50) || ghosts != nil {
		t.Fatalf("first frame %v ghosts %v, want a where it was and b flat on the right edge", frames, ghosts)
	}
	frames, _ = drawPanes(&m, t0.Add(paneEnter/2), false, two)
	if a, b := frames["a"], frames["b"]; a.Max.X <= left.Max.X || a.Max.X >= whole.Max.X || b.Min.X <= right.Min.X || b.Max != right.Max {
		t.Fatalf("midway %v, want a giving room and b growing from the right edge", frames)
	}
	frames, _ = drawPanes(&m, t0.Add(paneEnter), false, two)
	if frames["a"] != left || frames["b"] != right || m.from != nil {
		t.Fatalf("at the end %v running %v, want both in place and the run over", frames, m.from != nil)
	}
}

func TestAPaneThatLeftShrinksIntoItsEdgeWhileItsNeighbourTakesTheRoom(t *testing.T) {
	var m paneMotion
	t0 := time.Unix(100, 0)
	drawPanes(&m, t0, false, map[string]image.Rectangle{"a": left, "b": right})
	one := map[string]image.Rectangle{"a": whole}
	frames, ghosts := drawPanes(&m, t0, false, one)
	if frames["a"] != left || len(ghosts) != 1 || ghosts[0] != right {
		t.Fatalf("first frame %v ghosts %v, want a where it was and b's ghost whole", frames, ghosts)
	}
	_, ghosts = drawPanes(&m, t0.Add(paneLeave/2), false, one)
	if len(ghosts) != 1 || ghosts[0].Max != right.Max || ghosts[0].Min.X <= right.Min.X {
		t.Fatalf("midway ghosts %v, want b shrinking into the right edge", ghosts)
	}
	frames, ghosts = drawPanes(&m, t0.Add(paneLeave), false, one)
	if frames["a"] != whole || ghosts != nil {
		t.Fatalf("at the end %v ghosts %v", frames, ghosts)
	}
}

func TestOnlyAChangeOfCompanyMovesThePanes(t *testing.T) {
	t0 := time.Unix(100, 0)
	two := map[string]image.Rectangle{"a": left, "b": right}
	still := func(name string, zoomed bool, next map[string]image.Rectangle, prepare func(m *paneMotion)) {
		var m paneMotion
		drawPanes(&m, t0, false, two)
		if prepare != nil {
			prepare(&m)
		}
		frames, ghosts := drawPanes(&m, t0, zoomed, next)
		for id, r := range next {
			if frames[id] != r {
				t.Errorf("%s: %s drawn at %v, want %v at once", name, id, frames[id], r)
			}
		}
		if ghosts != nil {
			t.Errorf("%s: ghosts %v", name, ghosts)
		}
	}
	still("a divider dragged", false, map[string]image.Rectangle{"a": image.Rect(0, 0, 30, 50), "b": image.Rect(36, 0, 100, 50)}, nil)
	still("a zoom", true, map[string]image.Rectangle{"a": whole}, nil)
	still("out of a zoom", false, two, func(m *paneMotion) { drawPanes(m, t0, true, map[string]image.Rectangle{"a": whole}) })
	still("another tab", false, map[string]image.Rectangle{"c": whole}, nil)
}

func TestAPaneSplitBelowGrowsFromTheBottom(t *testing.T) {
	top, bottom := image.Rect(0, 0, 100, 22), image.Rect(0, 28, 100, 50)
	if got, ok := collapsed(bottom, whole); !ok || got != image.Rect(0, 50, 100, 50) {
		t.Fatalf("collapsed(bottom) = %v %v", got, ok)
	}
	if got, ok := collapsed(top, whole); !ok || got != image.Rect(0, 0, 100, 0) {
		t.Fatalf("collapsed(top) = %v %v", got, ok)
	}
	if _, ok := collapsed(whole, whole); ok {
		t.Fatal("a pane that fills its donor has no edge to grow from")
	}
	if _, ok := collapsed(image.Rect(200, 0, 300, 50), whole); ok {
		t.Fatal("a pane outside the donor grows from none of its edges")
	}
}
