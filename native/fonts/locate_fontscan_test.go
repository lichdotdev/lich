//go:build windows || darwin

package fonts

import (
	"os"
	"slices"
	"testing"

	"gioui.org/font"
)

func TestLocateFindsTheSystemMonospaceInEveryStyle(t *testing.T) {
	for _, style := range []font.Font{{}, {Weight: font.Bold}, {Style: font.Italic}, {Weight: font.Bold, Style: font.Italic}} {
		style.Typeface = SystemMono
		loc, err := Locate(style)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(loc.Path); err != nil {
			t.Errorf("%+v: %v", style, err)
		}
	}
}

func TestLoadReadsEveryInterfaceWeightOfTheSystemSans(t *testing.T) {
	faces, err := Load(SystemSans)
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != len(interfaceWeights) {
		t.Fatalf("got %d faces, want %d", len(faces), len(interfaceWeights))
	}
}

func TestMonoFamiliesAreAmongTheFamilies(t *testing.T) {
	mono, err := MonoFamilies()
	if err != nil {
		t.Fatal(err)
	}
	all, err := Families()
	if err != nil {
		t.Fatal(err)
	}
	if len(mono) == 0 {
		t.Fatal("no monospace family installed")
	}
	for _, name := range mono {
		if !slices.Contains(all, name) {
			t.Errorf("%q is mono but not listed", name)
		}
	}
}

func TestCoveringFindsAFontForALatinLetter(t *testing.T) {
	path, err := Covering('a')
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("no font covers 'a'")
	}
}
