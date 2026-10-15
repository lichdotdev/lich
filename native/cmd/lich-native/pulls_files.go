package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (pulls.pullsFiles.*).
const (
	pullsFilesViewed = "%d of %d viewed"
	pullsViewedLabel = "Viewed"
	pullsHideTree    = "Hide the file tree"
	pullsShowTree    = "Show the file tree"
)

// The file tree's width in rem, PullsFiles' TREE_DEFAULT_REM and the bounds
// its handle drags between, kept under treeWidthKey.
const (
	treeDefaultRem = 16
	treeMinRem     = 12
	treeMaxRem     = 40
	treeWidthKey   = "lich.pulls.tree.width"
)

// parseTreeWidth is the stored width, the default for garbage or nothing.
func parseTreeWidth(raw string) float64 {
	rem, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsInf(rem, 0) || math.IsNaN(rem) || rem <= 0 {
		return treeDefaultRem
	}
	return min(treeMaxRem, max(treeMinRem, rem))
}

func loadTreeWidth(ctx context.Context, c *lichclient.Client) (float64, error) {
	raw, err := c.GetSetting(ctx, treeWidthKey, "")
	return parseTreeWidth(raw), err
}

// filesTree is the Files tab's file tree: the PR's paths nested, folders open
// until folded, a file's click scrolling its card into view.
type filesTree struct {
	rows   treeRows
	closed map[string]bool
	hidden bool
	toggle iconActionState
	// widthRem is the tree's width, dragged by resize.
	widthRem float64
	resize   ui.ResizeHandle
	laidOut  int // px, the width the tree was last laid out at
	// builtFor is the file order the tree was built from.
	builtFor string
	active   string
}

// filesInput is what the Files tab needs from the window this frame: the
// active session, whose terminal takes injected references, and its
// checkout, whose comment batch the tab shares with the dock.
type filesInput struct {
	sessionID, checkout string
	light               bool
	code                godemirror.Theme
}

// viewedTick is the card's Viewed checkbox; ticking folds the file away.
func (c *fileCard) viewedTick(gtx C, th *ui.Theme, p *reviewPanel) D {
	f := p.pr
	tick, ok := f.ticks[c.git.path]
	if !ok {
		tick = new(widget.Bool)
		f.ticks[c.git.path] = tick
	}
	tick.Value = f.isViewed(c)
	if tick.Update(gtx) {
		if tick.Value {
			p.markViewed(c)
		} else {
			delete(f.viewed, c.git.path)
		}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Checkbox(tick).Layout),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, pullsViewedLabel).Muted().Layout),
	)
}

// filesTab is PullsFiles: the toolbar over the PR's diff, one card per file
// with the comment batch at the foot, the dock's Review cards read-only.
func (v *pullView) filesTab(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) D {
	files := v.files
	if files == nil {
		return D{}
	}
	viewed, ok := v.viewed[d.URL]
	if !ok {
		viewed = map[string]string{}
		v.viewed[d.URL] = viewed
	}
	files.pr.viewed = viewed
	review := v.pendingOf(d.URL)
	files.pr.toReview = review.add
	v.inline = inlineFrame{th: th, p: p, d: d, act: act, now: gtx.Now}
	files.pr.blocks = v.diffBlocks
	if len(d.Commits) > 0 {
		files.pr.head = d.Commits[len(d.Commits)-1].OID
	}
	in := p.files
	files.th, files.light, files.code, files.target = th, in.light, in.code, in.checkout
	files.inject = dockInjector(files.client, in.sessionID)
	read := p.store.diff(p.project.Path, d.Number)
	switch {
	case !read.loaded:
		return panelNotice(gtx, th, pullsDiffLoading)
	case read.err != "":
		return panelNotice(gtx, th, fmt.Sprintf(pullsDiffFailed, read.err))
	}
	files.takeRead(d.URL, &diffRead{text: read.val})
	files.takeAnswers()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return v.filesToolbar(gtx, th) }),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D { return v.filesBody(gtx, th) }),
	)
}

