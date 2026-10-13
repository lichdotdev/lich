// Package fonts embeds the interface face of the native window, Onest
// (SIL Open Font License 1.1, onest/OFL.txt), as static instances from
// Google Fonts: Gio picks a weight by face, not by a variable axis.
package fonts

import (
	"embed"
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

// Sans is the typeface the faces are registered under.
const Sans font.Typeface = "Onest"

//go:embed onest/*.ttf
var onest embed.FS

var weights = []struct {
	file   string
	weight font.Weight
}{
	{"Onest-Regular.ttf", font.Normal},
	{"Onest-Medium.ttf", font.Medium},
	{"Onest-SemiBold.ttf", font.SemiBold},
	{"Onest-Bold.ttf", font.Bold},
}

// Faces are Onest's weights under Sans, named by the weight they are rather
// than by each file's own family name, which differs per instance.
func Faces() ([]font.FontFace, error) {
	faces := make([]font.FontFace, 0, len(weights))
	for _, w := range weights {
		data, err := onest.ReadFile("onest/" + w.file)
		if err != nil {
			return nil, err
		}
		face, err := opentype.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", w.file, err)
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: Sans, Weight: w.weight}, Face: face})
	}
	return faces, nil
}
