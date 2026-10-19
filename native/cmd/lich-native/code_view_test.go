package main

import (
	"testing"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
)

func TestTheCodeAndDiffViewsOpenInTheSystemMonospace(t *testing.T) {
	if _, err := newCodeView(fonts.SystemMono, 13); err != nil {
		t.Fatalf("code view: %v", err)
	}
	if _, err := newDiffView(fonts.SystemMono, 13, godemirror.DiffOptions{}); err != nil {
		t.Fatalf("diff view: %v", err)
	}
}
