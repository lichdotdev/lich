package main

import (
	"image"
	"log"
	"slices"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// accountGate is why the Pulls screen asks for a gh account before it lists.
type accountGate int

const (
	gateNone accountGate = iota
	// gateChoose: the project runs as gh's active account and gh holds more
	// than one, so which one answers is up to whatever gh last switched to.
	gateChoose
	// gateHidden: GitHub hides the repository from the project's account.
	gateHidden
)

func accountGateOf(st backend.GitHubAccountState) accountGate {
	switch {
	case len(st.Accounts) < 2:
		return gateNone
	case st.Account == "":
		return gateChoose
	case st.Hidden:
		return gateHidden
	}
	return gateNone
}

// defaultPick is the account the gate's picker starts on: gh's first, which
// is its active one, or the first other than the one that cannot see.
func defaultPick(st backend.GitHubAccountState) string {
	for _, a := range st.Accounts {
		if a != st.Account {
			return a
		}
	}
	return ""
}

// accountPicker is the gate's controls and the pick it stores.
type accountPicker struct {
	mu      sync.Mutex
	pick    string
	saving  bool
	saveErr string

	panel   ui.DropdownPanelState
	options map[string]*widget.Clickable
	confirm widget.Clickable
}

// accountGateLayout draws the gate over the whole island, ok false when the
// project needs none and the screen draws as usual.
func (p *pullsScreen) accountGateLayout(gtx C, th *ui.Theme, st backend.GitHubAccountState) (D, bool) {
	gate := accountGateOf(st)
	if gate == gateNone {
		return D{}, false
	}
	a := &p.account
	a.mu.Lock()
	if !slices.Contains(st.Accounts, a.pick) || a.pick == st.Account {
		a.pick = defaultPick(st)
	}
	pick, saving, saveErr := a.pick, a.saving, a.saveErr
	a.mu.Unlock()
	if a.confirm.Clicked(gtx) && !saving {
		p.saveAccount(pick)
	}
	title := i18n.T("pulls.pullsAccount.chooseTitle")
	description := i18n.T("pulls.pullsAccount.chooseDescription")
	button := i18n.T("pulls.pullsAccount.continue")
	if gate == gateHidden {
		_, login := splitAccount(st.Account)
		title = i18n.T("pulls.pullsAccount.hiddenTitle", i18n.Args{"account": login})
		description = i18n.T("pulls.pullsAccount.hiddenDescription")
		button = i18n.T("common.action.tryAgain")
	}
	actions := func(gtx C) D {
		b := th.Button(&a.confirm, button)
		b.Disabled = saving
		children := []layout.FlexChild{layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return p.accountPicker(gtx, th, st, pick) }),
				ui.Gap(2),
				layout.Rigid(b.Layout),
			)
		})}
		note := th.Text(ui.TextXS, i18n.T("pulls.pullsAccount.later")).Muted()
		if saveErr != "" {
			note = th.Text(ui.TextXS, i18n.T("pulls.pullsAccount.saveFailed", i18n.Args{"error": saveErr})).In(th.Destructive)
		}
		children = append(children, ui.VGap(3), layout.Rigid(note.Layout))
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
	}
	return th.EmptyState(icPullTeam, title, description, actions).Layout(gtx), true
}

// accountPicker is the Select of gh's accounts, the one picked checked.
func (p *pullsScreen) accountPicker(gtx C, th *ui.Theme, st backend.GitHubAccountState, pick string) D {
	a := &p.account
	pop := th.DropdownPanel(p.root, &a.panel, vcsPickerWidth, func(gtx C, th *ui.Theme) D {
		rows := make([]layout.FlexChild, 0, len(st.Accounts))
		for _, o := range st.Accounts {
			rows = append(rows, layout.Rigid(func(gtx C) D {
				return a.option(gtx, th, o, accountLabel(o, st.Accounts), o == pick)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
	return pop.Layout(gtx, th.SelectTrigger(accountLabel(pick, st.Accounts), vcsPickerWidth).Layout)
}

// option is one SelectItem; picking it only moves the pick; the button
// stores it.
func (a *accountPicker) option(gtx C, th *ui.Theme, value, label string, chosen bool) D {
	if a.options == nil {
		a.options = map[string]*widget.Clickable{}
	}
	click := clickableOf(a.options, value)
	if click.Clicked(gtx) {
		a.mu.Lock()
		a.pick = value
		a.mu.Unlock()
		a.panel.Close()
	}
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(fontRowHeight)))
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, on.Text(ui.TextSM, label).Layout),
				layout.Rigid(func(gtx C) D {
					if !chosen {
						return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)}
					}
					return icCheck.Layout(gtx, ui.Space(4), on.Foreground)
				}),
			)
		})
	})
}

// saveAccount stores account as the project's, the value Settings › Version
// Control writes, and only then reads again: a read that overtook the write
// would answer as the old account.
func (p *pullsScreen) saveAccount(account string) {
	a, project := &p.account, p.project.ID
	a.mu.Lock()
	a.saving, a.saveErr = true, ""
	a.mu.Unlock()
	go func() {
		err := p.store.client.SetSetting(p.store.ctx, vcsAccountKey, project, account)
		a.mu.Lock()
		a.saving = false
		if err != nil {
			log.Printf("pulls: save %s: %v", vcsAccountKey, err)
			a.saveErr = backend.ErrorText(err)
		}
		a.mu.Unlock()
		if err == nil && p.onAccount != nil {
			p.onAccount()
		}
		p.store.stale()
	}()
}
