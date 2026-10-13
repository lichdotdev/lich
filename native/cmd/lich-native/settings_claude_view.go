package main

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (claudeProfile.*).
const (
	profileDefaultTag = "· default"
	profileLogIn      = "Log in"
	profileLogInAgain = "Log in again"
	profileCopyFrom   = "Copy plugins from…"
	profileRename     = "Rename"
	profileDelete     = "Delete"
	profileNew        = "New profile"
	profileOptions    = "Options for %s"
	profilePlanTitle  = "Plan usage · %s"
	advancedBinary    = "Advanced · Binary"
	profileLoading    = "Reading the profiles…"
)

var (
	profilePickerW   = ui.Space(56)
	profileAvatarRow = ui.Space(7)
	profileAvatarSel = ui.Space(5)
	profileRowPadY   = ui.Space(2.5)
	icProfileMore    = icons.Lucide("ellipsis")
	icProfileLogIn   = icons.Lucide("log-in")
	icProfileCopy    = icons.Lucide("copy")
	icProfileRename  = icons.Lucide("pencil")
	icProfileDelete  = icons.Lucide("trash-2")
	icProfileNew     = icons.Lucide("plus")
	icFoldOpen       = icons.Lucide("chevron-down")
	icFoldShut       = icons.Lucide("chevron-right")
)

// profileAvatar is a profile's initial on its color: the same hash the scope
// selector paints a project with, so a profile keeps its color across screens.
func profileAvatar(gtx C, th *ui.Theme, name, slug string, size unit.Dp) D {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	bg := projectSwatch(slug)
	ui.Fill(gtx, bg, size/2)
	label := th.On(bg).Text(ui.TextXS, profileInitial(name)).Weight(font.SemiBold)
	return layout.Center.Layout(gtx, label.Layout)
}

// effectiveScopeProfile is the profile the open scope's new sessions are born
// in: the project's own value when it sets one, else Global's.
func effectiveScopeProfile(global, project string, projectSet bool) string {
	if projectSet {
		return effectiveProfile(project)
	}
	return effectiveProfile(global)
}

// differingProfile is the profile a project's list row names: its own value
// when it differs from Global's, "" when it follows or restates it.
func differingProfile(global, project string, projectSet bool) string {
	if !projectSet || effectiveProfile(project) == effectiveProfile(global) {
		return ""
	}
	return effectiveProfile(project)
}

func (s *settingsScreen) scopeProfile(values scopedValues, m providersMachine) string {
	project, set := "", false
	if s.scope != "" {
		project, set = values.set(claudeProfileKey, s.scope)
	}
	return effectiveScopeProfile(m.globals[claudeProfileKey], project, set)
}

// profileTags is the profile a project's Claude Code row names in the list when
// it uses another than Global's.
func (s *settingsScreen) profileTags(kind string, values scopedValues, m providersMachine) []string {
	if kind != claudeKind || s.scope == "" || s.providers.claude.book == nil {
		return nil
	}
	project, set := values.set(claudeProfileKey, s.scope)
	slug := differingProfile(m.globals[claudeProfileKey], project, set)
	if slug == "" {
		return nil
	}
	return []string{s.providers.claude.book.nameOf(slug)}
}

// profilePickerRow is Profile: which login new sessions are born in, in the
// scope system like any overridable row.
func (s *settingsScreen) profilePickerRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, first bool) D {
	book := s.providers.claude.book
	e := settingEntry{title: profileTitle, key: claudeProfileKey, scope: scopeOverridable}
	r := s.providerRow(gtx, e, values)
	r.description = profileDesc
	r.globalValue = book.nameOf(m.globals[claudeProfileKey])
	shown := s.scopeProfile(values, m)
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return s.profilePicker(gtx, th, shown) })
}

