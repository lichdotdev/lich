package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

func TestOpeningAnotherFileShowsItsSource(t *testing.T) {
	view, err := codeview.New(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	p := newPreview(nil, view, func() {})
	p.openAt("README.md", 0)
	p.md.rendering = true
	p.openAt("docs/guide.md", 0)
	if p.md.rendering {
		t.Fatal("a newly opened file came up rendered, not as its source")
	}
}
