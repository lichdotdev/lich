package fonts

import (
	"runtime"
	"testing"

	"gioui.org/font"
)

func TestFacesCoverEveryWeightTheWindowDraws(t *testing.T) {
	faces, err := Faces()
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != len(weights) {
		t.Fatalf("got %d faces, want %d", len(faces), len(weights))
	}
	for i, f := range faces {
		if f.Font.Typeface != Sans || f.Font.Weight != weights[i].weight {
			t.Errorf("face %d: %+v", i, f.Font)
		}
	}
}

func TestInstalledHanFacesDrawTheirLanguage(t *testing.T) {
	for _, language := range []string{"zh-CN", "ja", "ko"} {
		faces, err := InstalledHan(language)
		if err != nil {
			t.Fatal(err)
		}
		probe := hanProbes[cjkOf(language)]
		for _, f := range faces {
			if _, ok := f.Face.Face().NominalGlyph(probe); !ok || f.Font.Typeface != Han {
				t.Errorf("%s: %+v does not draw %c under Han", language, f.Font, probe)
			}
		}
	}
}

func TestEveryLanguageButJapaneseAndKoreanDrawsSimplifiedChinese(t *testing.T) {
	cases := map[string]cjk{"ja": japanese, "ja-JP": japanese, "ko": korean, "KO-kr": korean, "zh-CN": simplifiedChinese, "en": simplifiedChinese, "": simplifiedChinese}
	for language, want := range cases {
		if got := cjkOf(language); got != want {
			t.Errorf("cjkOf(%q) = %d, want %d", language, got, want)
		}
	}
}

func TestParseFaceReadsTheFaceLocated(t *testing.T) {
	loc, err := Locate(font.Font{Typeface: SystemMono})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseFace(loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Font.Typeface == "" {
		t.Errorf("%+v names no family", loc)
	}
	if _, ok := got.Face.Face().NominalGlyph('M'); !ok {
		t.Errorf("%+v does not draw M", loc)
	}
	loc.Index = 64
	if _, err := ParseFace(loc); err == nil {
		t.Errorf("face index 64 of %s was read", loc.Path)
	}
}

// The bold face fontconfig answers for both semibold and bold, and the
// interface and the terminal sharing a family, ask for one file several times:
// each ask while a face of it is held gets that face, not a parse of its own,
// and a face nobody holds any more is freed rather than kept for a next ask.
func TestParseFaceSharesAFaceWhileItIsHeld(t *testing.T) {
	loc, err := Locate(font.Font{Typeface: SystemMono})
	if err != nil {
		t.Fatal(err)
	}
	first, err := ParseFace(loc)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseFace(loc)
	if err != nil {
		t.Fatal(err)
	}
	if first.Face.(collectionFace).font != again.Face.(collectionFace).font {
		t.Fatal("a second ParseFace of a held face parsed the file again")
	}
	first, again = font.FontFace{}, font.FontFace{}
	runtime.GC()
	parsedFaces.Lock()
	held := parsedFaces.byLocation[loc].Value()
	parsedFaces.Unlock()
	if held != nil {
		t.Fatal("a face nobody holds was kept")
	}
}
