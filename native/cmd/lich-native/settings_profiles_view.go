package main

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
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
	icProfilePrefs   = icons.Lucide("sliders-horizontal")
)

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

func (s *settingsScreen) scopeProfile(c *profileSettings, values scopedValues, m providersMachine) string {
	project, set := "", false
	if s.scope != "" {
		project, set = values.set(c.provider.key, s.scope)
	}
	return effectiveScopeProfile(m.globals[c.provider.key], project, set)
}

// profileTags is the profile a project's row of a provider with profiles names
// in the list when it uses another than Global's.
func (s *settingsScreen) profileTags(kind string, values scopedValues, m providersMachine) []string {
	c := s.providers.profiles(kind)
	if c == nil || s.scope == "" || c.book == nil {
		return nil
	}
	project, set := values.set(c.provider.key, s.scope)
	slug := differingProfile(m.globals[c.provider.key], project, set)
	if slug == "" {
		return nil
	}
	return []string{c.book.nameOf(slug)}
}

// profilePickerRow is Profile: which login new sessions are born in, in the
// scope system like any overridable row.
func (s *settingsScreen) profilePickerRow(gtx C, th *ui.Theme, c *profileSettings, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	e := settingEntry{titleKey: "settings.settingsProfiles.title", key: c.provider.key, scope: scopeOverridable}
	r := s.providerRow(gtx, e, values)
	r.description = profileSentence(c.provider, c.provider.words.profileDescKey)
	r.globalValue = c.book.nameOf(m.globals[c.provider.key])
	shown := s.scopeProfile(c, values, m)
	r.warning = s.profileBinaryWarning(gtx, c, values, m, p, c.book.nameOf(shown))
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D { return s.profilePicker(gtx, th, c, shown) })
}

