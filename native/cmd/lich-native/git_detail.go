package main

import (
	"image"
	"log"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icGitBack = icons.Lucide("arrow-left")

// gitDetailPane is the selected commit's detail, and the one file of it whose
// diff is standing in for the log.
type gitDetailPane struct {
	list    widget.List
	message widget.List
	files   map[string]*widget.Clickable

	open     *gitDiffKey
	back     widget.Clickable
	split    bool
	splitBtn ui.IconActionState
	view     *godemirror.DiffView
	viewFail bool
	// patchFor is the answer patch was parsed from.
	patchFor string
	patch    godemirror.FilePatch
	binary   bool
	empty    bool
}

func (d *gitDetailPane) init() {
	d.list.Axis, d.message.Axis = layout.Vertical, layout.Vertical
	d.files = map[string]*widget.Clickable{}
}

func (d *gitDetailPane) diffOpen() bool { return d.open != nil }
func (d *gitDetailPane) closeFile()     { d.open = nil }

func (g *gitPanel) detailColumn(gtx C, th *ui.Theme, in gitPanelInput) D {
	if g.selected == "" {
		return gitNotice(gtx, th, i18n.T("git.panel.selectCommit"))
	}
	read := g.store.commit(in.path, g.selected)
	switch {
	case !read.loaded && !read.prev:
		return layout.UniformInset(ui.Space(3)).Layout(gtx, th.SkeletonText(ui.TextSM, unit.Dp(180)).Layout)
	case read.err != "":
		return gitNotice(gtx, th, i18n.T("git.panel.commitFailed", i18n.Args{"error": read.err}))
	}
	return g.detailSplit(gtx, th, read.val)
}

// detailSplit is the commit's files over its message, as IntelliJ stacks them,
// each scrolling alone, the seam between them dragged.
func (g *gitPanel) detailSplit(gtx C, th *ui.Theme, c backend.GitCommitDetail) D {
	size := gtx.Constraints.Max
	spare := gtx.Metric.PxToDp(size.Y) - unit.Dp(gitFilesFloorRem*remDp)
	messageHeight, released := g.messageCol.width(gtx, spare)
	if released {
		g.saveColumn(g.messageCol)
	}
	seam := gtx.Dp(1)
	filesHeight := max(size.Y-messageHeight-seam, 0)
	pane := func(height int, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(size.X, height))
			return w(gtx)
		})
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		pane(filesHeight, func(gtx C) D { return g.detailFiles(gtx, th, c) }),
		layout.Rigid(th.Separator),
		pane(messageHeight, func(gtx C) D {
			list := gitScroll(th, &g.detail.message)
			return list.Layout(gtx, 1, func(gtx C, _ int) D { return gitCommitHead(gtx, th, c) })
		}),
	)
	defer op.Offset(image.Pt(0, filesHeight+seam)).Push(gtx.Ops).Pop()
	handle := gtx
	handle.Constraints = layout.Exact(image.Pt(size.X, messageHeight))
	g.messageCol.resize.Layout(handle, th)
	return D{Size: size}
}

// gitFilesTotal sums what a commit changed, the figure over its file list.
func gitFilesTotal(files []backend.GitCommitFile) (added, deleted int) {
	for _, f := range files {
		added += f.Added
		deleted += f.Deleted
	}
	return added, deleted
}

func (g *gitPanel) detailFiles(gtx C, th *ui.Theme, c backend.GitCommitDetail) D {
	added, deleted := gitFilesTotal(c.Files)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(7))))
			return layout.Inset{Left: ui.Space(3.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						// Two sizes on one line: under the row's Exact height they would not share a baseline.
						gtx.Constraints.Min = image.Point{}
						return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
							layout.Rigid(th.Text(gitGroupLabelSize, strings.ToUpper(i18n.T("git.panel.files"))).Muted().Layout),
							ui.Gap(1.5),
							layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(len(c.Files))).Mono().Muted().Layout),
						)
					}),
					layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min = image.Point{}
						return th.DiffStat(added, deleted).Layout(gtx)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			list := gitScroll(th, &g.detail.list)
			return list.Layout(gtx, len(c.Files), func(gtx C, i int) D { return g.detailFile(gtx, th, c.Hash, c.Files[i]) })
		}),
	)
}

