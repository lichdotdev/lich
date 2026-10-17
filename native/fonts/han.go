package fonts

import (
	"fmt"
	"os"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

// Han is the typeface the Chinese faces are registered under. No text asks
// for it by name: the shaper reaches it per rune, for the characters the
// interface's own faces lack.
const Han font.Typeface = "lich Han"

// hanProbe is the character a face must draw to count as covering Han.
const hanProbe = '中'

var installedHan struct {
	once  sync.Once
	faces []font.FontFace
	err   error
}

// InstalledHan reads, once, the installed faces that draw Simplified Chinese
// in the interface's weights, registered under Han; none when nothing
// installed covers it. Locating always answers with some face, so each is
// kept only if it really has a glyph for hanProbe.
func InstalledHan() ([]font.FontFace, error) {
	installedHan.once.Do(func() {
		installedHan.faces, installedHan.err = loadHan()
	})
	return installedHan.faces, installedHan.err
}

func loadHan() ([]font.FontFace, error) {
	var faces []font.FontFace
	parsed := map[Location]font.Face{}
	for _, w := range interfaceWeights {
		loc, err := locateHan(w)
		if err != nil {
			return nil, err
		}
		if loc.Path == "" {
			continue
		}
		face, ok := parsed[loc]
		if !ok {
			if face, err = parseLocation(loc); err != nil {
				return nil, err
			}
			parsed[loc] = face
		}
		if _, ok := face.Face().NominalGlyph(hanProbe); !ok {
			continue
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: Han, Weight: w}, Face: face})
	}
	return faces, nil
}

func parseLocation(loc Location) (font.Face, error) {
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return nil, err
	}
	parsed, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", loc.Path, err)
	}
	if loc.Index < 0 || loc.Index >= len(parsed) {
		return nil, fmt.Errorf("locate Han: face index %d is not one of the %d in %s", loc.Index, len(parsed), loc.Path)
	}
	return parsed[loc.Index].Face, nil
}

// coverWeights registers, for each interface weight faces lack, the face of
// the nearest weight they have, the heavier on a tie: the shaper falls back
// to a face of another family only in the weight asked for, so a missing
// weight draws Chinese as tofu.
func coverWeights(faces []font.FontFace) []font.FontFace {
	if len(faces) == 0 {
		return faces
	}
	have := map[font.Weight]bool{}
	for _, f := range faces {
		have[f.Font.Weight] = true
	}
	covered := faces
	for _, w := range interfaceWeights {
		if have[w] {
			continue
		}
		nearest := faces[0]
		for _, f := range faces[1:] {
			d, best := distance(f.Font.Weight, w), distance(nearest.Font.Weight, w)
			if d < best || d == best && f.Font.Weight > nearest.Font.Weight {
				nearest = f
			}
		}
		nearest.Font.Weight = w
		covered = append(covered, nearest)
	}
	return covered
}

func distance(a, b font.Weight) font.Weight {
	if a > b {
		return a - b
	}
	return b - a
}
