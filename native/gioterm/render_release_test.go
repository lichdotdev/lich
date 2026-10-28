package gioterm

import (
	"crypto/sha256"
	"image"
	"reflect"
	"slices"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// A terminal off screen lets go of what its renderer keeps between frames;
// shown again, one frame rebuilds every row and paints what it painted before.
func TestReleasedRendererRepaintsTheSameFrame(t *testing.T) {
	for _, kind := range []string{RenderRows, RenderAtlas, RenderPaths} {
		t.Run(kind, func(t *testing.T) {
			r, err := NewRenderer(kind, "monospace", 14)
			if err != nil {
				t.Fatal(err)
			}
			vt, err := New(40, 6, 8, 16, func([]byte) {})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(vt.Close)
			vt.Write([]byte("hello \x1b[31mred\x1b[0m\r\n\x1b[44m blue \x1b[0m world"))

			before, _ := drawFrame(t, r, vt)
			pixels := rowPixels(r)
			r.Release()
			if rr, ok := r.(*rowRenderer); ok && (rr.imgs != nil || rr.ops != nil) {
				t.Fatal("the row images outlived Release")
			}
			after, redrawn := drawFrame(t, r, vt)
			if !reflect.DeepEqual(rowPixels(r), pixels) {
				t.Error("the rows composed after Release hold other pixels than before it")
			}
			if redrawn != 6 {
				t.Errorf("redrew %d rows after Release, want all 6", redrawn)
			}
			if after != before {
				t.Error("the frame after Release paints something else than before it")
			}
		})
	}
}

// drawFrame draws vt's current snapshot through r and returns a digest of the
// paint and how many rows r redrew.
func drawFrame(t *testing.T, r Renderer, vt *Terminal) ([32]byte, int) {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 600))}
	if err := r.Measure(gtx); err != nil {
		t.Fatal(err)
	}
	s, err := vt.Update()
	if err != nil {
		t.Fatal(err)
	}
	redrawn, err := r.Draw(gtx, s)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(reflect.ValueOf(&ops.Internal).Elem().FieldByName("data").Bytes()), redrawn
}

// rowPixels copies the row images a row renderer paints, nil for the others,
// whose paint the digest covers.
func rowPixels(r Renderer) [][]byte {
	rr, ok := r.(*rowRenderer)
	if !ok {
		return nil
	}
	var pix [][]byte
	for _, img := range rr.imgs {
		pix = append(pix, slices.Clone(img.Pix))
	}
	return pix
}
