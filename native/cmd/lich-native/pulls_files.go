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
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/diffview"
	"github.com/lichdotdev/lich/native/ui/filetree"
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

func loadTreeWidth(ctx context.Context, c *backend.Client) (float64, error) {
	raw, err := c.GetSetting(ctx, treeWidthKey, "")
	return parseTreeWidth(raw), err
}

// filesTree is the Files tab's file tree: the PR's paths nested, folders open
// until folded, a file's click scrolling its card into view.
type filesTree struct {
	rows   filetree.Rows
	closed map[string]bool
	hidden bool
	toggle ui.IconActionState
	// widthRem is the tree's width, dragged by resize.
	widthRem float64
	resize   ui.ResizeHandle
	laidOut  int // px, the width the tree was last laid out at
	// builtFor is the file order the tree was built from.
	builtFor string
	active   string
}

// filesInput is what the Files tab needs from the window this frame: the
// active session, and how to inject into and submit to a session, bound to
// the PR's own (reviewSession).
type filesInput struct {
	activeID  string
	injector  func(id string) func(string) bool
	submitter func(id string) func(api.RequestOptions, func(error)) bool
	light     bool
	code      godemirror.Theme
}

// viewedTick is the card's Viewed checkbox; ticking folds the file away.
func (c *fileCard) viewedTick(gtx C, th *ui.Theme, p *reviewPanel) D {
	f := p.pr
	tick, ok := f.ticks[c.git.path]
	if !ok {
		tick = new(ui.Bool)
		f.ticks[c.git.path] = tick
	}
	tick.Value = f.isViewed(c)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Checkbox(tick).Layout),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("diff.fileDiff.viewed")).Muted().Layout),
	)
}

// takeTicks carries out the cards' Viewed ticks before the toolbar counts
// them and the cards are drawn.
func (p *reviewPanel) takeTicks(gtx C) {
	f := p.pr
	for _, path := range p.order {
		c, tick := p.cards[path], f.ticks[path]
		if tick == nil {
			continue
		}
		tick.Value = f.isViewed(c)
		if !tick.Update(gtx) {
			continue
		}
		if tick.Value {
			p.markViewed(c)
			p.saveViewed(c)
		} else {
			delete(f.viewed.files, c.git.path)
			p.saveUnviewed(c)
		}
	}
}

// filesTab is PullsFiles: the toolbar over the PR's diff, one card per file
// with the comment batch at the foot, the dock's Review cards read-only.
func (v *pullView) filesTab(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	files := v.files
	if files == nil {
		return D{}
	}
	viewed, ok := v.viewed[d.URL]
	if !ok {
		viewed = &viewedTicks{}
		v.viewed[d.URL] = viewed
		v.loadViewed(d.URL, viewed)
	}
	files.pr.viewed, files.pr.url, files.pr.number, files.pr.forge = viewed, d.URL, d.Number, v.forge
	review := v.pendingOf(d.URL)
	files.pr.toReview, files.pr.own = review.add, v.ownPR(p, d)
	if v.forge == api.ForgeGitLab {
		files.pr.toReview = nil
	}
	v.talkClicks(gtx, review)
	v.inline = inlineFrame{th: th, p: p, d: d, act: act, now: gtx.Now}
	files.pr.blocks = v.diffBlocks
	if len(d.Commits) > 0 {
		files.pr.head = d.Commits[len(d.Commits)-1].OID
	}
	read, key := v.filesRead(p, d)
	if files.pr.base != "" {
		files.pr.toReview, files.pr.blocks = nil, nil
	}
	in := p.files
	session, target := p.reviewSession(d)
	if target != d.URL {
		files.comments.move(d.URL, target)
	}
	files.th, files.light, files.code, files.target, files.checkout = th, in.light, in.code, target, p.prCheckout(d)
	files.inject, files.submit = in.injector(session), in.submitter(session)
	switch {
	// The cards fold by their ticks when first read, so they wait for them.
	case !read.loaded, !viewed.loaded:
		return v.skeletonFiles(gtx, th)
	case read.err != "":
		notice := th.PanelNotice(i18n.T("pulls.pullsFiles.loadFailed", i18n.Args{"error": read.err}), pullsPadX).Layout
		if key == d.URL {
			return notice(gtx)
		}
		// The picker stays, to go back to every commit.
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return v.filesToolbar(gtx, th, p, d) }),
			layout.Rigid(notice),
		)
	}
	files.takeRead(key, &diffRead{text: read.val})
	files.takeAnswers()
	files.takeTicks(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return v.filesToolbar(gtx, th, p, d) }),
		layout.Flexed(1, func(gtx C) D { return v.filesBody(gtx, th) }),
	)
}

