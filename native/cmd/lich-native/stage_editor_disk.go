package main

// The stage editor against an agent writing the same file: with nothing
// unsaved it takes the disk's text on the line the cursor was on; with
// unsaved changes it warns over the code, with the diff of what the disk
// changed since the editor read it, and waits for Reload or Keep mine.

import (
	"context"
	"fmt"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// editorDiskTick is how often the editor on the stage reads its file again.
// A session that writes files says so (session-touched), which reads it at
// once; the tick only catches a write from outside lich.
const editorDiskTick = 5 * time.Second

// diskBannerDiff is the most room the warning's diff takes before it
// scrolls, so the code stays in sight under it.
const diskBannerDiff = unit.Dp(240)

// diskWatch is what the editor knows of the disk: the reads, seen the last
// text it weighed, and theirs the disk's text the user has to decide on.
type diskWatch struct {
	read  remote[string]
	nudge int
	seen  string
	// theirs is the disk's text since the editor read the file, while the
	// user holds changes of their own; nil while there is none.
	theirs       *string
	reload, keep widget.Clickable
}

// recheck reads the file again on the next frame: a session wrote, the
// window came back, or the editor came back to the stage.
func (f *fileEditor) recheck() {
	f.disk.nudge++
	f.invalidate()
}

// watchDisk reads the file on editorDiskTick and on every recheck, and
// weighs each new text against what the editor read and holds.
func (f *fileEditor) watchDisk(gtx C) {
	if f.kind != blobNone || f.ed == nil || f.loaded != f.opens {
		return
	}
	tick := gtx.Now.Truncate(editorDiskTick)
	gtx.Execute(op.InvalidateCmd{At: tick.Add(editorDiskTick)})
	key := fmt.Sprintf("%s\x00%d\x00%d", f.fileKey(), tick.Unix(), f.disk.nudge)
	client, checkout, rel := f.client, f.checkout, f.rel
	f.disk.read.want(key, f.fileKey(), func(ctx context.Context) (string, error) { return client.ReadFile(ctx, checkout, rel) })
	if f.disk.reload.Clicked(gtx) && f.disk.theirs != nil {
		f.reloadFrom(*f.disk.theirs)
	}
	if f.disk.keep.Clicked(gtx) && f.disk.theirs != nil {
		f.base, f.disk.theirs = *f.disk.theirs, nil
		f.gen++
		f.setDirty(f.ed.State().Doc() != f.base)
	}
	text, loading, err := f.disk.read.get()
	if loading || err != nil || text == f.disk.seen {
		return
	}
	f.disk.seen = text
	f.weighDisk(text)
}

// weighDisk takes text, the disk's, as diskVerdictOf rules.
func (f *fileEditor) weighDisk(text string) {
	disk := normalizedText(text)
	doc := f.ed.State().Doc()
	switch diskVerdictOf(f.base, doc, disk) {
	case diskSame:
		f.disk.theirs = nil
		if disk == doc && disk != f.base {
			f.base = disk
			f.gen++
			f.setDirty(false)
		}
	case diskReload:
		f.reloadFrom(disk)
	case diskAsk:
		f.disk.theirs = &disk
		f.diff.SetDocs(f.rel, f.base, disk)
	}
}

// reloadFrom takes text from the disk over what the editor holds, keeping
// the cursor's line and the scroll.
func (f *fileEditor) reloadFrom(text string) {
	st := f.ed.State()
	line, top := st.LineAt(st.Selection().Main().Head()).Number, f.ed.ScrollTop()
	f.disk.theirs = nil
	if err := f.setText(text); err != nil {
		return
	}
	st = f.ed.State()
	at := st.Line(min(line, st.Lines())).From
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(at, at), UserEvent: "select"})
	f.ed.SetScrollTop(top)
}

// diskBanner is the warning over the code while the disk changed under the
// user's changes: what happened, the two ways out, and the disk's diff.
func (f *fileEditor) diskBanner(gtx C, th *ui.Theme) D {
	if f.disk.theirs == nil || f.ed == nil {
		return D{}
	}
	tint := th.Over(ui.Alpha(th.ToneWait, editorBannerTint))
	on := th.On(tint)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, tint, 0) },
				func(gtx C) D { return f.bannerRow(gtx, on) })
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y/2, gtx.Dp(diskBannerDiff))
			gtx.Constraints.Min = gtx.Constraints.Max
			f.diff.SetTheme(f.env.code)
			d, _ := f.diff.Layout(gtx)
			return d
		}),
		layout.Rigid(th.Separator),
	)
}

// editorBannerTint is how much of the wait tone the warning's strip takes.
const editorBannerTint = 0.14

func (f *fileEditor) bannerRow(gtx C, th *ui.Theme) D {
	keep := th.Button(&f.disk.keep, i18n.T("dock.editor.keepMine"))
	keep.Size, keep.Variant = ui.SizeSM, ui.VariantOutline
	reload := th.Button(&f.disk.reload, i18n.T("dock.editor.reload"))
	reload.Size = ui.SizeSM
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, para(th.Text(ui.TextSM, i18n.T("dock.editor.changedOnDisk"))).Layout),
			ui.Gap(2),
			layout.Rigid(keep.Layout),
			ui.Gap(1.5),
			layout.Rigid(reload.Layout),
		)
	})
}
