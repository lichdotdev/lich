package markdown

import (
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/x/richtext"
	"github.com/yuin/goldmark/ast"

	"github.com/lichdotdev/lich/native/ui"
)

// diffTone is what a line of a diff block is.
type diffTone uint8

const (
	diffContext diffTone = iota
	diffAdded
	diffDeleted
	diffHeader
)

// diffRowMix is DESIGN.md's tint of an added or removed row.
const diffRowMix = 0.12

// diffTones is the tone of each line of a fence named diff or patch, nil for
// any other block.
func diffTones(source []byte, n ast.Node) []diffTone {
	fenced, ok := n.(*ast.FencedCodeBlock)
	if !ok {
		return nil
	}
	if lang := strings.ToLower(string(fenced.Language(source))); lang != "diff" && lang != "patch" {
		return nil
	}
	lines := strings.Split(rawCode(source, n), "\n")
	tones := make([]diffTone, len(lines))
	for i, line := range lines {
		tones[i] = toneOf(line)
	}
	return tones
}

func toneOf(line string) diffTone {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "@@"),
		strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
		return diffHeader
	case strings.HasPrefix(line, "+"):
		return diffAdded
	case strings.HasPrefix(line, "-"):
		return diffDeleted
	}
	return diffContext
}

func (d diffTone) color(th *ui.Theme, fg color.NRGBA) color.NRGBA {
	switch d {
	case diffAdded:
		return th.TonePass
	case diffDeleted:
		return th.Destructive
	case diffHeader:
		return th.MutedForeground
	}
	return fg
}

// diffCell is a diff block's code, each line in its tone.
func diffCell(th *ui.Theme, source []byte, n ast.Node, tones []diffTone, fg color.NRGBA) *markdownCell {
	cell := &markdownCell{}
	base := richtext.SpanStyle{Font: font.Font{Typeface: th.Mono}, Size: BodySize}
	for i, line := range strings.Split(rawCode(source, n), "\n") {
		span := base
		span.Color = tones[i].color(th, fg)
		if i < len(tones)-1 {
			line += "\n"
		}
		cell.add(span, expandTabs(line))
	}
	return cell
}
