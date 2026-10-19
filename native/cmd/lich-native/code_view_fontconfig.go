//go:build !windows && !darwin

package main

import (
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
)

// newCodeView builds the code view in family by its fontconfig name, which
// keeps godemirror's own fallback for characters the family lacks.
func newCodeView(family string, size unit.Sp) (*godemirror.View, error) {
	return godemirror.New(family, size)
}

func newDiffView(family string, size unit.Sp, opts godemirror.DiffOptions) (*godemirror.DiffView, error) {
	return godemirror.NewDiffView(family, size, opts)
}
