// Package codeview draws source text read-only: a godemirror view that picks
// its grammar from the filename, with lich's style tags and fonts.
package codeview

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
)

// StyleTags tags more of the code than CodeMirror's grammars do, so a
// theme can color it as an IDE does: Java's enum constants, annotations,
// fields and import paths, parameter declarations, and each language's
// builtins.
var StyleTags = []godemirror.StyleTags{
	godemirror.RicherJavaTags(),
	godemirror.RicherGoTags(), godemirror.GoBuiltinTags(),
	godemirror.RicherJavaScriptTags(), godemirror.JavaScriptBuiltinTags(),
	godemirror.RicherPythonTags(), godemirror.PythonBuiltinTags(),
	godemirror.RicherRustTags(), godemirror.RustBuiltinTags(),
}

// OnSurface is the code palette on the theme's surface.
func OnSurface(th *ui.Theme, code godemirror.Theme) godemirror.Theme {
	code.Background, code.GutterBackground = th.Surface, th.Surface
	return code
}

// Source is what a View shows; the Filename's extension picks the grammar.
type Source struct {
	Filename string
	Text     string
	// Blocks hang widgets under lines; nil for none.
	Blocks []godemirror.BlockWidget
}

func (s Source) content() godemirror.Content {
	return godemirror.Content{Filename: s.Filename, Text: s.Text, StyleTags: StyleTags, Blocks: s.Blocks}
}

// View is a read-only code view. The embedded editor answers selection,
// scrolling, search and focus; focus is on the editor, View.View.
type View struct {
	*godemirror.View
	family string
	size   unit.Sp
}

// New is a View in family at size.
func New(family string, size unit.Sp) (*View, error) {
	view, err := NewEditor(family, size)
	if err != nil {
		return nil, err
	}
	return &View{View: view, family: family, size: size}, nil
}

// SetSource shows s from its top.
func (v *View) SetSource(s Source) { v.View.SetContent(s.content()) }

// UpdateSource shows s keeping the scroll and the selection.
func (v *View) UpdateSource(s Source) { v.View.UpdateContent(s.content()) }

// SetFont swaps the editor for one in family at size and reports whether it
// did. The new editor is empty: the caller gives it its source again.
func (v *View) SetFont(family string, size unit.Sp) (bool, error) {
	if family == v.family && size == v.size {
		return false, nil
	}
	view, err := NewEditor(family, size)
	if err != nil {
		return false, err
	}
	v.View, v.family, v.size = view, family, size
	return true, nil
}

// Layout draws the source, its background th's surface.
func (v *View) Layout(gtx layout.Context, th *ui.Theme, code godemirror.Theme) (layout.Dimensions, error) {
	v.View.Theme = OnSurface(th, code)
	return v.View.Layout(gtx)
}
