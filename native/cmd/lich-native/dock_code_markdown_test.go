package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/fonts"
)

func TestOpeningAnotherFileShowsItsSource(t *testing.T) {
	view, err := newCodeView(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	p := newPreview(nil, view, fonts.SystemMono, 13, func() {})
	p.openAt("README.md", 0)
	p.md.rendering = true
	p.openAt("docs/guide.md", 0)
	if p.md.rendering {
		t.Fatal("a newly opened file came up rendered, not as its source")
	}
}