func (s *settingsScreen) profilePicker(gtx C, th *ui.Theme, c *profileSettings, shown string) D {
	list, _ := c.book.snapshot()
	pop := th.DropdownPanel(s.root, &c.picker, profilePickerW, func(gtx C, th *ui.Theme) D {
		var rows []layout.FlexChild
		for _, p := range list {
			rows = append(rows, layout.Rigid(func(gtx C) D { return s.profileOption(gtx, th, c, p, p.Slug == shown) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
	name := c.book.nameOf(shown)
	lead := func(gtx C) D { return profileAvatar(gtx, th, c.book.look(shown), profileAvatarSel) }
	trigger := th.SelectTrigger(name, profilePickerW)
	trigger.Lead = lead
	return pop.Layout(gtx, trigger.Layout)
}

// profileOption is one SelectItem of the picker: the avatar and the name,
// checked when chosen.
func (s *settingsScreen) profileOption(gtx C, th *ui.Theme, c *profileSettings, p backend.Profile, chosen bool) D {
	if c.options == nil {
		c.options = map[string]*ui.Clickable{}
	}
	click := clickableOf(c.options, p.Slug)
	if click.Clicked(gtx) {
		s.setProfile(c, p.Slug)
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
				layout.Rigid(func(gtx C) D { return profileAvatar(gtx, on, c.book.look(p.Slug), profileAvatarSel) }),
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

// profilesBlock is Profiles: the logins this machine keeps for the provider, a
// row each naming the model and effort its new chats start on, and the way to
// add one. It is global; a project's scope leaves it out.
func (s *settingsScreen) profilesBlock(gtx C, th *ui.Theme, c *profileSettings, values scopedValues, m providersMachine, first bool) D {
	r := s.providerRow(gtx, settingEntry{titleKey: "settings.settingsProfiles.profilesTitle"}, values)
	r.description = profileSentence(c.provider, c.provider.words.profilesDescKey)
	cols := s.profilePrefColumns(c, m)
	for _, col := range cols {
		if names := overriders(settingEntry{key: col.key, scope: scopeOverridable}, s.projects, values); len(names) > 0 {
			r.kind, r.overriddenIn = rowOverridden, names
		}
	}
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D { return s.profileRows(gtx, th, c, m, cols) })
}

// profilePrefColumn is a chat preference a profile's Model and effort sets.
type profilePrefColumn struct {
	category, key, titleKey string
	choices                 []api.ChatChoice
}

// profilePrefColumns are the model and, once a chat offered one, the effort,
// for a provider whose chats have them.
func (s *settingsScreen) profilePrefColumns(c *profileSettings, m providersMachine) []profilePrefColumn {
	kind := c.provider.kind
	if !hasChatPrefs(kind) {
		return nil
	}
	var cols []profilePrefColumn
	for _, col := range []profilePrefColumn{
		{category: api.OptionModel, titleKey: "settings.chatPrefSetting.modelTitle"},
		{category: api.OptionEffort, titleKey: "settings.chatPrefSetting.effortTitle"},
	} {
		col.key = providerPrefKey(kind, col.category)
		col.choices = chatPrefChoices(m.globals[providerChoicesKey(kind, col.category)])
		if chatPrefShown(col.category, col.choices, m.globals[col.key]) {
			cols = append(cols, col)
		}
	}
	return cols
}

func (s *settingsScreen) profileRows(gtx C, th *ui.Theme, c *profileSettings, m providersMachine, cols []profilePrefColumn) D {
	list, loaded := c.book.snapshot()
	if !loaded {
		return th.Text(ui.TextXS, i18n.T("settings.settingsProfilesView.loading")).Muted().Layout(gtx)
	}
	var children []layout.FlexChild
	for i, p := range list {
		children = append(children, layout.Rigid(func(gtx C) D { return s.profileListRow(gtx, th, c, m, cols, p, i == 0) }))
	}
	if c.add.Clicked(gtx) {
		s.startNewProfile(c)
	}
	add := th.Button(&c.add, i18n.T("settings.settingsProfilesView.newProfile"))
	add.Variant, add.Size, add.Icon = ui.VariantGhost, ui.SizeSM, icProfileNew
	children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return layout.W.Layout(gtx, add.Layout) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// profileListRow is one profile: its mark and name, the account it is logged
// in to and the model and effort its chats start on, or the amber word that it
// is not, and its menu's ⋯ under the pointer.
func (s *settingsScreen) profileListRow(gtx C, th *ui.Theme, c *profileSettings, m providersMachine, cols []profilePrefColumn, p backend.Profile, first bool) D {
	st := c.row(p.Slug)
	if st.login.Clicked(gtx) {
		s.startLogin(c, p, nil)
	}
	hovered := st.hover.Update(gtx.Source)
	row := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return profileAvatar(gtx, th, c.book.look(p.Slug), profileAvatarRow) }),
			ui.Gap(3),
			layout.Flexed(1, func(gtx C) D { return profileText(gtx, th, c.provider, p, profilePrefSummary(c, m, cols, p.Slug)) }),
			ui.Gap(2),
		}
		if !p.LoggedIn {
			in := th.Button(&st.login, i18n.T("settings.settingsProfilesView.logIn"))
			in.Variant, in.Size = ui.VariantOutline, ui.SizeSM
			children = append(children, layout.Rigid(in.Layout))
		}
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D {
			if !hovered && !st.menu.Opened() {
				side := gtx.Dp(ui.Space(8))
				return D{Size: image.Pt(side, side)}
			}
			return s.profileMenu(gtx, th, c, m, len(cols) > 0, p, st)
		}))
		d := layout.Inset{Top: profileRowPadY, Bottom: profileRowPadY}.Layout(gtx,
			func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...) })
		// Over the row, passing the pointer on: under it, the ⋯ would take the
		// hover away from the row that draws it, every frame.
		area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		st.hover.Add(gtx.Ops)
		pass.Pop()
		area.Pop()
		return d
	}
	return row(gtx)
}

// profilePrefSummary is the model and effort a profile's chats start on, as
// its row names them; a value left to the provider is not named.
func profilePrefSummary(c *profileSettings, m providersMachine, cols []profilePrefColumn, slug string) string {
	var parts []string
	for _, col := range cols {
		if v := c.pref(col.key, slug, m); v != "" {
			parts = append(parts, chatPrefLabel(col.choices, v, c.provider.label))
		}
	}
	return strings.Join(parts, " ")
}

