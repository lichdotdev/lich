package icons

import (
	"image"
	"image/color"
	"testing"
)

func TestDeviconForPicksByNameThenExtension(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"vc/src/main/Pacote.java", "JavaOriginal"},
		{"build.gradle", "GradleOriginal"},
		{"src/App.TSX", "TypescriptOriginal"},
		{"dbscripts/V1__init.sql", "Database"},
		{"deploy/Dockerfile", "DockerOriginal"},
		{".gitignore", "GitOriginal"},
		{"gradlew", ""},
		{".env", ""},
		{"notes.txt", ""},
	}
	for _, tc := range cases {
		got := DeviconFor(tc.path)
		if tc.want == "" {
			if got != nil {
				t.Errorf("%s: got an icon, want none", tc.path)
			}
			continue
		}
		if got == nil || got != deviconBitmaps[tc.want] {
			t.Errorf("%s: want %s", tc.path, tc.want)
		}
	}
}

func TestEveryManifestIconHasItsPNG(t *testing.T) {
	m, err := devicons()
	if err != nil {
		t.Fatal(err)
	}
	for ext := range m.Extensions {
		if DeviconFor("f."+ext) == nil {
			t.Errorf(".%s: no icon", ext)
		}
	}
	for name := range m.Names {
		if DeviconFor(name) == nil {
			t.Errorf("%s: no icon", name)
		}
	}
}

func TestDeviconKeepsItsColors(t *testing.T) {
	img := DeviconFor("main.py").render(28, color.NRGBA{A: 255}).(*image.NRGBA)
	blue, yellow := false, false
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		if a < 200 {
			continue
		}
		blue = blue || (b > r+40 && b > g)
		yellow = yellow || (r > b+80 && g > b+80)
	}
	if !blue || !yellow {
		t.Fatalf("python mark lost its brand colors: blue %v, yellow %v", blue, yellow)
	}
}

// Markdown's devicon sets no fill and renders black, as FileIcon.tsx's mono
// flag says; it must take the color it is given.
func TestInkOnlyDeviconTakesTheGivenColor(t *testing.T) {
	want := color.NRGBA{R: 200, G: 10, B: 90, A: 255}
	img := DeviconFor("README.md").render(28, want).(*image.NRGBA)
	painted := 0
	for i := 0; i < len(img.Pix); i += 4 {
		p := img.Pix[i : i+4]
		if p[3] == 0 {
			continue
		}
		painted++
		if p[0] != want.R || p[1] != want.G || p[2] != want.B {
			t.Fatalf("pixel %v, want the color %v", p, want)
		}
	}
	if painted == 0 {
		t.Fatal("mono mark drew nothing")
	}
}

func TestInkOnlyTellsBlackMarksFromColoredOnes(t *testing.T) {
	for _, p := range []string{"a.md", "a.rs"} {
		if !DeviconFor(p).mono {
			t.Errorf("%s: want a mono mark", p)
		}
	}
	for _, p := range []string{"a.py", "a.go", "a.java", "a.sh"} {
		if DeviconFor(p).mono {
			t.Errorf("%s: colored mark taken for mono", p)
		}
	}
}
