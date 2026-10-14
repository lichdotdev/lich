package main

import (
	"fmt"
	"image"
	"log"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (settings.versionControlSettings).
const (
	vcsAccountTitle    = "GitHub account"
	vcsAccountFor      = "GitHub account for %s"
	vcsAccountAlso     = "login gh switch"
	vcsAccountDesc     = "Which account lich runs gh as for this project — pull requests, checks and PR checkouts. gh keeps one active account per host, so a repository only another account can see reads as not found. Accounts on an enterprise host are listed with it."
	vcsActiveAccount   = "gh's active account"
	vcsListFailed      = "Could not list GitHub accounts: %s"
	vcsNoAccounts      = "gh is signed in to no account. Run `gh auth login` in a terminal."
	vcsActiveAccountID = "" // the stored value of no override
)

// vcsPickerWidth is the SelectTrigger's w-72.
var vcsPickerWidth = ui.Space(72)

// vcsAccountBlock is VersionControlSettings: the project's checkout, the
// account picker, what gh said if it could not list, and who commits land
// as. Only a project's scope draws it.
func (s *settingsScreen) vcsAccountBlock(gtx C, th *ui.Theme, r settingRow, first bool) D {
	v := &s.vcs
	project, path := s.scope, v.pathOf(s.scope)
	stored, _ := s.snapshot().set(vcsAccountKey, project)
	v.mu.Lock()
	accounts, listErr := v.accounts, v.accountsErr
	v.mu.Unlock()
	if up := upgradeAccount(stored, accounts); up != "" {
		s.setAccount(project, up)
		stored = up
	}
	options := accountOptions(accounts, stored)
	r.title = fmt.Sprintf(vcsAccountFor, s.scopeName())
	r.description = vcsAccountDesc
	under := func(gtx C, th *ui.Theme) D {
		children := []layout.FlexChild{
			layout.Rigid(th.Text(ui.TextXS, path).Muted().Layout),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D { return s.vcsPicker(gtx, th, project, stored, options) }),
		}
		var notice ui.LabelStyle
		switch {
		case listErr != "":
			notice = th.Text(ui.TextXS, fmt.Sprintf(vcsListFailed, listErr)).In(th.Destructive)
		case accounts != nil && len(accounts) == 0:
			notice = th.Text(ui.TextXS, vcsNoAccounts).Muted()
		}
		if notice.Text != "" {
			notice.MaxLines = 0
			children = append(children, ui.VGap(2), layout.Rigid(notice.Layout))
		}
		if id, ok := v.identity(s, path); ok && !v.failed(vcsGitBin) {
			children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return identityRow(gtx, th, identityLineOf(id)) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return r.layoutBlock(gtx, th, first, under)
}

func (s *settingsScreen) scopeName() string {
	for _, p := range s.projects {
		if p.id == s.scope {
			return p.name
		}
	}
	return ""
}

// identity is path's commit identity, read once per visit; ok false until
// git answered.
func (v *vcsSettings) identity(s *settingsScreen, path string) (lichclient.CommitIdentity, bool) {
	return v.identities.get(&v.mu, path, s.invalidate, func() (lichclient.CommitIdentity, bool) {
		id, err := s.client.CommitIdentity(s.ctx, path)
		if err != nil {
			log.Printf("settings: commit identity of %s: %v", path, err)
			return id, false
		}
		return id, true
	})
}

// setAccount stores project's account, "" for gh's active one, and only
// then has the pull request badges ask again: a re-read that overtook the
// write would replay the old account's answer.
func (s *settingsScreen) setAccount(projectID, account string) {
	s.mu.Lock()
	if account == vcsActiveAccountID {
		delete(s.values[vcsAccountKey], projectID)
	} else {
		if s.values == nil {
			s.values = scopedValues{}
		}
		if s.values[vcsAccountKey] == nil {
			s.values[vcsAccountKey] = map[string]string{}
		}
		s.values[vcsAccountKey][projectID] = account
	}
	s.mu.Unlock()
	go func() {
		if err := s.client.SetSetting(s.ctx, vcsAccountKey, projectID, account); err != nil {
			log.Printf("settings: save %s: %v", vcsAccountKey, err)
			return
		}
		if s.vcs.onAccount != nil {
			s.vcs.onAccount()
		}
	}()
}

// identityRow is the line under a top border saying who commits land as,
// the email in mono.
func identityRow(gtx C, th *ui.Theme, l identityLine) D {
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))}.Op())
	lead := th.Text(ui.TextXS, l.lead).Muted()
	if l.email == "" {
		lead = th.Text(ui.TextXS, l.lead)
	}
	children := []layout.FlexChild{layout.Rigid(lead.Layout)}
	if l.email != "" {
		children = append(children, ui.Gap(1), layout.Rigid(th.Text(ui.TextXS, l.email).Muted().Mono().Layout))
	}
	note := th.Text(ui.TextXS, l.note).Muted()
	note.MaxLines = 0
	children = append(children, ui.Gap(1), layout.Flexed(1, note.Layout))
	return layout.Inset{Top: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, children...)
	})
}

