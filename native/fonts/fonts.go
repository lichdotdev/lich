// Package fonts embeds Onest (SIL Open Font License 1.1, onest/OFL.txt), an
// interface face the native window offers, as static instances from Google
// Fonts: Gio picks a weight by face, not by a variable axis. It also loads
// an installed family through fontconfig.
package fonts

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

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

// The fontconfig aliases for the system's own faces.
const (
	SystemSans = "sans-serif"
	SystemMono = "monospace"
)

// fontconfigWeights are fontconfig's numbers for the weights the interface
// draws in (fontconfig.h FC_WEIGHT_*).
var fontconfigWeights = []struct {
	fc     int
	weight font.Weight
}{
	{80, font.Normal},
	{100, font.Medium},
	{180, font.SemiBold},
	{200, font.Bold},
}

// Load reads family's regular, medium, semibold and bold faces through
// fontconfig, registered under family; SystemSans names the system's own.
// fontconfig answers a weight the family lacks with its nearest one.
func Load(family string) ([]font.FontFace, error) {
	faces := make([]font.FontFace, 0, len(fontconfigWeights))
	for _, w := range fontconfigWeights {
		out, err := exec.Command("fc-match", "-f", "%{file}\n%{index}", fmt.Sprintf("%s:weight=%d", family, w.fc)).Output()
		if err != nil {
			return nil, fmt.Errorf("fc-match %s: %w", family, err)
		}
		path, index, _ := strings.Cut(string(out), "\n")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		i, err := strconv.Atoi(strings.TrimSpace(index))
		if err != nil || i < 0 || i >= len(parsed) {
			return nil, fmt.Errorf("fc-match %s: face index %q is not one of the %d in %s", family, index, len(parsed), path)
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: font.Typeface(family), Weight: w.weight}, Face: parsed[i].Face})
	}
	return faces, nil
}
