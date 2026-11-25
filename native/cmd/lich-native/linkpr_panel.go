package main

import (
	"context"
	"strings"
	"sync"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The Link pull request… panel, from the approved mockup (lody-pendentes,
// section 4): 300px wide, 10px around, 6px under its label, 8px over its
// buttons and 6px between them.
var (
	linkPRPanelW     = ui.Space(75)
	linkPRPanelPad   = ui.Space(2.5)
	linkPRLabelGap   = float32(1.5)
	linkPRButtonsGap = float32(2)
	linkPRButtonGap  = float32(1.5)
)

// linkPRPanel is the card menu's Link pull request…: one field for #N or a
// URL, Cancel, and Link, which closes it once the backend kept the link.
type linkPRPanel struct {
	panel        ui.DropdownPanelState
	field        widget.Editor
	cancel, link ui.Clickable
	// session, path and forge are the card's that asked: #N reads in path's
	// repository.
	session, path, forge string

	mu sync.Mutex
	// gen is the open the link running belongs to, so one that answers after
	// the panel was opened again for another card is dropped.
	gen  int
	busy bool
	done bool
	err  string
}

// show opens the panel for sv where the pointer is, on the menu row picked.
func (p *linkPRPanel) show(sv sessionView, root *ui.Root) {
	p.open(sv)
	p.panel.OpenAtPointer(root)
}

func (p *linkPRPanel) open(sv sessionView) {
	p.session, p.path, p.forge = sv.ID, sv.path, sv.forge
	p.field.SingleLine, p.field.Submit = true, true
	p.field.SetText("")
	p.mu.Lock()
	p.gen++
	p.busy, p.done, p.err = false, false, ""
	p.mu.Unlock()
}

func (p *linkPRPanel) layout(ctx context.Context, gtx C, th *ui.Theme, root *ui.Root, client *backend.Client, invalidate func()) {
	if !p.panel.Opened() {
		return
	}
	pop := th.DropdownPanel(root, &p.panel, linkPRPanelW, func(gtx C, th *ui.Theme) D {
		return p.content(ctx, gtx, th, client, invalidate)
	})
	pop.Layout(gtx, func(C) D { return D{} })
}

func (p *linkPRPanel) content(ctx context.Context, gtx C, th *ui.Theme, client *backend.Client, invalidate func()) D {
	submit, escaped := p.fieldEvents(gtx)
	p.mu.Lock()
	busy, done, errText := p.busy, p.done, p.err
	p.mu.Unlock()
	ref := strings.TrimSpace(p.field.Text())
	if done || escaped || p.cancel.Clicked(gtx) {
		p.panel.Close()
	}
	if (submit || p.link.Clicked(gtx)) && ref != "" && !busy {
		p.start(ctx, client, ref, invalidate)
		busy = true
	}
	if !gtx.Focused(&p.field) {
		gtx.Execute(key.FocusCmd{Tag: &p.field})
	}
	field := th.Input(&p.field, i18n.T(forgeKey(p.forge, "sidebar.sessionCard.linkPlaceholder", "sidebar.sessionCard.linkPlaceholderGitLab")))
	field.TextSize, field.Disabled = ui.TextXS, busy
	cancel := th.Button(&p.cancel, i18n.T("common.action.cancel"))
	cancel.Variant, cancel.Size = ui.VariantGhost, ui.SizeSM
	link := th.Button(&p.link, i18n.T("sidebar.sessionCard.link"))
	link.Size, link.Disabled = ui.SizeSM, busy || ref == ""
	title := i18n.T(forgeKey(p.forge, "sidebar.sessionCard.pullRequest", "sidebar.sessionCard.mergeRequest"))
	return layout.UniformInset(linkPRPanelPad).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, title).Muted().Layout),
			ui.VGap(linkPRLabelGap),
			layout.Rigid(field.Layout),
			layout.Rigid(func(gtx C) D {
				if errText == "" {
					return D{}
				}
				return layout.Inset{Top: ui.Space(linkPRLabelGap)}.Layout(gtx, th.Text(ui.TextXS, errText).In(th.Destructive).Wrapped().Layout)
			}),
			ui.VGap(linkPRButtonsGap),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
					layout.Rigid(cancel.Layout), ui.Gap(linkPRButtonGap), layout.Rigid(link.Layout))
			}),
		)
	})
}

// fieldEvents reports Enter and Escape in the field, and drops the error once
// what it holds changes.
func (p *linkPRPanel) fieldEvents(gtx C) (submit, escaped bool) {
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.field, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			escaped = true
		}
	}
	for {
		ev, ok := p.field.Update(gtx)
		if !ok {
			return submit, escaped
		}
		switch ev.(type) {
		case widget.SubmitEvent:
			submit = true
		case widget.ChangeEvent:
			p.mu.Lock()
			p.err = ""
			p.mu.Unlock()
		}
	}
}

// start links ref off the frame. The panel closes once the backend kept it;
// the badges hear of it from api.SessionPullRequestsEvent.
func (p *linkPRPanel) start(ctx context.Context, client *backend.Client, ref string, invalidate func()) {
	p.mu.Lock()
	gen := p.gen
	p.busy, p.err = true, ""
	p.mu.Unlock()
	session, path := p.session, p.path
	go func() {
		err := client.LinkPullRequest(ctx, session, path, ref)
		p.mu.Lock()
		if gen == p.gen {
			p.busy, p.done = false, err == nil
			if err != nil {
				p.err = i18n.T("sidebar.sessionCard.linkFailed", i18n.Args{"error": err.Error()})
			}
		}
		p.mu.Unlock()
		invalidate()
	}()
}
