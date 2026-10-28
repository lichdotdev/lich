package main

import (
	"image"
	"net/http"
	"net/http/httptest"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/gioterm"
)

// A session that leaves the screen lets go of its terminal's row images, and
// the one still shown keeps them.
func TestHiddenTerminalReleasesItsRenderer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{}")) }))
	t.Cleanup(srv.Close)
	ts := &terminals{client: clientOf(srv), byID: map[string]*termView{}}
	for _, id := range []string{"a", "b"} {
		r, err := gioterm.NewRenderer(gioterm.RenderRows, "monospace", 14)
		if err != nil {
			t.Fatal(err)
		}
		vt, err := gioterm.New(40, 6, 8, 16, func([]byte) {})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(vt.Close)
		ts.byID[id] = &termView{id: id, vt: vt, r: r}
		drawTerm(t, ts.byID[id])
	}
	ts.onScreen("a", "b")
	ts.onScreen("b")
	if n := drawTerm(t, ts.byID["a"]); n != 6 {
		t.Errorf("the hidden terminal redrew %d rows when shown again, want all 6: it kept its images", n)
	}
	// The cursor's row is recomposed every frame; the rest stays as it was.
	if n := drawTerm(t, ts.byID["b"]); n > 1 {
		t.Errorf("the terminal still on screen redrew %d rows, want at most the cursor's", n)
	}
}

func drawTerm(t *testing.T, v *termView) int {
	t.Helper()
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 600))}
	if err := v.r.Measure(gtx); err != nil {
		t.Fatal(err)
	}
	s, err := v.vt.Update()
	if err != nil {
		t.Fatal(err)
	}
	n, err := v.r.Draw(gtx, s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