// gitCommitHead is the commit's subject, who and when, every ref it carries
// and the rest of its message.
func gitCommitHead(gtx C, th *ui.Theme, c backend.GitCommitDetail) D {
	subject, body, _ := strings.Cut(c.Message, "\n")
	multi := func(l ui.LabelStyle) layout.Widget {
		l.MaxLines = 0
		return l.Layout
	}
	short := c.Hash[:min(len(c.Hash), shortHashLen)]
	when := time.Unix(c.When, 0).In(gtx.Now.Location()).Format("Jan 2, 2006 15:04")
	kids := []layout.FlexChild{
		layout.Rigid(multi(th.Text(ui.TextSM, gitCleanLine(subject)).Weight(font.Medium))),
		ui.VGap(1.5),
		layout.Rigid(multi(th.Text(ui.TextXS, short+"  "+gitCleanLine(c.Author)).Mono().Muted())),
		layout.Rigid(multi(th.Text(ui.TextXS, when).Mono().Muted())),
	}
	if len(c.Refs) > 0 {
		names := make([]string, len(c.Refs))
		for i, r := range c.Refs {
			names[i] = r.Name
		}
		kids = append(kids, ui.VGap(1.5), layout.Rigid(multi(th.Text(ui.TextXS, strings.Join(names, "  ")).Mono().Muted())))
	}
	if body = strings.TrimSpace(body); body != "" {
		kids = append(kids, ui.VGap(2), layout.Rigid(multi(th.Text(ui.TextXS, strings.ReplaceAll(body, "\t", "    ")).Muted())))
	}
	return layout.Inset{Left: ui.Space(3.5), Right: ui.Space(2), Top: ui.Space(2.5), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

// detailFile is one changed file; choosing it opens its diff in the log's
// place.
func (g *gitPanel) detailFile(gtx C, th *ui.Theme, hash string, f backend.GitCommitFile) D {
	click, ok := g.detail.files[f.Path]
	if !ok {
		click = new(widget.Clickable)
		g.detail.files[f.Path] = click
	}
	for click.Clicked(gtx) {
		g.detail.open = &gitDiffKey{hash: hash, file: f.Path, oldFile: f.OldPath}
	}
	return layout.Inset{Left: ui.Space(1), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return click.Layout(gtx, func(gtx C) D {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitRowHeight))
			gtx.Constraints = layout.Exact(size)
			open := g.detail.open != nil && g.detail.open.hash == hash && g.detail.open.file == f.Path
			rowFill(gtx, th, open, click.Hovered())
			return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// From its end: the file's name is what tells two rows apart.
					layout.Flexed(1, ui.Spanning(func(gtx C) D { return th.Text(ui.TextXS, gitCleanLine(f.Path)).Mono().LayoutTail(gtx) })),
					ui.Gap(1.5),
					layout.Rigid(func(gtx C) D {
						if f.Binary {
							return D{}
						}
						// A baseline row under the row's Exact height would sit at its top.
						gtx.Constraints.Min = image.Point{}
						return th.DiffStat(f.Added, f.Deleted).Layout(gtx)
					}),
				)
			})
		})
	})
}

// layoutDiff stands in for the log: a strip with the way back over one file's
// patch.
func (d *gitDetailPane) layoutDiff(gtx C, th *ui.Theme, g *gitPanel, in gitPanelInput) D {
	for d.back.Clicked(gtx) {
		d.closeFile()
		return D{Size: gtx.Constraints.Max}
	}
	for d.splitBtn.Click.Clicked(gtx) {
		d.split = !d.split
	}
	splitLabel := i18n.T("diff.dockReviewBody.split")
	if d.split {
		splitLabel = i18n.T("diff.dockReviewBody.unified")
	}
	key := *d.open
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitFilterHeight)))
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
				b := th.Button(&d.back, i18n.T("git.panel.back"))
				b.Variant, b.Size, b.Icon = ui.VariantGhost, ui.SizeSM, icGitBack
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(b.Layout),
					ui.Gap(2),
					layout.Flexed(1, func(gtx C) D {
						return th.Text(ui.TextXS, gitCleanLine(key.file)).Mono().Muted().Truncated().Layout(gtx)
					}),
					ui.Gap(2),
					layout.Rigid(func(gtx C) D { return th.IconAction(g.root, &d.splitBtn, icSplit, splitLabel, d.split).Layout(gtx) }),
				)
			})
		}),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return d.diffBody(gtx, th, g, in, key)
		}),
	)
}

func (d *gitDetailPane) diffBody(gtx C, th *ui.Theme, g *gitPanel, in gitPanelInput, key gitDiffKey) D {
	read := g.store.fileDiff(in.path, key)
	switch {
	case !read.loaded && !read.prev:
		return gitLogSkeleton(gtx, th)
	case read.err != "":
		return gitNotice(gtx, th, i18n.T("git.panel.diffFailed", i18n.Args{"error": read.err}))
	}
	if read.loaded {
		d.takeDiff(read)
	}
	switch {
	case d.binary:
		return gitNotice(gtx, th, i18n.T("git.panel.binary"))
	case d.empty:
		return gitNotice(gtx, th, i18n.T("git.panel.noDiff"))
	}
	if d.view == nil && !d.viewFail {
		view, err := codeview.NewDiffEditor(g.family, g.size, godemirror.DiffOptions{HighlightChanges: true, StyleTags: codeview.StyleTags})
		if err != nil {
			d.viewFail = true
			log.Printf("git diff view: %v", err)
		}
		d.view = view
	}
	if d.view == nil {
		return gitNotice(gtx, th, i18n.T("git.panel.noDiff"))
	}
	t := in.code
	t.Background, t.GutterBackground = th.Surface, th.Surface
	d.view.SetTheme(t)
	d.view.Split = d.split && gtx.Constraints.Max.X >= gtx.Dp(splitMinWidth)
	d.view.SetPatch(d.patch)
	dims, err := d.view.Layout(gtx)
	if err != nil {
		log.Printf("git diff: %v", err)
	}
	return dims
}

// takeDiff parses an answer once.
func (d *gitDetailPane) takeDiff(read gitLoaded[string]) {
	if d.patchFor == read.key {
		return
	}
	d.patchFor, d.binary, d.empty = read.key, false, false
	files, err := parsePatchFiles(read.val)
	if err != nil {
		log.Printf("git diff: %v", err)
	}
	if len(files) == 0 {
		d.empty = true
		return
	}
	// A rename's old side prints no header of its own, so the first file is
	// the one asked for.
	d.binary = files[0].binary
	if d.patch, err = files[0].filePatch(); err != nil {
		log.Printf("git diff: %v", err)
	}
	d.empty = !d.binary && len(files[0].hunks) == 0
}
