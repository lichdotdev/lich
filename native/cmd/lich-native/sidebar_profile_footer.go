package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icProfileFooter = icons.Lucide("chevrons-up-down")

// profileFooter is the panel's last line: the profile new sessions of the
// provider in focus are born in, in the shown project, opening a menu that
// changes it.
type profileFooter struct {
	menu  ui.MenuState
	hover widget.Clickable // the row's hover; the menu takes the press
}

// profileFocus is one provider's line: its book, the project and the profile
// that project's new sessions of it are born in.
type profileFocus struct {
	kind    string
	book    *profileBook
	project lichclient.Project
	choice  projectProfile
}

// profilePick is a footer menu choice: the project's profile setting for one
// provider, "" to follow Global again.
type profilePick struct{ key, projectID, value string }

// focusedProfile is the footer's line: the active card's provider, the
// default one for a terminal or no card. False when there is no project or
// that provider has fewer than two logins to pick between.
func (m *model) focusedProfile() (profileFocus, bool) {
	p, ok := m.project()
	if !ok {
		return profileFocus{}, false
	}
	kind := m.newSessionKind()
	if sv, ok := m.activeSession(); ok && sv.Kind != "shell" {
		kind = sv.Kind
	}
	return m.profileFocusOf(kind, p)
}

func (m *model) profileFocusOf(kind string, p lichclient.Project) (profileFocus, bool) {
	book := m.profileBook(kind)
	if book == nil || len(book.loggedIn()) < 2 {
		return profileFocus{}, false
	}
	return profileFocus{kind: kind, book: book, project: p, choice: m.providers.projectProfile(kind, p.ID)}, true
}

// otherProfileFoci are the lines of every other enabled provider with
// profiles, for the menu's Other providers.
func (m *model) otherProfileFoci(f profileFocus) []profileFocus {
	var out []profileFocus
	for _, lp := range m.launchProviders() {
		if lp.id == f.kind {
			continue
		}
		if o, ok := m.profileFocusOf(lp.id, f.project); ok {
			out = append(out, o)
		}
	}
	return out
}

// profileEntries are the logins of f's provider, its profile checked, each
// with its five-hour usage, then Use Global while the project sets its own.
func (m *model) profileEntries(th *ui.Theme, f profileFocus, act *sidebarAction) []ui.MenuEntry {
	key := f.book.provider.key
	var out []ui.MenuEntry
	for _, p := range f.book.loggedIn() {
		plan, ok := planFor(m.quota.plans(profileQuotaKey(f.book.provider, p.Slug)), f.kind)
		usage, limited := profileUsage(plan, ok)
		out = append(out, ui.MenuEntry{
			Kind: ui.MenuRadioItem, Label: p.Name, Checked: p.Slug == f.choice.shown,
			Shortcut: usage, ShortcutDestructive: limited,
			Lead: func(gtx C, size unit.Dp) D { return profileAvatar(gtx, th, p.Name, p.Slug, size) },
			OnClick: func() {
				act.profilePick = profilePick{key: key, projectID: f.project.ID, value: profileChoiceValue(p.Slug, f.project.ID)}
			},
		})
	}
	if f.choice.own {
		out = append(out, ui.MenuEntry{Kind: ui.MenuSeparator}, ui.MenuEntry{
			Label:   i18n.T("sidebar.sidebarProfile.useGlobal", i18n.Args{"profile": f.book.nameOf(f.choice.global)}),
			OnClick: func() { act.profilePick = profilePick{key: key, projectID: f.project.ID} },
		})
	}
	return out
}

// footerEntries is the footer's menu: the focused provider's logins, then a
// submenu per other provider with profiles.
func (m *model) footerEntries(th *ui.Theme, f profileFocus, act *sidebarAction) []ui.MenuEntry {
	title := i18n.T("sidebar.sidebarProfile.footerTitle", i18n.Args{"provider": f.book.provider.label, "project": f.project.Name})
	out := append([]ui.MenuEntry{{Kind: ui.MenuLabel, Label: title}}, m.profileEntries(th, f, act)...)
	others := m.otherProfileFoci(f)
	if len(others) == 0 {
		return out
	}
	var sub []ui.MenuEntry
	for _, o := range others {
		sub = append(sub, ui.MenuEntry{Kind: ui.MenuSubTrigger, Label: o.book.provider.label, Icon: providerIcon(o.kind), Sub: m.profileEntries(th, o, act)})
	}
	return append(out, ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.sidebarProfile.otherProviders"), Sub: sub})
}

// layout is the footer row: the provider's glyph, the profile's avatar and
// name, and the menu's chevrons.
func (pf *profileFooter) layout(gtx C, th *ui.Theme, root *ui.Root, m *model, f profileFocus, act *sidebarAction) D {
	name := f.book.nameOf(f.choice.shown)
	row := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return pf.hover.Layout(gtx, func(gtx C) D {
			h := gtx.Dp(ui.Space(8))
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			bg := th.Surface
			if pf.menu.Opened() || pf.hover.Hovered() {
				bg = th.Over(ui.Alpha(th.Muted, ghostHoverFill))
				ui.Fill(gtx, bg, ui.RadiusMD)
			}
			on := th.On(bg)
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return providerIcon(f.kind).Layout(gtx, 14, on.MutedForeground) }),
					ui.Gap(2),
					layout.Rigid(func(gtx C) D { return profileAvatar(gtx, on, name, f.choice.shown, ui.Space(4)) }),
					ui.Gap(2),
					layout.Flexed(1, on.Text(ui.TextSM, name).Layout),
					layout.Rigid(func(gtx C) D { return icProfileFooter.Layout(gtx, 14, on.MutedForeground) }),
				)
			})
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(1)))
			return ui.Fill(gtx, th.Over(th.Border), 0)
		}),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return th.DropdownMenu(root, &pf.menu, m.footerEntries(th, f, act)).Layout(gtx, row) }),
	)
}