func (s *settingsScreen) profilePicker(gtx C, th *ui.Theme, shown string) D {
	c := &s.providers.claude
	list, _ := c.book.snapshot()
	pop := th.DropdownPanel(s.root, &c.picker, profilePickerW, func(gtx C, th *ui.Theme) D {
		var rows []layout.FlexChild
		for _, p := range list {
			rows = append(rows, layout.Rigid(func(gtx C) D { return s.profileOption(gtx, th, p, p.Slug == shown) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
	name := c.book.nameOf(shown)
	lead := func(gtx C) D { return profileAvatar(gtx, th, name, shown, profileAvatarSel) }
	return pop.Layout(gtx, func(gtx C) D { return selectTriggerWith(gtx, th, lead, name, profilePickerW) })
}

// profileOption is one SelectItem of the picker: the avatar and the name,
// checked when chosen.
func (s *settingsScreen) profileOption(gtx C, th *ui.Theme, p lichclient.ClaudeProfile, chosen bool) D {
	c := &s.providers.claude
	if c.options == nil {
		c.options = map[string]*widget.Clickable{}
	}
	click := clickableOf(c.options, p.Slug)
	if click.Clicked(gtx) {
		s.setClaudeProfile(p.Slug)
		c.picker.Close()
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
				layout.Rigid(func(gtx C) D { return profileAvatar(gtx, on, p.Name, p.Slug, profileAvatarSel) }),
				ui.Gap(2),
				layout.Flexed(1, on.Text(ui.TextSM, p.Name).Layout),
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

// profilesBlock is Profiles: the logins this machine keeps for Claude Code, a
// row each, and the way to add one. It is global; from a project it is dimmed.
func (s *settingsScreen) profilesBlock(gtx C, th *ui.Theme, values scopedValues, first bool) D {
	r := s.providerRow(gtx, settingEntry{title: profilesTitle}, values)
	r.description = profilesDesc
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D { return s.profileRows(gtx, th) })
}

func (s *settingsScreen) profileRows(gtx C, th *ui.Theme) D {
	c := &s.providers.claude
	list, loaded := c.book.snapshot()
	if !loaded {
		return th.Text(ui.TextXS, profileLoading).Muted().Layout(gtx)
	}
	var children []layout.FlexChild
	for i, p := range list {
		children = append(children, layout.Rigid(func(gtx C) D { return s.profileListRow(gtx, th, p, i == 0) }))
	}
	if c.add.Clicked(gtx) {
		s.startNewProfile()
	}
	add := th.Button(&c.add, profileNew)
	add.Variant, add.Size, add.Icon = ui.VariantGhost, ui.SizeSM, icProfileNew
	children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return layout.W.Layout(gtx, add.Layout) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// profileListRow is one profile: its mark and name, the account it is logged
// in to or the amber word that it is not, and its menu.
func (s *settingsScreen) profileListRow(gtx C, th *ui.Theme, p lichclient.ClaudeProfile, first bool) D {
	st := s.providers.claude.row(p.Slug)
	if st.login.Clicked(gtx) {
		s.startLogin(p, nil)
	}
	row := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return profileAvatar(gtx, th, p.Name, p.Slug, profileAvatarRow) }),
			ui.Gap(3),
			layout.Flexed(1, func(gtx C) D { return profileText(gtx, th, p) }),
			ui.Gap(2),
		}
		if !p.LoggedIn {
			in := th.Button(&st.login, profileLogIn)
			in.Variant, in.Size = ui.VariantOutline, ui.SizeSM
			children = append(children, layout.Rigid(in.Layout), ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return s.profileMenu(gtx, th, p, st) }))
		return layout.Inset{Top: profileRowPadY, Bottom: profileRowPadY}.Layout(gtx,
			func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...) })
	}
	if first {
		return row(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(th.Separator), layout.Rigid(row))
}

// profileText is a row's name and the line under it.
func profileText(gtx C, th *ui.Theme, p lichclient.ClaudeProfile) D {
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, p.Name).Weight(font.Medium).Layout)}
	if p.Personal {
		name = append(name, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, profileDefaultTag).Muted().Layout))
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, name...) }),
		layout.Rigid(func(gtx C) D { return profileStatusLine(gtx, th, p) }),
	}
	if len(p.Copied) > 0 {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, profilesCopy).Muted()) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func profileStatusLine(gtx C, th *ui.Theme, p lichclient.ClaudeProfile) D {
	switch {
	case p.LoggedIn:
		return th.Text(ui.TextXS, profileAccountLine(p)).Muted().Layout(gtx)
	case p.StatusError != "":
		return th.Text(ui.TextXS, p.StatusError).In(th.Destructive).Layout(gtx)
	}
	return th.Text(ui.TextXS, profileNotLoggedIn).In(th.ToneWait).Layout(gtx)
}