// filesBody is the tree, when shown, beside the cards.
func (v *pullView) filesBody(gtx C, th *ui.Theme) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	dims := layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if v.tree.hidden {
				return D{}
			}
			return v.resizableTree(gtx, th)
		}),
		layout.Rigid(func(gtx C) D {
			if v.tree.hidden {
				return D{}
			}
			return verticalRule(gtx, th)
		}),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if err := v.files.bodyWithBatch(gtx, th); err != nil {
				return notice(gtx, th, err.Error())
			}
			return D{Size: gtx.Constraints.Max}
		}),
	)
	if !v.tree.hidden {
		v.treeHandle(gtx, th)
	}
	return dims
}

// treeHandle lays the tree's resize handle out past the rule, over the
// cards' margin: on the tree's own edge its lit strip would cover the
// tree's scrollbar. Its box starts at the tree's left edge, which a drag
// leaves in place, so the handle reads the drag as a right-edge panel's.
func (v *pullView) treeHandle(gtx C, th *ui.Theme) {
	gtx.Constraints = layout.Exact(image.Pt(v.tree.laidOut+gtx.Dp(1)+gtx.Dp(ui.ResizeWidth), gtx.Constraints.Max.Y))
	v.tree.resize.Layout(gtx, th)
}

// resizableTree is the tree at its width; a drag of the handle (treeHandle)
// that ends stores the width.
func (v *pullView) resizableTree(gtx C, th *ui.Theme) D {
	t := &v.tree
	if t.widthRem == 0 {
		t.widthRem = treeDefaultRem
	}
	w, released := t.resize.Update(gtx, unit.Dp(t.widthRem*remDp), treeMinRem*remDp, treeMaxRem*remDp)
	t.widthRem = float64(w) / remDp
	if released {
		rem, store := t.widthRem, v.files.client
		go func() {
			if err := store.SetSetting(context.Background(), treeWidthKey, "", strconv.FormatFloat(rem, 'f', -1, 64)); err != nil {
				log.Printf("tree width: %v", err)
			}
		}()
	}
	t.laidOut = gtx.Dp(w)
	gtx.Constraints = layout.Exact(image.Pt(t.laidOut, gtx.Constraints.Max.Y))
	return v.treeLayout(gtx, th)
}

// treeLayout is the tree over the PR's files; a click on a file scrolls the
// cards to it.
func (v *pullView) treeLayout(gtx C, th *ui.Theme) D {
	t, files := &v.tree, v.files
	if key := strings.Join(files.order, "\x00"); key != t.builtFor {
		t.builtFor = key
		t.rows.tree = buildTree(files.order)
		stats := map[string]lineDelta{}
		for path, c := range files.cards {
			stats[path] = lineDelta{c.patch.Added, c.patch.Deleted}
		}
		t.rows.stats = withDirStats(t.rows.tree, stats)
	}
	rows := visibleRows(t.rows.tree, func(path string) bool { return !t.closed[path] })
	for _, r := range rows {
		if !t.rows.click(r.node.path).Clicked(gtx) {
			continue
		}
		if r.node.dir {
			t.closed[r.node.path] = !t.closed[r.node.path]
			continue
		}
		t.active = r.node.path
		files.scrollToFile(r.node.path)
	}
	return t.rows.layout(gtx, th, rows, t.active)
}

// verticalRule is a 1px border-r seam the height of gtx.
func verticalRule(gtx C, th *ui.Theme) D {
	size := image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	return D{Size: size}
}

