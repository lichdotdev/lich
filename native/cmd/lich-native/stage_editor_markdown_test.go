package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/fonts"
)

func TestOpeningAnotherFileShowsItsSource(t *testing.T) {
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("README.md", 0, 0, agentNote{})
	f.md.rendering = true
	f.open("docs/guide.md", 0, 0, agentNote{})
	if f.md.rendering {
		t.Fatal("a newly opened file came up rendered, not as its source")
	}
}