// profileMenu is a row's ⋯: logging in again, bringing plugins over and
// renaming for every profile, deleting for every one but Personal.
func (s *settingsScreen) profileMenu(gtx C, th *ui.Theme, p lichclient.ClaudeProfile, st *profileRowState) D {
	entries := []ui.MenuEntry{
		{Label: profileLogInAgain, Icon: icProfileLogIn, OnClick: func() { s.startLogin(p, nil) }},
		{Label: profileCopyFrom, Icon: icProfileCopy, Disabled: !p.LoggedIn, OnClick: func() { s.startCopy(p) }},
		{Label: profileRename, Icon: icProfileRename, OnClick: func() { s.startRename(p) }},
	}
	if !p.Personal {
		entries = append(entries, ui.MenuEntry{Kind: ui.MenuSeparator},
			ui.MenuEntry{Label: profileDelete, Icon: icProfileDelete, Variant: ui.VariantDestructive, OnClick: func() { s.startDelete(p) }})
	}
	hint := fmt.Sprintf(profileOptions, p.Name)
	return th.DropdownMenu(s.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &st.tip, hint).Layout(gtx, func(gtx C) D {
			btn := th.Button(&st.more, "")
			btn.Icon, btn.Variant, btn.Size = icProfileMore, ui.VariantGhost, ui.SizeIconSM
			return btn.Layout(gtx)
		})
	})
}

// claudePlanBlock is Plan usage for the profile the open scope uses: Personal
// is the machine's own reading, any other profile is asked for by itself.
func (s *settingsScreen) claudePlanBlock(gtx C, th *ui.Theme, values scopedValues, slug string, plan lichclient.QuotaPlan, first bool) D {
	title := fmt.Sprintf(profilePlanTitle, s.providers.claude.book.nameOf(slug))
	return s.planBlock(gtx, th, values, plan, first, title)
}

// claudePlan is the reading of the profile slug, the one the pane watches.
func (s *settingsScreen) claudePlan(slug string) (lichclient.QuotaPlan, bool) {
	if s.footer == nil {
		return lichclient.QuotaPlan{}, false
	}
	return planFor(s.footer.quota.plans(profileQuotaKey(slug)), claudeKind)
}

// advancedBinaryFold is the Binary block under a fold: a Claude Code binary is
// rarely changed once the profiles are what the screen is about.
func (s *settingsScreen) advancedBinaryFold(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	c := &s.providers.claude
	if c.fold.Clicked(gtx) {
		c.advanced = !c.advanced
	}
	if s.lit == binTitle && gtx.Now.Before(s.litUntil) {
		c.advanced = true
	}
	lead := icFoldShut
	if c.advanced {
		lead = icFoldOpen
	}
	head := func(gtx C) D {
		return c.fold.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			ink := th.MutedForeground
			if c.fold.Hovered() {
				ink = th.Foreground
			}
			return layout.Inset{Left: providerRowPadX, Right: providerRowPadX, Top: providerRowPadY, Bottom: providerRowPadY}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return lead.Layout(gtx, ui.Space(4), ink) }),
					ui.Gap(2),
					layout.Rigid(th.Text(ui.TextSM, advancedBinary).In(ink).Layout),
				)
			})
		})
	}
	if !c.advanced {
		return head(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(head),
		layout.Rigid(func(gtx C) D { return s.binBlock(gtx, th, values, m, p, false) }),
	)
}
