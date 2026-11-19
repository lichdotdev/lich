package main

// The stage editor's blame: who last changed the cursor's line, how long ago
// and why, at the end of that line alone. The file's blame is read when the
// editor takes its text from the disk (open, reload, save) and kept; moving
// the cursor only reads it.

import (
	"context"
	"image"
	"strconv"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// blameGap is the room between the line's code and its blame.
	blameGap = unit.Dp(32)
	// blameMinRoom is the narrowest blame worth drawing; past the code's
	// end with less room, the line shows none.
	blameMinRoom = unit.Dp(80)
	// blameDateLayout is the tooltip's full date.
	blameDateLayout = "2006-01-02 15:04"
	daysPerMonth    = 30
	daysPerYear     = 365
)

// editorBlame is the cursor line's blame as the margin draws it.
type editorBlame struct {
	read remote[api.GitBlame]
	// gen counts the reads asked for: every time base is the disk's again.
	gen int
	// text is the line at the end of the cursor's line, "" for none; commit
	// and hash the commit it names, hash "" for an uncommitted line. cells is
	// how many cells the line's text takes.
	text   string
	hash   string
	commit api.GitBlameCommit
	cells  int
	tip    ui.Tooltip
}

// readBlame asks for the file's blame while it is the disk's text.
func (f *fileEditor) readBlame() {
	if f.kind != blobNone || f.loaded != f.opens || f.dir != "" {
		return
	}
	key := f.fileKey() + "\x00" + strconv.Itoa(f.blame.gen)
	client, checkout, rel := f.client, f.checkout, f.rel
	f.blame.read.want(key, f.fileKey(), func(ctx context.Context) (api.GitBlame, error) {
		return client.GitBlame(ctx, checkout, rel)
	})
}

// blameLine is what the blame says of doc's line (0-based): the commit's
// line, or that the line is the user's own while it differs from the disk or
// was never committed. kept maps doc's lines to base's, as diffLines does.
func blameLine(b api.GitBlame, kept []int, line int, now time.Time) (text, hash string) {
	if line < 0 || line >= len(kept) {
		return "", ""
	}
	if at := kept[line]; at >= 0 && at < len(b.Lines) {
		hash = b.Lines[at]
	}
	c, ok := b.Commits[hash]
	if hash == "" || !ok {
		return i18n.T("dock.editor.blameUncommitted"), ""
	}
	return i18n.T("dock.editor.blame", i18n.Args{"author": c.Author, "age": blameAge(c.Time, now), "summary": c.Summary}), hash
}

// blameAge is agoLabel, with months and years past a month: a commit's age
// runs to years, where weeks stop saying anything.
func blameAge(at int64, now time.Time) string {
	days := int(max(now.Sub(time.Unix(at, 0)), 0) / (24 * time.Hour))
	switch {
	case days >= daysPerYear:
		return i18n.T("session.age.ago.year", i18n.Args{"count": days / daysPerYear})
	case days >= daysPerMonth:
		return i18n.T("session.age.ago.month", i18n.Args{"count": days / daysPerMonth})
	}
	return agoLabel(at, now)
}

// blameRoom is how wide the blame may be drawn, given the code area's width
// (a line widget's widest): the editor draws it a cell past the line's text,
// TextX plus the line's cells, and the code area starts at GutterWidth.
func blameRoom(code, cells int, g godemirror.Geometry) int {
	return code + g.GutterWidth - g.TextX - (cells+1)*g.CellWidth
}

// blameWidget draws the cursor line's blame, muted and italic past a gap,
// cut with "…" where the code area ends, with the commit in its tooltip.
func (f *fileEditor) blameWidget(gtx C) D { return f.blameLabel(gtx, f.ed.Geometry()) }

func (f *fileEditor) blameLabel(gtx C, g godemirror.Geometry) D {
	th := f.th
	room := blameRoom(gtx.Constraints.Max.X, f.blame.cells, g)
	if room < gtx.Dp(blameGap+blameMinRoom) {
		return D{}
	}
	gtx.Constraints = layout.Constraints{Max: image.Pt(room, gtx.Constraints.Max.Y)}
	label := func(gtx C) D {
		return layout.Inset{Left: blameGap}.Layout(gtx, func(gtx C) D { return oblique(gtx, th.Text(ui.TextXS, f.blame.text).Muted().Layout) })
	}
	if f.blame.hash == "" {
		return label(gtx)
	}
	return th.TooltipCard(f.env.root, &f.blame.tip, f.blameTooltip).Layout(gtx, label)
}

// blameTooltip is the commit: its short hash and full date, then its whole
// message.
func (f *fileEditor) blameTooltip(gtx C) D {
	th := f.th.On(f.th.Popover)
	c, hash := f.blame.commit, f.blame.hash
	head := hash[:min(shortHashLen, len(hash))] + " · " + time.Unix(c.Time, 0).Format(blameDateLayout)
	return ui.Stack(1.5,
		th.Text(ui.TextXS, head).Mono().Muted().Layout,
		th.Text(ui.TextXS, c.Author).Weight(font.Medium).Layout,
		para(th.Text(ui.TextXS, c.Message).Wrapped()).Layout,
	)(gtx)
}

// obliqueShear is how far oblique leans its text, in radians: about the 12°
// of a sans face's own italic.
const obliqueShear = -0.2

// oblique draws w leaning right from its foot: Onest, the interface's face,
// has no italic for Italic to pick.
func oblique(gtx C, w layout.Widget) D {
	macro := op.Record(gtx.Ops)
	d := w(gtx)
	call := macro.Stop()
	foot := f32.Pt(0, float32(d.Size.Y))
	defer op.Affine(f32.Affine2D{}.Shear(foot, obliqueShear, 0)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return d
}
