package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestDecidePatchNotes(t *testing.T) {
	notes := backend.PatchNotes{Version: "1.4.0", Groups: []backend.PatchNotesGroup{{Label: "Added"}}}
	cases := []struct {
		name     string
		notes    backend.PatchNotes
		lastSeen string
		want     patchNotesAction
	}{
		{"dev build without a section", backend.PatchNotes{}, "1.3.0", patchNotesNone},
		{"section without groups", backend.PatchNotes{Version: "1.4.0"}, "1.3.0", patchNotesNone},
		{"first launch ever", notes, "", patchNotesRecord},
		{"already seen", notes, "1.4.0", patchNotesNone},
		{"new release", notes, "1.3.0", patchNotesShow},
	}
	for _, c := range cases {
		if got := decidePatchNotes(c.notes, c.lastSeen); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
