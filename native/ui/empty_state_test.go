package ui

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/native/ui/icons"
)

func TestEmptyStateOmitsWhatItIsNotGiven(t *testing.T) {
	th := testTheme()
	full := layoutAt(th.EmptyState(icons.Lucide("file"), "Title", "Words", func(gtx C) D { return D{Size: image.Pt(10, 40)} }).Layout, 300, 300)
	bare := layoutAt(th.EmptyState(nil, "Title", "", nil).Layout, 300, 300)
	if bare.Size.Y >= full.Size.Y {
		t.Errorf("bare is %d tall, full %d: a missing glyph, line or action left its room", bare.Size.Y, full.Size.Y)
	}
}
