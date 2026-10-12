package ui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
)

// CSS's reference ranges: the value 100% stands for in each component.
const (
	okChromaFull = 0.4 // oklch C and oklab a/b
	rgbFull      = 255
)

var cssKeywords = map[string]color.NRGBA{
	"transparent": {},
	"black":       {A: 255},
	"white":       {R: 255, G: 255, B: 255, A: 255},
}

// ParseColor reads a CSS color as a theme file writes one: #hex, rgb(),
// rgba(), hsl(), hsla(), oklch(), oklab(), or transparent, black and white.
// lab(), lch(), color() and the other named colors are refused by name: the
// theme format allows them, and a theme that uses one fails here rather than
// drawing in a guessed color.
func ParseColor(css string) (color.NRGBA, error) {
	s := strings.ToLower(strings.TrimSpace(css))
	if c, ok := cssKeywords[s]; ok {
		return c, nil
	}
	if strings.HasPrefix(s, "#") {
		return parseHex(s[1:])
	}
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return color.NRGBA{}, fmt.Errorf("unsupported color %q", css)
	}
	args, alpha, err := cssArgs(s[open+1 : len(s)-1])
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("color %q: %w", css, err)
	}
	var c color.NRGBA
	switch s[:open] {
	case "rgb", "rgba":
		c, err = rgbArgs(args, alpha)
	case "hsl", "hsla":
		c, err = hslArgs(args, alpha)
	case "oklch":
		c, err = oklchArgs(args, alpha)
	case "oklab":
		c, err = oklabArgs(args, alpha)
	default:
		err = fmt.Errorf("unsupported function %s()", s[:open])
	}
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("color %q: %w", css, err)
	}
	return c, nil
}

func parseHex(h string) (color.NRGBA, error) {
	if len(h) == 3 || len(h) == 4 {
		var long strings.Builder
		for _, r := range h {
			long.WriteString(strings.Repeat(string(r), 2))
		}
		h = long.String()
	}
	if len(h) == 6 {
		h += "ff"
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if len(h) != 8 || err != nil {
		return color.NRGBA{}, fmt.Errorf("bad hex color #%s", h)
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}

// cssArgs splits a function's arguments, legacy commas or modern spaces,
// and returns the alpha after "/" or as a fourth argument (1 when absent).
func cssArgs(body string) ([]string, float64, error) {
	channels, alphaPart, slashed := strings.Cut(body, "/")
	args := strings.FieldsFunc(channels, func(r rune) bool { return r == ',' || r == ' ' })
	if slashed {
		args = append(args, strings.TrimSpace(alphaPart))
	}
	if len(args) == 3 {
		return args, 1, nil
	}
	if len(args) != 4 {
		return nil, 0, fmt.Errorf("want 3 components and an optional alpha, got %d", len(args))
	}
	alpha, err := component(args[3], 1)
	return args[:3], math.Max(0, math.Min(1, alpha)), err
}

// component is a number, or a percentage of full.
func component(arg string, full float64) (float64, error) {
	if p, ok := strings.CutSuffix(arg, "%"); ok {
		v, err := strconv.ParseFloat(p, 64)
		return v / 100 * full, err
	}
	return strconv.ParseFloat(arg, 64)
}

func hue(arg string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSuffix(arg, "deg"), 64)
}

func components(args []string, full [3]float64) ([3]float64, error) {
	var out [3]float64
	for i, a := range args {
		v, err := component(a, full[i])
		if err != nil {
			return out, fmt.Errorf("bad component %q", a)
		}
		out[i] = v
	}
	return out, nil
}

func rgbArgs(args []string, alpha float64) (color.NRGBA, error) {
	v, err := components(args, [3]float64{rgbFull, rgbFull, rgbFull})
	if err != nil {
		return color.NRGBA{}, err
	}
	return rgbColor(v[0], v[1], v[2], alpha), nil
}

func rgbColor(r, g, b, alpha float64) color.NRGBA {
	channel := func(x float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(rgbFull, x)))) }
	return color.NRGBA{R: channel(r), G: channel(g), B: channel(b), A: uint8(math.Round(alpha * 255))}
}

// hslArgs takes saturation and lightness as percentages, or as the bare
// 0-100 numbers modern CSS also accepts.
func hslArgs(args []string, alpha float64) (color.NRGBA, error) {
	h, err := hue(args[0])
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("bad hue %q", args[0])
	}
	v, err := components(args[1:], [3]float64{100, 100})
	if err != nil {
		return color.NRGBA{}, err
	}
	sat, light := v[0]/100, v[1]/100
	f := func(n float64) float64 {
		k := math.Mod(n+h/30, 12)
		a := sat * math.Min(light, 1-light)
		return (light - a*math.Max(-1, math.Min(k-3, math.Min(9-k, 1)))) * rgbFull
	}
	return rgbColor(f(0), f(8), f(4), alpha), nil
}

func oklchArgs(args []string, alpha float64) (color.NRGBA, error) {
	v, err := components(args[:2], [3]float64{1, okChromaFull})
	if err != nil {
		return color.NRGBA{}, err
	}
	h, err := hue(args[2])
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("bad hue %q", args[2])
	}
	return OKLCH(v[0], v[1], h, alpha), nil
}

func oklabArgs(args []string, alpha float64) (color.NRGBA, error) {
	v, err := components(args, [3]float64{1, okChromaFull, okChromaFull})
	if err != nil {
		return color.NRGBA{}, err
	}
	return fromOKLab(v, alpha), nil
}