// vcsPicker is the Select: the outline trigger naming the account, and the
// list of gh's active account and every option under it.
func (s *settingsScreen) vcsPicker(gtx C, th *ui.Theme, project, stored string, options []string) D {
	v := &s.vcs
	label := vcsActiveAccount
	if stored != "" {
		label = accountLabel(stored, options)
	}
	pop := th.DropdownPanel(s.root, &v.picker, vcsPickerWidth, func(gtx C, th *ui.Theme) D {
		rows := []layout.FlexChild{layout.Rigid(func(gtx C) D {
			return s.vcsOption(gtx, th, project, vcsActiveAccountID, vcsActiveAccount, stored == "")
		})}
		for _, o := range options {
			rows = append(rows, layout.Rigid(func(gtx C) D {
				return s.vcsOption(gtx, th, project, o, accountLabel(o, options), o == stored)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
	return pop.Layout(gtx, func(gtx C) D { return selectTrigger(gtx, th, label, vcsPickerWidth) })
}

// vcsOption is one SelectItem, checked when chosen; picking it stores it and
// closes the list.
func (s *settingsScreen) vcsOption(gtx C, th *ui.Theme, project, value, label string, chosen bool) D {
	v := &s.vcs
	if v.options == nil {
		v.options = map[string]*widget.Clickable{}
	}
	click := clickableOf(v.options, value)
	if click.Clicked(gtx) {
		s.setAccount(project, value)
		v.picker.Close()
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

// selectTrigger is the outline SelectTrigger, width wide, naming the value.
func selectTrigger(gtx C, th *ui.Theme, value string, width unit.Dp) D {
	return selectTriggerWith(gtx, th, nil, value, width)
}

// selectTriggerWith is selectTrigger with a mark of its own before the value.
func selectTriggerWith(gtx C, th *ui.Theme, lead layout.Widget, value string, width unit.Dp) D {
	size := image.Pt(gtx.Dp(width), gtx.Dp(ui.Space(9)))
	gtx.Constraints = layout.Exact(size)
	bg := th.Over(ui.Alpha(th.Palette.Input, 0.3))
	ui.Fill(gtx, bg, ui.RadiusMD)
	insetBorder(gtx, size, gtx.Dp(ui.RadiusMD), th.Over(th.Palette.Input))
	on := th.On(bg)
	return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		var children []layout.FlexChild
		if lead != nil {
			children = append(children, layout.Rigid(lead), ui.Gap(2))
		}
		children = append(children,
			layout.Flexed(1, on.Text(ui.TextSM, value).Layout),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return icChevronDown.Layout(gtx, ui.Space(4), on.MutedForeground) }),
		)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}
