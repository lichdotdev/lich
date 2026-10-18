package main

import (
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icDelegates = icons.Lucide("network")

// delegatesChip is a pane header's offer to lay out the delegates its
// session opened that are not in the layout yet, nothing while the layout
// is zoomed or every delegate is there.
func (u *window) delegatesChip(gtx C, th *ui.Theme, v stageView, sv sessionView, p *paneUI) D {
	if v.layout.Zoomed {
		return D{}
	}
	o := u.model.delegateOffer(sv.ID)
	if len(o.missing) == 0 {
		return D{}
	}
	tip := th.Tooltip(&u.root, &p.delegatesTip, i18n.T("layouts.delegates.hint"))
	return layout.Inset{Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return tip.Layout(gtx, func(gtx C) D {
			return p.delegates.Layout(gtx, func(gtx C) D {
				return pill(gtx, th, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return icDelegates.Layout(gtx, 12, th.MutedForeground) }),
						ui.Gap(1.5),
						layout.Rigid(th.Text(ui.TextXS, delegatesText(o)).Muted().Layout),
					)
				}, ui.Space(2), ui.Space(2))
			})
		})
	})
}

// delegatesText is the chip's words: how many delegates there are, and how
// many of them the layout shows once it shows any.
func delegatesText(o delegateOffer) string {
	if o.here == 0 {
		return i18n.T("layouts.delegates.showBeside", i18n.Args{"count": len(o.all)})
	}
	return i18n.T("layouts.delegates.add", i18n.Args{"here": o.here, "count": len(o.all), "missing": len(o.missing)})
}

// layDelegatesOut lays out as many of orchestrator's missing delegates as
// fit at delegateSlot, refused with a toast when none does and asked about
// first when any sits in another layout.
func (u *window) layDelegatesOut(orchestrator string) {
	o := u.model.delegateOffer(orchestrator)
	target, below := u.model.delegateSlot(orchestrator, o.all)
	ids := o.missing[:min(len(o.missing), u.layoutStage.pointer.columnRoom(target, below))]
	if len(ids) == 0 {
		u.toaster.Show(ui.Toast{Title: i18n.T("layouts.split.noRoom")})
		return
	}
	if away := u.model.inOtherLayouts(orchestrator, ids); away > 0 {
		u.askMoveDelegates(orchestrator, ids, away)
		return
	}
	u.model.showDelegates(u.ctx, orchestrator, ids)
	u.model.invalidate()
}
