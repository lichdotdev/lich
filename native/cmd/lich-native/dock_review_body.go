package main

import (
	"context"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icCollapseAll = icons.Lucide("chevrons-down-up")
	icExpandAll   = icons.Lucide("chevrons-up-down")
	icSplit       = icons.Lucide("columns-2")
)

func loadHideWhitespace(ctx context.Context, c *lichclient.Client) (bool, error) {
	raw, err := c.GetSetting(ctx, hideWhitespaceKey, "")
	return raw == "true", err
}

// setHiding is Settings › Editor's whitespace switch, live: every card
// redraws from the diff it holds.
func (p *reviewPanel) setHiding(hiding bool) {
	if hiding == p.hiding {
		return
	}
	p.hiding = hiding
	for _, c := range p.cards {
		c.sync(c.git, hiding)
	}
}

// reviewActions are the header's IconActions while the Review tab is on.
type reviewActions struct {
	bulk, split iconActionState
}

// layoutActions is the Review tab's part of the dock header: the
// unified/split toggle, which starts from Settings › Editor's diff layout and
// then holds per panel, and collapse/expand all, as RightDock.tsx draws it.
func (p *reviewPanel) layoutActions(gtx C, th *ui.Theme) D {
	a := &p.actions
	for a.bulk.click.Clicked(gtx) {
		p.bulkOpen, p.bulk = !p.bulkOpen, p.bulk+1
	}
	for a.split.click.Clicked(gtx) {
		p.split = !p.split
	}
	bulkIcon, bulkLabel := icExpandAll, i18n.T("diff.diffBulk.expandAll")
	if p.bulkOpen {
		bulkIcon, bulkLabel = icCollapseAll, i18n.T("diff.diffBulk.collapseAll")
	}
	splitLabel := i18n.T("diff.dockReviewBody.split")
	if p.split {
		splitLabel = i18n.T("diff.dockReviewBody.unified")
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return iconAction(gtx, th, p.root, &a.split, icSplit, splitLabel, p.split) }),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D { return iconAction(gtx, th, p.root, &a.bulk, bulkIcon, bulkLabel, false) }),
	)
}

// panelBody is PanelBody: the reason there is nothing to show, or the files.
func (p *reviewPanel) panelBody(gtx C, th *ui.Theme) {
	switch r := p.shown; {
	case r == nil:
		notice(gtx, th, i18n.T("diff.reviewPanel.loading"))
	case r.failed:
		notice(gtx, th, i18n.T("diff.reviewPanel.notRepository"))
	case len(p.order) == 0:
		notice(gtx, th, i18n.T("diff.reviewPanel.noChanges"))
	default:
		p.fileList(gtx, th)
	}
}

// fileList is FileList: the cards p-2, each after the first set off by
// mt-2.5 border-t pt-2.5, and the note when a split does not fit.
func (p *reviewPanel) fileList(gtx C, th *ui.Theme) {
	p.splitOK = gtx.Constraints.Max.X-2*gtx.Dp(ui.Space(2)) >= gtx.Dp(splitMinWidth)
	heights := map[string]cardSpan{}
	squeezed := p.split && !p.splitOK
	count := len(p.order)
	if squeezed {
		count++
	}
	th.ScrollArea(&p.list).Layout(gtx, count, func(gtx C, i int) D {
		if squeezed {
			if i == 0 {
				return p.squeezedNote(gtx, th)
			}
			i--
		}
		return p.cardItem(gtx, i, heights)
	})
	if !squeezed {
		p.layoutSticky(gtx, heights)
	}
}

func (p *reviewPanel) cardItem(gtx C, i int, heights map[string]cardSpan) D {
	c := p.cards[p.order[i]]
	if c.bulkSeen != p.bulk {
		c.bulkSeen, c.open = p.bulk, p.bulkOpen
	}
	inset := layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(2.5)}
	if i == 0 {
		inset.Top = ui.Space(2)
	}
	if i == len(p.order)-1 {
		inset.Bottom = ui.Space(2)
	}
	top := gtx.Dp(inset.Top)
	return inset.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if i == 0 {
					return D{}
				}
				dims := layout.Inset{Bottom: ui.Space(2.5)}.Layout(gtx, p.th.Separator)
				top += dims.Size.Y
				return dims
			}),
			layout.Rigid(func(gtx C) D {
				dims := c.layout(gtx, p)
				heights[p.order[i]] = cardSpan{top: top, height: dims.Size.Y, header: c.headerHeight}
				return dims
			}),
		)
	})
}

func (p *reviewPanel) squeezedNote(gtx C, th *ui.Theme) D {
	small := unit.Sp(11)
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icSplit.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
			ui.Gap(1.5),
			layout.Rigid(th.Text(small, i18n.T("diff.reviewPanel.squeezed")).Muted().Layout),
		)
	})
}
