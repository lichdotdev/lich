package main

import "testing"

// A file outside the checkout opened to edit saves in its own folder.
func TestAFileOutsideTheCheckoutSavesInPlace(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\n")
	f.openOutside("/home/me/.claude", "CLAUDE.md")
	f.loaded = f.opens
	if err := f.setText("one\n"); err != nil {
		t.Fatal(err)
	}
	typeAt(t, f, 0, "x")
	if !f.dirty || !f.editable() {
		t.Fatalf("dirty %v editable %v after typing", f.dirty, f.editable())
	}
	f.startSave(nil, true)
	w := <-d.writes
	if w.Path != "/home/me/.claude" || w.Rel != "CLAUDE.md" || w.Content != "xone\n" {
		t.Errorf("wrote %+v", w)
	}
}
