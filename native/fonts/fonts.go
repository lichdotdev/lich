// Package fonts embeds Onest (SIL Open Font License 1.1, onest/OFL.txt), an
// interface face the native window offers, as static instances from Google
// Fonts: Gio picks a weight by face, not by a variable axis. It also finds
// installed families: through fontconfig on Linux and the BSDs, by scanning
// the system's font folders on Windows and macOS, which ship no fontconfig.
package fonts

import (
	"bytes"
	"embed"
	"fmt"
	"os"

	"gioui.org/font"
	"gioui.org/font/opentype"
	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
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

// The aliases for the system's own faces, fontconfig's names for them.
const (
	SystemSans = "sans-serif"
	SystemMono = "monospace"
)

// Location is one face of an installed font file: Index picks it among a
// collection's faces.
type Location struct {
	Path  string
	Index int
}

// interfaceWeights are the weights the interface draws in.
var interfaceWeights = []font.Weight{font.Normal, font.Medium, font.SemiBold, font.Bold}

// Load reads family's regular, medium, semibold and bold faces from the
// files Locate picks, registered under family; SystemSans names the
// system's own.
func Load(family string) ([]font.FontFace, error) {
	faces := make([]font.FontFace, 0, len(interfaceWeights))
	for _, w := range interfaceWeights {
		want := font.Font{Typeface: font.Typeface(family), Weight: w}
		loc, err := Locate(want)
		if err != nil {
			return nil, err
		}
		parsed, err := ParseFace(loc)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", family, err)
		}
		faces = append(faces, font.FontFace{Font: want, Face: parsed.Face})
	}
	return faces, nil
}

// ParseFace parses the one face loc names, under the family and style the face
// gives itself. Only that face: a collection holds several (a CJK one about
// ten), and opentype.ParseCollection parses them all, hundreds of megabytes
// allocated to keep one.
func ParseFace(loc Location) (font.FontFace, error) {
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return font.FontFace{}, err
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return font.FontFace{}, fmt.Errorf("parse %s: %w", loc.Path, err)
	}
	if loc.Index < 0 || loc.Index >= len(loaders) {
		return font.FontFace{}, fmt.Errorf("face index %d is not one of the %d in %s", loc.Index, len(loaders), loc.Path)
	}
	parsed, err := gotext.NewFont(loaders[loc.Index])
	if err != nil {
		return font.FontFace{}, fmt.Errorf("parse %s: %w", loc.Path, err)
	}
	return font.FontFace{Font: opentype.DescriptionToFont(parsed.Describe()), Face: collectionFace{parsed}}, nil
}

// collectionFace is a font.Face over one parsed face of a collection; like
// Gio's own opentype.Face, each call hands a shaper a face of its own.
type collectionFace struct{ font *gotext.Font }

func (f collectionFace) Face() *gotext.Face { return gotext.NewFace(f.font) }
