package fonts

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"

	"gioui.org/font"
	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

// Han is the typeface the CJK faces are registered under. No text asks for
// it by name: the shaper reaches it per rune, for the characters the
// interface's own faces lack.
const Han font.Typeface = "lich Han"

// cjk is a CJK language's own faces: Chinese, Japanese and Korean draw the
// same Han characters in different shapes, and Japanese and Korean need
// their own script besides.
type cjk int

const (
	simplifiedChinese cjk = iota
	japanese
	korean
)

// cjkOf is the CJK faces the interface in language draws with: its own for
// Japanese and Korean, Simplified Chinese's for every other language.
func cjkOf(language string) cjk {
	tag, _, _ := strings.Cut(strings.ToLower(language), "-")
	switch tag {
	case "ja":
		return japanese
	case "ko":
		return korean
	}
	return simplifiedChinese
}

// hanProbes are the characters a face must draw to count as covering each
// language: Han for Chinese, kana for Japanese, Hangul for Korean.
var hanProbes = [...]rune{simplifiedChinese: '中', japanese: 'あ', korean: '한'}

var installedHan [len(hanProbes)]struct {
	once  sync.Once
	faces []font.FontFace
	err   error
}

// InstalledHan reads, once per CJK language, the installed faces that draw
// the interface's language in the interface's weights, registered under Han;
// none when nothing installed covers it. A language that is not Japanese or
// Korean reads Simplified Chinese's. Locating always answers with some face,
// so each is kept only if it really has a glyph for the language's probe.
func InstalledHan(language string) ([]font.FontFace, error) {
	c := cjkOf(language)
	installed := &installedHan[c]
	installed.once.Do(func() {
		installed.faces, installed.err = loadHan(c)
	})
	return installed.faces, installed.err
}

func loadHan(c cjk) ([]font.FontFace, error) {
	var faces []font.FontFace
	parsed := map[Location]font.Face{}
	for _, w := range interfaceWeights {
		loc, err := locateHan(c, w)
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
		if _, ok := face.Face().NominalGlyph(hanProbes[c]); !ok {
			continue
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: Han, Weight: w}, Face: face})
	}
	return faces, nil
}

// parseLocation parses the one face loc names: a CJK collection holds about
// ten, and opentype.ParseCollection parsed all of them to keep one, hundreds
// of megabytes allocated at every start.
func parseLocation(loc Location) (font.Face, error) {
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return nil, err
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", loc.Path, err)
	}
	if loc.Index < 0 || loc.Index >= len(loaders) {
		return nil, fmt.Errorf("locate Han: face index %d is not one of the %d in %s", loc.Index, len(loaders), loc.Path)
	}
	parsed, err := gotext.NewFont(loaders[loc.Index])
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", loc.Path, err)
	}
	return collectionFace{parsed}, nil
}

// collectionFace is a font.Face over one parsed face of a collection; like
// Gio's own opentype.Face, each call hands a shaper a face of its own.
type collectionFace struct{ font *gotext.Font }

func (f collectionFace) Face() *gotext.Face { return gotext.NewFace(f.font) }

// coverWeights registers, for each interface weight faces lack, the face of
// the nearest weight they have, the heavier on a tie: the shaper falls back
// to a face of another family only in the weight asked for, so a missing
// weight draws CJK as tofu.
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
