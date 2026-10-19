//go:build windows || darwin

package main

import (
	"os"

	"gioui.org/font"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
)

// newCodeView builds the code view from the files fonts.Locate picks for
// family: godemirror resolves a family name only through fontconfig's fc-match,
// which Windows and macOS do not ship. A character the family lacks is drawn
// as a box.
func newCodeView(family string, size unit.Sp) (*godemirror.View, error) {
	files, err := codeFonts(family)
	if err != nil {
		return nil, err
	}
	return godemirror.NewFromFonts(files, size)
}

func newDiffView(family string, size unit.Sp, opts godemirror.DiffOptions) (*godemirror.DiffView, error) {
	files, err := codeFonts(family)
	if err != nil {
		return nil, err
	}
	return godemirror.NewDiffViewFromFonts(files, size, opts)
}

// codeFonts reads family's four styles. godemirror draws a collection's first
// face, so a style fonts.Locate finds deeper in a collection is drawn with
// that one, as the terminal's rasterizer does.
func codeFonts(family string) (godemirror.Fonts, error) {
	read := map[string][]byte{}
	load := func(weight font.Weight, style font.Style) ([]byte, error) {
		loc, err := fonts.Locate(font.Font{Typeface: font.Typeface(family), Weight: weight, Style: style})
		if err != nil {
			return nil, err
		}
		if data, ok := read[loc.Path]; ok {
			return data, nil
		}
		data, err := os.ReadFile(loc.Path)
		if err != nil {
			return nil, err
		}
		read[loc.Path] = data
		return data, nil
	}
	var files godemirror.Fonts
	var err error
	if files.Regular, err = load(font.Normal, font.Regular); err != nil {
		return files, err
	}
	if files.Bold, err = load(font.Bold, font.Regular); err != nil {
		return files, err
	}
	if files.Italic, err = load(font.Normal, font.Italic); err != nil {
		return files, err
	}
	files.BoldItalic, err = load(font.Bold, font.Italic)
	return files, err
}