// filesRead is the diff the Files tab shows, the whole pull request's or the
// picked commits', and the key its cards are kept under. It sets the cards'
// head and base to the commits'.
func (v *pullView) filesRead(p *pullsScreen, d backend.PRDetail) (pullsRead[string], string) {
	v.files.pr.base = ""
	from, to, ok := v.pick.span(d.URL, d.Commits)
	if !ok {
		return p.store.diff(p.project.Path, d.Number), d.URL
	}
	first, last := d.Commits[from].OID, d.Commits[to].OID
	r := p.store.commitRange(p.project.Path, first, last)
	v.files.pr.head, v.files.pr.base = last, r.val.Base
	return pullsRead[string]{val: r.val.Diff, err: r.err, loaded: r.loaded}, d.URL + "\x00" + first + ".." + last
}

// filesCount is the Files tab's count: the pull request's, or the picked
// commits' once their files are read.
func (v *pullView) filesCount(d backend.PRDetail) int {
	if _, _, ok := v.pick.span(d.URL, d.Commits); !ok {
		return d.ChangedFiles
	}
	if v.files == nil || v.files.pr.base == "" {
		return 0
	}
	return len(v.files.order)
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
			return th.VerticalRule(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if err := v.files.bodyWithBatch(gtx, th); err != nil {
				return th.Notice(err.Error()).Layout(gtx)
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
		t.rows.Tree = filetree.Build(files.order)
		stats := map[string]filetree.Delta{}
		for path, c := range files.cards {
			stats[path] = filetree.Delta{Added: c.patch.Added, Deleted: c.patch.Deleted}
		}
		t.rows.Stats = filetree.WithDirStats(t.rows.Tree, stats)
	}
	for _, node := range t.rows.Clicked(gtx) {
		if node.Dir {
			t.closed[node.Path] = !t.closed[node.Path]
			continue
		}
		t.active = node.Path
		files.scrollToFile(node.Path)
	}
	rows := filetree.VisibleRows(t.rows.Tree, func(path string) bool { return !t.closed[path] })
	return t.rows.Layout(gtx, th, rows, t.active)
}

// filesToolbar is the tree's toggle, the commit picker, the files changed
// since the review and "N of M viewed" (or, for picked commits, which ones
// and that they take no comment or tick), and the dock's split and
// collapse-all toggles; the diff's size sits on the tab strip.
func (v *pullView) filesToolbar(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	files := v.files
	if v.tree.toggle.Click.Clicked(gtx) {
		v.tree.hidden = !v.tree.hidden
	}
	treeIcon, treeLabel := icPullHide, i18n.T("pulls.pullsFiles.hideTree")
	if v.tree.hidden {
		treeIcon, treeLabel = icPullShow, i18n.T("pulls.pullsFiles.showTree")
	}
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return th.IconAction(v.root, &v.tree.toggle, treeIcon, treeLabel, false).Layout(gtx) }),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return v.commitPicker(gtx, th, p, d) }),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return v.filesProgress(gtx, th) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return files.layoutActions(gtx, th) }),
		)
	})
}

