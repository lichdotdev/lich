package main

import (
	"fmt"
	"image"
	"log"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/glyphs"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/diffview"
)

// chatDiffShown is the rows an open edit shows before its "Show N more lines",
// so one long write does not push the conversation out of the viewport.
const chatDiffShown = 20

// chatEditDiff is an open edit's diff drawn as the Review panel draws one: a
// view per file the call changed, built again when the diff or the code font
// changes.
type chatEditDiff struct {
	key   string
	files []chatDiffFile
	fail  bool
	more  ui.Clickable
	whole bool
}

type chatDiffFile struct {
	rows   int
	patch  godemirror.FilePatch
	view   *diffview.View
	blocks diffview.Blocks
}

// chatDiffCells measures the code font's rows, which a diff view is as tall
// as; family names the font it was built for.
type chatDiffCells struct {
	family string
	cells  *glyphs.Rasterizer
}

func (c *chatDiffCells) height(gtx C, family string, size unit.Sp) (int, error) {
	if c.cells == nil || c.family != family {
		cells, err := glyphs.New(family)
		if err != nil {
			return 0, err
		}
		c.cells, c.family = cells, family
	}
	if _, err := c.cells.SetSize(float64(gtx.Sp(size))); err != nil {
		return 0, err
	}
	return c.cells.CellH(), nil
}

func (d *chatEditDiff) build(diff, family string, size unit.Sp) {
	key := fmt.Sprintf("%s\x00%v\x00%s", family, size, diff)
	if d.key == key {
		return
	}
	d.key, d.files, d.fail = key, nil, false
	files, err := parsePatchFiles(gitPatch(diff))
	if err != nil {
		d.fail = true
		log.Printf("chat diff: %v", err)
		return
	}
	for _, f := range files {
		patch, err := f.filePatch()
		if err != nil {
			d.fail = true
			log.Printf("chat diff %s: %v", f.path, err)
			return
		}
		view, err := diffview.New(family, size, diffview.Options{Buttons: diffview.NewButtons(reviewDiffLabels)})
		if err != nil {
			d.fail = true
			log.Printf("chat diff %s: %v", f.path, err)
			return
		}
		d.files = append(d.files, chatDiffFile{rows: diffRows(f, false, nil), patch: patch, view: view})
	}
}

// layout draws the files' diffs one under the other, chatDiffShown rows of
// them until the way to the rest under them is clicked.
func (d *chatEditDiff) layout(gtx C, th *ui.Theme, code godemirror.Theme, cell int) D {
	if d.more.Clicked(gtx) {
		d.whole = true
	}
	budget, total := chatDiffShown, 0
	children := make([]layout.FlexChild, 0, len(d.files)+1)
	for i := range d.files {
		f := &d.files[i]
		total += f.rows
		rows := f.rows
		if !d.whole {
			rows = min(rows, budget)
			budget -= rows
		}
		if rows == 0 {
			continue
		}
		children = append(children, layout.Rigid(func(gtx C) D { return f.layout(gtx, th, code, rows*cell) }))
	}
	if hidden := total - chatDiffShown; !d.whole && hidden > 0 {
		children = append(children, layout.Rigid(func(gtx C) D { return chatDiffMore(gtx, th, &d.more, hidden) }))
	}
	// The code view is drawn on th.Surface; the room around it takes the same
	// color, rounded where it meets the box's bottom corners.
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			r := gtx.Dp(ui.RadiusMD)
			paint.FillShape(gtx.Ops, th.Surface, clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Min}, SE: r, SW: r}.Op(gtx.Ops))
			return D{Size: gtx.Constraints.Min}
		},
		func(gtx C) D {
			return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
}

func (f *chatDiffFile) layout(gtx C, th *ui.Theme, code godemirror.Theme, height int) D {
	defer clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Push(gtx.Ops).Pop()
	dims, _, err := f.view.Layout(gtx, diffview.Frame{Theme: th, Code: code, Patch: f.patch, Height: height, Blocks: &f.blocks})
	if err != nil {
		log.Printf("chat diff %s: %v", f.patch.Path, err)
	}
	return D{Size: image.Pt(dims.Size.X, height)}
}

func chatDiffMore(gtx C, th *ui.Theme, c *ui.Clickable, hidden int) D {
	return c.Layout(gtx, func(gtx C) D {
		fg := th.MutedForeground
		if c.Hovered() {
			fg = th.Foreground
		}
		return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1.5)}.Layout(gtx,
			th.Text(ui.TextXS, i18n.T("chat.view.moreLines", i18n.Args{"count": hidden})).In(fg).Layout)
	})
}

// gitPatch is diff with git's "diff --git" line over each file, which the
// Review panel's reader splits files by: a provider's diff has none, the
// one git printed for a shell command's files has them already.
func gitPatch(diff string) string {
	if strings.HasPrefix(diff, "diff --git ") {
		return diff
	}
	lines := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")
	var b strings.Builder
	oldLeft, newLeft := 0, 0
	for i, line := range lines {
		switch {
		case oldLeft > 0 || newLeft > 0:
			oldLeft, newLeft = hunkLeft(line, oldLeft, newLeft)
		case strings.HasPrefix(line, "@@"):
			if h, err := parseHunkHead(line); err == nil {
				oldLeft, newLeft = h.oldCount, h.newCount
			}
		case strings.HasPrefix(line, "--- ") && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "+++ "):
			p := pickDiffSide(strings.TrimSpace(line[4:]), strings.TrimSpace(lines[i+1][4:]))
			b.WriteString("diff --git a/" + p + " b/" + p + "\n")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// hunkLeft is the lines a hunk still has on each side once line is read.
func hunkLeft(line string, oldLeft, newLeft int) (int, int) {
	switch {
	case strings.HasPrefix(line, "+"):
		return oldLeft, newLeft - 1
	case strings.HasPrefix(line, "-"):
		return oldLeft - 1, newLeft
	case strings.HasPrefix(line, `\`):
		return oldLeft, newLeft
	}
	return oldLeft - 1, newLeft - 1
}
