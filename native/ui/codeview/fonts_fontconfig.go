//go:build !windows && !darwin

package codeview

import (
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
)

// NewEditor builds a godemirror view in family by its fontconfig name, which
// keeps godemirror's own fallback for characters the family lacks.
func NewEditor(family string, size unit.Sp) (*godemirror.View, error) {
	return godemirror.New(family, size)
}

func NewDiffEditor(family string, size unit.Sp, opts godemirror.DiffOptions) (*godemirror.DiffView, error) {
	return godemirror.NewDiffView(family, size, opts)
}