// filesProgress is the files changed since the review and "N of M viewed",
// or for picked commits the run and that it takes no comment or tick.
func (v *pullView) filesProgress(gtx C, th *ui.Theme) D {
	files := v.files
	if base := files.pr.base; base != "" {
		run := shortOID(base) + ".." + shortOID(files.pr.head)
		return th.Text(ui.TextXS, i18n.T("pulls.pullsFiles.rangeNote", i18n.Args{"range": run})).Muted().Layout(gtx)
	}
	viewed := 0
	for _, path := range files.order {
		if files.pr.isViewed(files.cards[path]) {
			viewed++
		}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return v.changedSinceReview(gtx, th) }),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("pulls.pullsFiles.viewed", i18n.Args{"viewed": viewed, "total": len(files.order)})).Muted().Layout),
	)
}

// changedSinceReview is how many files moved since they were ticked and the
// day of the last tick, nothing while none did.
func (v *pullView) changedSinceReview(gtx C, th *ui.Theme) D {
	files := v.files
	n, at := files.pr.viewed.changedSince(files.order, func(path string) string { return files.cards[path].gitHash })
	if n == 0 {
		return D{}
	}
	text := i18n.T("pulls.pullsFiles.filesChangedSinceReview", i18n.Args{"count": n, "date": reviewDay(at, gtx.Now)})
	return layout.Inset{Right: ui.Space(3)}.Layout(gtx, th.Text(ui.TextXS, text).Muted().Layout)
}

// inlineFrame is what the threads hung in the diff are drawn with this
// frame: the view lays its blocks out inside the Files tab's own layout.
type inlineFrame struct {
	th  *ui.Theme
	p   *pullsScreen
	d   backend.PRDetail
	act *pullsAction
	now time.Time
}

// diffBlocks are path's review threads still anchored to a line (an outdated
// one lives in the Conversation tab alone) and its pending comments, each
// under its line.
func (v *pullView) diffBlocks(path string) []diffview.Block {
	f := v.inline
	var out []diffview.Block
	if conv := f.p.store.conversation(f.p.project.Path, f.d.Number).val; conv != nil {
		for _, t := range conv.Threads {
			if t.Path != path || t.IsOutdated || t.Line == 0 {
				continue
			}
			id := t.ID
			out = append(out, diffview.Block{Key: "thread:" + id, At: lineRef(t.Side, t.Line), Widget: func(gtx C) D { return v.inlineThread(gtx, id) }})
		}
	}
	for i, c := range v.pendingOf(f.d.URL).snapshot() {
		if c.Path != path {
			continue
		}
		key := fmt.Sprint("pending:", i, ":", c.Body)
		index, comment := i, c
		out = append(out, diffview.Block{Key: key, At: lineRef(c.Side, c.Line), Widget: func(gtx C) D { return v.inlinePending(gtx, index, comment) }})
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

// inlinePending is a comment waiting for the review to be submitted, with
// the Conversation tab's Edit and Discard: talkClicks carries them out.
func (v *pullView) inlinePending(gtx C, i int, c backend.ReviewComment) D {
	th := v.inline.th
	s := &v.talk
	editing := s.pendingEdit.open && s.pendingEdit.index == i
	return composerSlot.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
			func(gtx C) D {
				on := th.On(th.Sidebar)
				return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D {
					body := func(gtx C) D { return on.Text(ui.TextSM, c.Body).Wrapped().Layout(gtx) }
					if editing {
						body = func(gtx C) D { return v.pendingEditBox(gtx, on) }
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, on.Text(ui.TextXS, i18n.T("pulls.pullsConversation.pendingNote")).In(on.ToneWait).Layout),
								layout.Rigid(func(gtx C) D {
									if editing {
										return D{}
									}
									editB := on.Button(clickableAt(s.edits, i), i18n.T("pulls.pullsConversation.edit"))
									editB.Size, editB.Variant = ui.SizeSM, ui.VariantGhost
									discardB := on.Button(clickableAt(s.discards, i), i18n.T("pulls.pullsConversation.discard"))
									discardB.Size, discardB.Variant = ui.SizeSM, ui.VariantDestructive
									return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(editB.Layout), ui.Gap(1), layout.Rigid(discardB.Layout))
								}),
							)
						}),
						ui.VGap(1),
						layout.Rigid(body),
					)
				})
			})
	})
}
