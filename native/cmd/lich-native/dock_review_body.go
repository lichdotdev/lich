package main

import (
	"context"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icCollapseAll = icons.Lucide("chevrons-down-up")
	icExpandAll   = icons.Lucide("chevrons-up-down")
	icSplit       = icons.Lucide("columns-2")
)

func loadHideWhitespace(ctx context.Context, c *backend.Client) (bool, error) {
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
	bulk, split ui.IconActionState
}

// layoutActions is the Review tab's part of the dock header: the
// unified/split toggle, which starts from Settings › Editor's diff layout and
// then holds per panel, shown only while the panel is wide enough to split,
// and collapse/expand all.
func (p *reviewPanel) layoutActions(gtx C, th *ui.Theme) D {
	a := &p.actions
	for a.bulk.Click.Clicked(gtx) {
		p.bulkOpen, p.bulk = !p.bulkOpen, p.bulk+1
	}
	for a.split.Click.Clicked(gtx) {
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
	bulk := layout.Rigid(func(gtx C) D { return th.IconAction(p.root, &a.bulk, bulkIcon, bulkLabel, false).Layout(gtx) })
	if !p.splitOK {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, bulk)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return th.IconAction(p.root, &a.split, icSplit, splitLabel, p.split).Layout(gtx) }),
		ui.Gap(1),
		bulk,
	)
}

// panelBody is PanelBody: the reason there is nothing to show, or the files.
func (p *reviewPanel) panelBody(gtx C, th *ui.Theme) {
	if r := p.resolvers[p.resolving]; r != nil {
		if r.layout(gtx, th, p) {
			p.resolving = ""
		}
		return
	}
	switch r := p.shown; {
	case r == nil:
		th.Notice(i18n.T("diff.reviewPanel.loading")).Layout(gtx)
	case r.failed:
		th.Notice(i18n.T("diff.reviewPanel.notRepository")).Layout(gtx)
	case len(p.order) == 0:
		th.Notice(i18n.T("diff.reviewPanel.noChanges")).Layout(gtx)
	default:
		p.fileList(gtx, th)
	}
}

// fileList lays out the cards, each after the first set off by a top border.
func (p *reviewPanel) fileList(gtx C, th *ui.Theme) {
	p.splitOK = gtx.Constraints.Max.X >= gtx.Dp(reviewSplitMinWidth)
	heights := map[string]cardSpan{}
	p.scrollArea(gtx, th, len(p.order), func(gtx C, i int) D {
		return p.cardItem(gtx, i, heights)
	})
	p.layoutSticky(gtx, heights)
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
				dims := layout.Spacer{Height: ui.Space(2.5)}.Layout(gtx)
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
