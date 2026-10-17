//go:build !windows && !darwin

package fonts

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"gioui.org/font"
)

// fontconfigWeights are fontconfig's numbers for the weights the window
// draws in (fontconfig.h FC_WEIGHT_*).
var fontconfigWeights = map[font.Weight]int{
	font.Normal:   80,
	font.Medium:   100,
	font.SemiBold: 180,
	font.Bold:     200,
}

// Locate asks fontconfig for the face that draws f's family in its weight
// and style. fontconfig answers a weight or style the family lacks, or a
// family not installed, with its nearest.
func Locate(f font.Font) (Location, error) {
	weight, ok := fontconfigWeights[f.Weight]
	if !ok {
		return Location{}, fmt.Errorf("locate %s: no fontconfig weight for %v", f.Typeface, f.Weight)
	}
	pattern := fmt.Sprintf("%s:weight=%d", f.Typeface, weight)
	if f.Style == font.Italic {
		pattern += ":italic"
	}
	out, err := exec.Command("fc-match", "-f", "%{file}\n%{index}", pattern).Output()
	if err != nil {
		return Location{}, fmt.Errorf("fc-match %s: %w", pattern, err)
	}
	path, index, _ := strings.Cut(string(out), "\n")
	i, err := strconv.Atoi(strings.TrimSpace(index))
	if err != nil {
		return Location{}, fmt.Errorf("fc-match %s: face index %q: %w", pattern, index, err)
	}
	return Location{Path: path, Index: i}, nil
}

// preferredHan are the families asked for by name before fontconfig's own
// pick for the language, which on a stock install may be a face of another
// region (WenQuanYi for Japanese) or one with no CJK at all at a weight.
var preferredHan = [...][]string{
	simplifiedChinese: {"Noto Sans CJK SC", "Noto Sans SC", "Source Han Sans SC"},
	japanese:          {"Noto Sans CJK JP", "Noto Sans JP", "Source Han Sans JP"},
	korean:            {"Noto Sans CJK KR", "Noto Sans KR", "Source Han Sans KR"},
}

// fontconfigLangs are fontconfig's names for each CJK language.
var fontconfigLangs = [...]string{simplifiedChinese: "zh-cn", japanese: "ja", korean: "ko"}

// locateHan is the face for c in weight w: the first of its preferred
// families installed, else fontconfig's pick for its language, which may not
// cover it at all when nothing installed does.
func locateHan(c cjk, w font.Weight) (Location, error) {
	for _, family := range preferredHan[c] {
		loc, err := locateFamily(family, w)
		if err != nil || loc.Path != "" {
			return loc, err
		}
	}
	return Locate(font.Font{Typeface: font.Typeface(SystemSans + ":lang=" + fontconfigLangs[c]), Weight: w})
}

// locateFamily is family's face in weight w, or no Location when family is
// not installed and fontconfig would answer with another.
func locateFamily(family string, w font.Weight) (Location, error) {
	pattern := fmt.Sprintf("%s:weight=%d", family, fontconfigWeights[w])
	out, err := exec.Command("fc-match", "-f", "%{family}\n%{file}\n%{index}", pattern).Output()
	if err != nil {
		return Location{}, fmt.Errorf("fc-match %s: %w", pattern, err)
	}
	return parseFamilyMatch(string(out), family)
}

// parseFamilyMatch reads `fc-match -f '%{family}\n%{file}\n%{index}'`, kept
// only when one of the matched family's names is family.
func parseFamilyMatch(out, family string) (Location, error) {
	names, rest, _ := strings.Cut(out, "\n")
	path, index, _ := strings.Cut(rest, "\n")
	if !slices.Contains(strings.Split(names, ","), family) {
		return Location{}, nil
	}
	i, err := strconv.Atoi(strings.TrimSpace(index))
	if err != nil {
		return Location{}, fmt.Errorf("fc-match %s: face index %q: %w", family, index, err)
	}
	return Location{Path: path, Index: i}, nil
}

// Covering names a font file that has a glyph for r, the closest to a plain
// monospace among those that do, or "" when none does.
func Covering(r rune) (string, error) {
	charset := fmt.Sprintf(":charset=%x", r)
	out, err := exec.Command("fc-list", charset, "file").Output()
	if err != nil {
		return "", fmt.Errorf("fc-list %s: %w", charset, err)
	}
	covering := parseFcListFiles(string(out))
	if len(covering) == 0 {
		return "", nil
	}
	// fc-list knows who covers the character but in no useful order; fc-match
	// ranks every font against a plain monospace, so the first covering font
	// in its ranking is the closest regular face.
	ranked, err := exec.Command("fc-match", "-s", "-f", "%{file}\n", SystemMono).Output()
	if err != nil {
		return "", fmt.Errorf("fc-match -s %s: %w", SystemMono, err)
	}
	for f := range strings.Lines(string(ranked)) {
		if f = strings.TrimSpace(f); slices.Contains(covering, f) {
			return f, nil
		}
	}
	return covering[0], nil
}

// parseFcListFiles reads `fc-list <pattern> file`, one "path: " per line.
func parseFcListFiles(out string) []string {
	var files []string
	for line := range strings.Lines(out) {
		if f := strings.TrimSuffix(strings.TrimSpace(line), ":"); f != "" {
			files = append(files, f)
		}
	}
	return files
}

// MonoFamilies lists, sorted, the installed families fontconfig marks
// monospace or dual-width (the Nerd Font families that are not "Mono"),
// which a terminal can draw on its grid.
func MonoFamilies() ([]string, error) {
	seen := map[string]bool{}
	for _, spacing := range []string{":spacing=mono", ":spacing=dual"} {
		out, err := exec.Command("fc-list", spacing, "family").Output()
		if err != nil {
			return nil, fmt.Errorf("fc-list %s: %w", spacing, err)
		}
		for _, name := range parseFamilyLines(string(out)) {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// Families lists every installed family, sorted.
func Families() ([]string, error) {
	out, err := exec.Command("fc-list", ":", "family").Output()
	if err != nil {
		return nil, fmt.Errorf("fc-list: %w", err)
	}
	names := parseFamilyLines(string(out))
	slices.Sort(names)
	return slices.Compact(names), nil
}

// parseFamilyLines reads fc-list's family lines: a line may hold
// comma-separated localized aliases, and the first is the family's name.
func parseFamilyLines(out string) []string {
	var names []string
	for line := range strings.SplitSeq(out, "\n") {
		if name := strings.TrimSpace(strings.SplitN(line, ",", 2)[0]); name != "" {
			names = append(names, name)
		}
	}
	return names
}
