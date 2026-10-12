package ui

import (
	"image/color"
	"testing"
)

func TestParseColor(t *testing.T) {
	cases := map[string]color.NRGBA{
		"#e0def4":                    {R: 0xe0, G: 0xde, B: 0xf4, A: 255},
		"#FFF":                       {R: 255, G: 255, B: 255, A: 255},
		"#11223380":                  {R: 0x11, G: 0x22, B: 0x33, A: 0x80},
		"#1238":                      {R: 0x11, G: 0x22, B: 0x33, A: 0x88},
		"transparent":                {},
		" White ":                    {R: 255, G: 255, B: 255, A: 255},
		"rgb(255, 0, 128)":           {R: 255, B: 128, A: 255},
		"rgba(255,0,0,0.5)":          {R: 255, A: 128},
		"rgb(100% 0% 0% / 50%)":      {R: 255, A: 128},
		"hsl(120, 100%, 50%)":        {G: 255, A: 255},
		"hsl(0 0 100)":               {R: 255, G: 255, B: 255, A: 255},
		"oklch(0.141 0.005 285.823)": OKLCH(0.141, 0.005, 285.823, 1),
		"oklch(1 0 0 / 10%)":         OKLCH(1, 0, 0, 0.1),
		"oklch(62.8% 50% 29deg)":     OKLCH(0.628, 0.2, 29, 1),
		"oklab(0.5 0 0)":             OKLCH(0.5, 0, 0, 1),
	}
	for css, want := range cases {
		got, err := ParseColor(css)
		if err != nil {
			t.Errorf("ParseColor(%q): %v", css, err)
			continue
		}
		if got != want {
			t.Errorf("ParseColor(%q) = %v, want %v", css, got, want)
		}
	}
}

// What the theme format allows but this parser does not draw is an error
// naming the value, never a guessed color.
func TestParseColorRefuses(t *testing.T) {
	for _, css := range []string{"rebeccapurple", "lab(50 20 30)", "color(srgb 1 0 0)", "#12", "#ggg", "rgb(1 2)", "oklch(a b c)", "rgb(1 2 3"} {
		if c, err := ParseColor(css); err == nil {
			t.Errorf("ParseColor(%q) = %v, want an error", css, c)
		}
	}
}