// profileText is a row's name and the line under it, prefs ending it.
func profileText(gtx C, th *ui.Theme, prov profileProvider, p backend.Profile, prefs string) D {
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, p.Name).Weight(font.Medium).Layout)}
	if p.Personal {
		name = append(name, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.settingsProfilesView.defaultTag")).Muted().Layout))
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, name...) }),
		layout.Rigid(func(gtx C) D { return profileStatusLine(gtx, th, prov, p, prefs) }),
	}
	if len(p.Copied) > 0 {
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
			return th.Text(ui.TextXS, profileSentence(prov, prov.words.copiedNoteKey)).Muted().Wrapped().Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func profileStatusLine(gtx C, th *ui.Theme, prov profileProvider, p backend.Profile, prefs string) D {
	switch {
	case p.LoggedIn:
		line := profileAccountLine(prov, p)
		if prefs != "" {
			line += profileAccountSplit + prefs
		}
		return th.Text(ui.TextXS, line).Muted().Layout(gtx)
	case p.StatusError != "":
		return th.Text(ui.TextXS, p.StatusError).In(th.Destructive).Layout(gtx)
	}
	return th.Text(ui.TextXS, i18n.T("settings.profiles.notLoggedIn")).In(th.ToneWait).Layout(gtx)
}

// profileMenu is a row's ⋯: the model and effort where chats have them,
// logging in again, bringing plugins over and renaming for every profile,
// deleting for every one but Personal.
func (s *settingsScreen) profileMenu(gtx C, th *ui.Theme, c *profileSettings, m providersMachine, prefs bool, p backend.Profile, st *profileRowState) D {
	var entries []ui.MenuEntry
	if prefs {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.settingsProfilesView.prefs"), Icon: icProfilePrefs, OnClick: func() { s.startPrefs(c, p, m) }})
	}
	entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.settingsProfilesView.logInAgain"), Icon: icProfileLogIn, OnClick: func() { s.startLogin(c, p, nil) }})
	if c.provider.plugins {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.settingsProfilesView.copyFrom"), Icon: icProfileCopy, Disabled: !p.LoggedIn, OnClick: func() { s.startCopy(c, p) }})
	}
	entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.settingsProfilesView.rename"), Icon: icProfileRename, OnClick: func() { s.startRename(c, p) }})
	entries = append(entries, s.profileLookEntries(c, p)...)
	if !p.Personal {
		entries = append(entries, ui.MenuEntry{Kind: ui.MenuSeparator},
			ui.MenuEntry{Label: i18n.T("settings.settingsProfilesView.delete"), Icon: icProfileDelete, Variant: ui.VariantDestructive, OnClick: func() { s.startDelete(c, p) }})
	}
	hint := i18n.T("settings.settingsProfilesView.options", i18n.Args{"name": p.Name})
	return th.DropdownMenu(s.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &st.tip, hint).Layout(gtx, func(gtx C) D {
			btn := th.Button(&st.more, "")
			btn.Icon, btn.Variant, btn.Size = icProfileMore, ui.VariantGhost, ui.SizeIconSM
			return btn.Layout(gtx)
		})
	})
}

// profilePlanBlock is Plan usage for the profile the open scope uses: Personal
// is the machine's own reading, any other profile is asked for by itself.
func (s *settingsScreen) profilePlanBlock(gtx C, th *ui.Theme, c *profileSettings, values scopedValues, slug string, plan backend.QuotaPlan, first bool) D {
	title := i18n.T("settings.planUsageSetting.title") + profileAccountSplit + c.book.nameOf(slug)
	return s.planBlock(gtx, th, values, plan, first, title)
}

// profilePlan is the reading of the profile slug, the one the pane watches.
func (s *settingsScreen) profilePlan(c *profileSettings, slug string) (backend.QuotaPlan, bool) {
	if s.footer == nil {
		return backend.QuotaPlan{}, false
	}
	return planFor(s.footer.quota.plans(profileQuotaKey(c.provider, slug)), c.provider.kind)
}
