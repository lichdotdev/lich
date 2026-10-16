// Package fonts embeds Onest (SIL Open Font License 1.1, onest/OFL.txt), an
// interface face the native window offers, as static instances from Google
// Fonts: Gio picks a weight by face, not by a variable axis. It also finds
// installed families: through fontconfig on Linux and the BSDs, by scanning
// the system's font folders on Windows and macOS, which ship no fontconfig.
package fonts

import (
	"embed"
	"fmt"
	"os"

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
		data, err := os.ReadFile(loc.Path)
		if err != nil {
			return nil, err
		}
		parsed, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", loc.Path, err)
		}
		if loc.Index < 0 || loc.Index >= len(parsed) {
			return nil, fmt.Errorf("locate %s: face index %d is not one of the %d in %s", family, loc.Index, len(parsed), loc.Path)
		}
		faces = append(faces, font.FontFace{Font: want, Face: parsed[loc.Index].Face})
	}
	return faces, nil
}
