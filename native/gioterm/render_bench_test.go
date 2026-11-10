package gioterm

import (
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// A full screen of ASCII redrawn every frame, the worst case of a frame a
// program repainting all of it (a TUI, a pager) costs.
func BenchmarkFullScreenASCIIFrame(b *testing.B) {
	const cols, rows = 200, 50
	line := strings.Repeat("The quick brown fox jumps over the lazy dog 0123456789 ", 4)[:cols]
	screen := "\x1b[H" + strings.Repeat(line, rows)
	for _, kind := range []string{RenderRows, RenderAtlas, RenderPaths} {
		b.Run(kind, func(b *testing.B) {
			r, err := NewRenderer(kind, "monospace", 14)
			if err != nil {
				b.Fatal(err)
			}
			vt, err := New(cols, rows, 8, 16, func([]byte) {})
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(vt.Close)
			var ops op.Ops
			gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(1600, 800))}
			for b.Loop() {
				b.StopTimer()
				vt.Write([]byte(screen))
				ops.Reset()
				b.StartTimer()
				if err := r.Measure(gtx); err != nil {
					b.Fatal(err)
				}
				s, err := vt.Update()
				if err != nil {
					b.Fatal(err)
				}
				if n, err := r.Draw(gtx, s); err != nil || n != rows {
					b.Fatalf("redrew %d rows, %v; want %d", n, err, rows)
				}
			}
		})
	}
}

// The snapshot of a full dirty ASCII screen and the composing of one of its
// rows, the two steps a grapheme's extra codepoints pass through.
func BenchmarkASCIISnapshotAndRow(b *testing.B) {
	const cols, rows = 200, 50
	line := strings.Repeat("The quick brown fox jumps over the lazy dog 0123456789 ", 4)[:cols]
	screen := []byte("\x1b[H" + strings.Repeat(line, rows))
	vt, err := New(cols, rows, 8, 16, func([]byte) {})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(vt.Close)
	b.Run("snapshot", func(b *testing.B) {
		for b.Loop() {
			b.StopTimer()
			vt.Write(screen)
			b.StartTimer()
			if _, err := vt.Update(); err != nil {
				b.Fatal(err)
			}
		}
	})
	r, err := NewRenderer(RenderRows, "monospace", 14)
	if err != nil {
		b.Fatal(err)
	}
	rr := r.(*rowRenderer)
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	if err := rr.Measure(gtx); err != nil {
		b.Fatal(err)
	}
	vt.Write(screen)
	s, err := vt.Update()
	if err != nil {
		b.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, cols*rr.r.CellW(), rr.r.CellH()))
	clear := rgba(s.s.bg)
	b.Run("compose row", func(b *testing.B) {
		for b.Loop() {
			if err := rr.compose(img, s.cells[:cols], s.s, -1, clear); err != nil {
				b.Fatal(err)
			}
		}
	})
}
