package icons

import "testing"

// A shape's path data parses back into the same segments.
func TestSVGPathsRoundTrip(t *testing.T) {
	for _, ic := range []*Icon{BibataArrow(), Lucide("terminal"), Lobe("claude")} {
		paths := ic.SVGPaths()
		if len(paths) != len(ic.shapes) {
			t.Fatalf("%d paths for %d shapes", len(paths), len(ic.shapes))
		}
		for i, p := range paths {
			back, err := fillOnly(p.D)
			if err != nil {
				t.Fatalf("path %q: %v", p.D, err)
			}
			want, got := ic.shapes[i].segs, back.shapes[0].segs
			if len(got) != len(want) {
				t.Fatalf("path %q parsed into %d segments, want %d", p.D, len(got), len(want))
			}
			for j := range want {
				if got[j] != want[j] {
					t.Fatalf("segment %d: %+v, want %+v", j, got[j], want[j])
				}
			}
			if p.Fill != ic.shapes[i].fill || p.Stroke != ic.shapes[i].stroke {
				t.Fatalf("path %d paints fill %v stroke %v, want %+v", i, p.Fill, p.Stroke, ic.shapes[i])
			}
		}
	}
}
