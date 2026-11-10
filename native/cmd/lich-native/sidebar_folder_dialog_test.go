package main

import (
	"image"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

// The dialog takes the focus for its panel the frame it opens, so the field's
// has to be asked for after it.
func TestTheFolderDialogOpensWithTheFieldFocused(t *testing.T) {
	var s sidebar
	rig := newFrameRig(image.Pt(1000, 800), func(gtx C, th *ui.Theme) {
		s.layoutFolderDialog(gtx, th, nil)
	})
	rig.frame()
	s.askFolderName([]string{"s1"})
	for range 3 {
		rig.frame()
		rig.now = rig.now.Add(50 * time.Millisecond)
	}
	if !rig.router.Source().Focused(&s.naming.name) {
		t.Fatal("the name field does not have the keyboard")
	}
}
