package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (pulls.pullsFiles.*).
const (
	pullsFilesViewed = "%d of %d viewed"
	pullsViewedLabel = "Viewed"
)

// filesInput is what the Files tab needs from the window this frame: the
// active session, whose terminal takes injected references, and its
// checkout, whose comment batch the tab shares with the dock.
type filesInput struct {
	sessionID, checkout string
	light               bool
}

// viewedTick is the card's Viewed checkbox; ticking folds the file away.
func (c *fileCard) viewedTick(gtx C, th *ui.Theme, f *prFiles) D {
	tick, ok := f.ticks[c.git.path]
	if !ok {
		tick = new(widget.Bool)
		f.ticks[c.git.path] = tick
	}
	tick.Value = f.isViewed(c)
	if tick.Update(gtx) {
		if tick.Value {
			f.viewed[c.git.path] = c.gitText
			c.open = false
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
func (v *pullView) filesTab(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail) D {
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
	if len(d.Commits) > 0 {
		files.pr.head = d.Commits[len(d.Commits)-1].OID
	}
	in := p.files
	files.th, files.light, files.target = th, in.light, in.checkout
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
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if err := files.bodyWithBatch(gtx, th); err != nil {
				return notice(gtx, th, err.Error())
			}
			return D{Size: gtx.Constraints.Max}
		}),
	)
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
	return layout.Inset{Left: pullsPadX, Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf(pullsFilesViewed, viewed, len(files.order))).Muted().Layout),
			ui.Gap(3),
			layout.Rigid(func(gtx C) D { return diffStat(gtx, th, added, deleted) }),
			layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
			layout.Rigid(func(gtx C) D { return files.layoutActions(gtx, th) }),
		)
	})
}
