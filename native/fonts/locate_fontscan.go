//go:build windows || darwin

package fonts

import (
	"fmt"
	"log"
	"os"
	"slices"
	"sync"

	"gioui.org/font"
	gotext "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
)

// probeRune is the character a family is matched on, the one the terminal
// measures its cell by.
const probeRune = 'M'

// systemFonts indexes the system's font folders once (fontscan keeps the
// index in the user cache dir), behind a lock: a FontMap is not safe for
// concurrent use.
var systemFonts struct {
	once sync.Once
	mu   sync.Mutex
	fm   *fontscan.FontMap
	err  error
}

// withFontMap runs use on the system FontMap, indexing it on first use.
func withFontMap(use func(fm *fontscan.FontMap)) error {
	systemFonts.once.Do(func() {
		fm := fontscan.NewFontMap(log.Default())
		if err := fm.UseSystemFonts(""); err != nil {
			systemFonts.err = fmt.Errorf("index system fonts: %w", err)
			return
		}
		systemFonts.fm = fm
	})
	if systemFonts.err != nil {
		return systemFonts.err
	}
	systemFonts.mu.Lock()
	defer systemFonts.mu.Unlock()
	use(systemFonts.fm)
	return nil
}

// queryFamilies puts the platform's own faces ahead of an alias: fontscan
// expands SystemMono and SystemSans with fontconfig's Linux-first lists,
// which reach Courier New and Verdana on Windows before anything better.
func queryFamilies(family string) []string {
	switch family {
	case SystemMono:
		return append(slices.Clone(preferredMono), family)
	case SystemSans:
		return append(slices.Clone(preferredSans), family)
	}
	return []string{family}
}

func aspectOf(f font.Font) gotext.Aspect {
	style := gotext.StyleNormal
	if f.Style == font.Italic {
		style = gotext.StyleItalic
	}
	// Gio counts weight from Normal at 0, OpenType from 400.
	return gotext.Aspect{Style: style, Weight: gotext.WeightNormal + gotext.Weight(f.Weight)}
}

// Locate finds the installed face that draws f's family in its weight and
// style, or the nearest one the system has, as fontconfig would.
func Locate(f font.Font) (Location, error) {
	var loc Location
	var found bool
	err := withFontMap(func(fm *fontscan.FontMap) {
		fm.SetQuery(fontscan.Query{Families: queryFamilies(string(f.Typeface)), Aspect: aspectOf(f)})
		if face := fm.ResolveFace(probeRune); face != nil {
			id := fm.FontLocation(face.Font)
			loc, found = Location{Path: id.File, Index: int(id.Index)}, true
		}
	})
	if err != nil {
		return Location{}, err
	}
	if !found {
		return Location{}, fmt.Errorf("locate %s: no installed font", f.Typeface)
	}
	return loc, nil
}

// Covering names a font file that has a glyph for r, preferring the
// monospace ones, or "" when none does. The platform's emoji family is asked
// for by name: fontscan's fallback is by script, and an emoji's script is
// Common, which reaches no emoji font the way fontconfig's lists do on Linux.
func Covering(r rune) (string, error) {
	var path string
	err := withFontMap(func(fm *fontscan.FontMap) {
		fm.SetQuery(fontscan.Query{Families: append(queryFamilies(SystemMono), preferredEmoji...)})
		face := fm.ResolveFace(r)
		if face == nil {
			return
		}
		if _, ok := face.NominalGlyph(r); ok {
			path = fm.FontLocation(face.Font).File
		}
	})
	return path, err
}

type installedFamily struct {
	name string
	mono bool
}

var installed struct {
	once     sync.Once
	families []installedFamily
	err      error
}

// installedFamilies reads each installed family's name and spacing from one
// of its files, once: the index keeps names lowercased without spaces.
// ponytail: one font parse per family, a few hundred on a stock Windows;
// read the name table alone if opening the picker is ever felt.
func installedFamilies() ([]installedFamily, error) {
	installed.once.Do(func() {
		footprints, err := fontscan.SystemFonts(log.Default(), "")
		if err != nil {
			installed.err = fmt.Errorf("index system fonts: %w", err)
			return
		}
		seen := map[string]bool{}
		for _, fp := range footprints {
			if seen[fp.Family] {
				continue
			}
			seen[fp.Family] = true
			ft, err := parseFace(fp.Location)
			if err != nil {
				log.Printf("fonts: skip %s: %v", fp.Location.File, err)
				continue
			}
			installed.families = append(installed.families, installedFamily{name: ft.Describe().Family, mono: ft.IsMonospace()})
		}
	})
	return installed.families, installed.err
}

func parseFace(id fontscan.Location) (*gotext.Font, error) {
	file, err := os.Open(id.File)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	faces, err := gotext.ParseTTC(file)
	if err != nil {
		return nil, err
	}
	if int(id.Index) >= len(faces) {
		return nil, fmt.Errorf("face index %d is not one of the %d", id.Index, len(faces))
	}
	return faces[id.Index].Font, nil
}

// MonoFamilies lists, sorted, the installed families whose glyphs share one
// advance, which a terminal can draw on its grid.
func MonoFamilies() ([]string, error) {
	return familyNames(func(f installedFamily) bool { return f.mono })
}

// Families lists every installed family, sorted.
func Families() ([]string, error) {
	return familyNames(func(installedFamily) bool { return true })
}

func familyNames(keep func(installedFamily) bool) ([]string, error) {
	families, err := installedFamilies()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range families {
		if keep(f) {
			names = append(names, f.name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}