// filesToolbar is "N of M viewed", the diff's size, and the dock's split and
// collapse-all toggles.
func (v *pullView) filesToolbar(gtx C, th *ui.Theme) D {
	files := v.files
	viewed, added, deleted := 0, 0, 0
	for _, path := range files.order {
		c := files.cards[path]
		added, deleted = added+c.patch.Added, deleted+c.patch.Deleted
		if files.pr.isViewed(c) {
			viewed++
		}
	}
	if v.tree.toggle.click.Clicked(gtx) {
		v.tree.hidden = !v.tree.hidden
	}
	treeIcon, treeLabel := icPullHide, pullsHideTree
	if v.tree.hidden {
		treeIcon, treeLabel = icPullShow, pullsShowTree
	}
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return iconAction(gtx, th, v.root, &v.tree.toggle, treeIcon, treeLabel, false) }),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf(pullsFilesViewed, viewed, len(files.order))).Muted().Layout),
			ui.Gap(3),
			layout.Rigid(func(gtx C) D { return diffStat(gtx, th, added, deleted) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return files.layoutActions(gtx, th) }),
		)
	})
}

// inlineFrame is what the threads hung in the diff are drawn with this
// frame: the view lays its blocks out inside the Files tab's own layout.
type inlineFrame struct {
	th  *ui.Theme
	p   *pullsScreen
	d   lichclient.PRDetail
	act *pullsAction
	now time.Time
}

// diffBlocks are path's review threads still anchored to a line (an outdated
// one lives in the Conversation tab alone) and its pending comments, each
// under its line.
func (v *pullView) diffBlocks(path string) []cardBlock {
	f := v.inline
	var out []cardBlock
	if conv := f.p.store.conversation(f.p.project.Path, f.d.Number).val; conv != nil {
		for _, t := range conv.Threads {
			if t.Path != path || t.IsOutdated || t.Line == 0 {
				continue
			}
			id := t.ID
			out = append(out, cardBlock{key: "thread:" + id, at: lineRef(t.Side, t.Line), widget: func(gtx C) D { return v.inlineThread(gtx, id) }})
		}
	}
	for i, c := range v.pendingOf(f.d.URL).snapshot() {
		if c.Path != path {
			continue
		}
		key := fmt.Sprint("pending:", i, ":", c.Body)
		comment := c
		out = append(out, cardBlock{key: key, at: lineRef(c.Side, c.Line), widget: func(gtx C) D { return v.inlinePending(gtx, comment) }})
	}
	return out
}

// lineRef is GitHub's side of a line as the diff names it.
func lineRef(side string, line int) godemirror.LineRef {
	if side == "LEFT" {
		return godemirror.LineRef{Side: godemirror.OldSide, Line: line}
	}
	return godemirror.LineRef{Side: godemirror.NewSide, Line: line}
}

// inlineThread is the thread's card under its line, read fresh each frame so
// a reply or a resolve shows once the conversation is read again.
func (v *pullView) inlineThread(gtx C, id string) D {
	f := v.inline
	conv := f.p.store.conversation(f.p.project.Path, f.d.Number).val
	if conv == nil {
		return D{}
	}
	for _, t := range conv.Threads {
		if t.ID != id {
			continue
		}
		// The diff shows the lines GitHub's hunk would repeat.
		t.Comments = slices.Clone(t.Comments)
		if len(t.Comments) > 0 {
			t.Comments[0].DiffHunk = ""
		}
		return composerSlot.Layout(gtx, func(gtx C) D { return v.threadBox(gtx, f.th, f.p, f.d, t, f.act, f.now) })
	}
	return D{}
}

// inlinePending is a comment waiting for the review to be submitted.
func (v *pullView) inlinePending(gtx C, c lichclient.ReviewComment) D {
	th := v.inline.th
	return composerSlot.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
			func(gtx C) D {
				on := th.On(th.Sidebar)
				return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(on.Text(ui.TextXS, talkPendingNote).In(on.ToneWait).Layout),
						ui.VGap(1),
						layout.Rigid(func(gtx C) D { return wrapped(gtx, on.Text(ui.TextSM, c.Body)) }),
					)
				})
			})
	})
}
