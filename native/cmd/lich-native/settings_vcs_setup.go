package main

import (
	"image"
	"log"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The commands that sign the forge CLIs in, run in a terminal where they ask
// what they need.
const (
	vcsGhSignIn   = "gh auth login"
	vcsGlabSignIn = "glab auth login"
)

// vcsSetup is what the tools block offers past where each tool resolves: the
// package manager's command for a missing one, glab's accounts and the
// global commit identity, each read with the rest of the section. The
// guarded fields are under vcsSettings.mu.
type vcsSetup struct {
	installs     map[string]string // by binary; "" when no package manager has it
	versions     map[string]string // by binary; "" until it answered or when it did not
	glabAccounts []string          // nil until glab answered
	global       *backend.CommitIdentity
	failure      string // why the last terminal or save did not happen

	// openTerminal runs a command in a new shell card and takes Settings out
	// of the way, false with no project to open it in.
	openTerminal func(command string) bool
	run, site    map[string]*ui.Clickable
	more         map[string]*vcsMore
	form         identityForm
}

// vcsMore is a tool's ⋯ and its menu, and the tooltip naming where it
// resolves.
type vcsMore struct {
	button ui.Clickable
	menu   ui.MenuState
	tip    ui.Tooltip
}

// identityForm is the global identity's name and email and their Save.
type identityForm struct {
	name, email widget.Editor
	save        ui.Clickable
	saving      bool
	filled      bool // the fields hold the read identity, or the user's text
	open        bool // Edit identity opened it over an identity git has
}

// readSetup reads the install commands, glab's accounts and the global
// identity. A failed read leaves its part unknown, drawn as before.
func (v *vcsSettings) readSetup(s *settingsScreen) {
	installs, versions := map[string]string{}, map[string]string{}
	for _, t := range vcsTools {
		cmd, err := s.client.ToolInstall(s.ctx, t.bin)
		if err != nil {
			log.Printf("settings: install command for %s: %v", t.bin, err)
		}
		installs[t.bin] = cmd
		// A missing tool fails here; its row says so without a version.
		versions[t.bin], _ = s.client.ProviderVersion(s.ctx, t.bin)
	}
	glab, err := s.client.GitLabAccounts(s.ctx)
	if err != nil {
		log.Printf("settings: glab accounts: %v", err)
		glab = nil
	}
	global, gerr := s.client.GlobalCommitIdentity(s.ctx)
	v.mu.Lock()
	v.setup.installs, v.setup.versions, v.setup.glabAccounts, v.setup.global = installs, versions, glab, nil
	if gerr == nil {
		v.setup.global = &global
	}
	v.mu.Unlock()
	s.invalidate()
}

// runInTerminal opens command in a shell card, or says why it could not.
func (v *vcsSettings) runInTerminal(s *settingsScreen, command string) {
	failure := ""
	if v.setup.openTerminal == nil || !v.setup.openTerminal(command) {
		failure = i18n.T("settings.settingsProfiles.noTerminal")
	}
	v.mu.Lock()
	v.setup.failure = failure
	v.mu.Unlock()
	s.invalidate()
}

// saveIdentity writes the form's name and email as the global identity and
// reads it back.
func (v *vcsSettings) saveIdentity(s *settingsScreen) {
	f := &v.setup.form
	name, email := strings.TrimSpace(f.name.Text()), strings.TrimSpace(f.email.Text())
	v.mu.Lock()
	f.saving = true
	v.mu.Unlock()
	go func() {
		err := s.client.SetGlobalCommitIdentity(s.ctx, name, email)
		v.mu.Lock()
		f.saving, v.setup.failure = false, ""
		if err != nil {
			v.setup.failure = backend.ErrorText(err)
		} else {
			v.setup.global = &backend.CommitIdentity{Name: name, Email: email}
			f.open = false
			v.identities.reset()
		}
		v.mu.Unlock()
		s.invalidate()
	}()
}

// vcsTone is how loud a row's detail line is.
type vcsTone uint8

const (
	vcsToneMuted   vcsTone = iota
	vcsToneSetUp           // something to configure: ToneWait
	vcsToneMissing         // the tool is missing: Destructive
)

// vcsToolState is what a tool's row says and offers.
type vcsToolState struct {
	known    bool // where it resolves has been answered
	gone     bool
	ready    bool   // installed, and signed in or with an identity
	version  string // what --version printed, "" before it answered
	detail   string // the second line
	tone     vcsTone
	install  string // the package manager's command for a missing tool
	signIn   string // the command that signs it in
	identity bool   // git without a global name or email
	resolved string // where it resolves, for the name's tooltip
}

// toolState reads t's row out of the section's answers.
func (v *vcsSettings) toolState(t vcsTool) vcsToolState {
	c, known := v.check(t.bin)
	v.mu.Lock()
	defer v.mu.Unlock()
	st := vcsToolState{known: known, resolved: c.Path, version: v.setup.versions[t.bin]}
	switch {
	case !known:
		return st
	case c.Failed():
		st.gone, st.tone, st.install = true, vcsToneMissing, v.setup.installs[t.bin]
		st.detail = i18n.T("settings.vcsToolsSetting.notInstalled") + " · " + i18n.T(t.without)
		return st
	}
	switch t.bin {
	case vcsGitBin:
		v.gitState(&st)
	case vcsGhBin:
		if v.accounts != nil || v.accountsErr != "" {
			accountState(&st, v.accounts, vcsGhSignIn)
		}
	case vcsGlabBin:
		if v.setup.glabAccounts != nil {
			accountState(&st, v.setup.glabAccounts, vcsGlabSignIn)
		}
	}
	return st
}

// gitState is git's detail: who commits land as, or the form's prompt.
func (v *vcsSettings) gitState(st *vcsToolState) {
	g := v.setup.global
	switch {
	case g == nil:
	case g.Name == "" || g.Email == "":
		st.detail, st.tone, st.identity = i18n.T("settings.vcsToolsSetting.noIdentity"), vcsToneSetUp, true
	default:
		st.detail, st.ready = i18n.T("settings.vcsToolsSetting.commitsAs", i18n.Args{"name": g.Name, "email": g.Email}), true
	}
}

// accountState is a forge CLI's detail: the first account it is signed in
// to, or No account and the command that signs one in.
func accountState(st *vcsToolState, accounts []string, signIn string) {
	st.signIn = signIn
	if len(accounts) == 0 {
		st.detail, st.tone = i18n.T("settings.vcsToolsSetting.noAccount"), vcsToneSetUp
		return
	}
	host, login := splitAccount(accounts[0])
	st.detail, st.ready = i18n.T("settings.vcsToolsSetting.signedInAs", i18n.Args{"login": login, "host": host}), true
}

var (
	vcsTileSize = ui.Space(9)    // the mark's square, 36px
	vcsMarkSize = ui.Space(4.75) // the mark in it, 19px
	vcsRowGap   = ui.Space(3.5)
	// vcsUnderInset lines what goes under a row up with its text, past the
	// tile and its gap.
	vcsUnderInset = vcsTileSize + vcsRowGap
	icVcsReady    = icons.Lucide("check")
)

// vcsToolRow is a tool's row: its mark in a square, its name and version over
// what it lacks or who it runs as, and at the right the action that fixes
// what it lacks, or a check and the ⋯ of a ready one; under it, the identity
// form and what Install runs.
func (s *settingsScreen) vcsToolRow(gtx C, th *ui.Theme, t vcsTool) D {
	v := &s.vcs
	st := v.toolState(t)
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return s.vcsToolLine(gtx, th, t, st) })}
	if st.identity || st.ready && t.bin == vcsGitBin && v.setup.form.open {
		children = append(children, layout.Rigid(func(gtx C) D { return s.identityFields(gtx, th) }))
	}
	if st.gone && st.install != "" {
		hint := i18n.T("settings.vcsToolsSetting.runsInTerminal", i18n.Args{"command": st.install})
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: vcsUnderInset, Bottom: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, hint).Muted().Layout)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *settingsScreen) vcsToolLine(gtx C, th *ui.Theme, t vcsTool, st vcsToolState) D {
	v := &s.vcs
	if v.setup.run == nil {
		v.setup.run, v.setup.site, v.setup.more = map[string]*ui.Clickable{}, map[string]*ui.Clickable{}, map[string]*vcsMore{}
	}
	run, site := clickableOf(v.setup.run, t.bin), clickableOf(v.setup.site, t.bin)
	m := v.setup.more[t.bin]
	if m == nil {
		m = new(vcsMore)
		v.setup.more[t.bin] = m
	}
	if command := st.install + st.signIn; run.Clicked(gtx) && command != "" {
		v.runInTerminal(s, command)
	}
	if site.Clicked(gtx) {
		s.openExternal(t.url)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: ui.Space(3), Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return vcsTile(gtx, th, t) }),
			layout.Rigid(layout.Spacer{Width: vcsRowGap}.Layout),
			layout.Flexed(1, func(gtx C) D {
				if st.resolved == "" {
					return vcsToolText(gtx, th, t, st)
				}
				return th.Tooltip(s.root, &m.tip, st.resolved).Layout(gtx, func(gtx C) D { return vcsToolText(gtx, th, t, st) })
			}),
		}
		children = append(children, s.vcsToolActions(th, t, st, run, site, m)...)
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// vcsTile is the tool's mark on a Muted square.
func vcsTile(gtx C, th *ui.Theme, t vcsTool) D {
	size := gtx.Dp(vcsTileSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	ui.Fill(gtx, th.Over(th.Muted), ui.RadiusMD)
	return layout.Center.Layout(gtx, func(gtx C) D { return t.icon.Layout(gtx, vcsMarkSize, th.Foreground) })
}

// vcsToolText is the name and version over the detail, in its tone.
func vcsToolText(gtx C, th *ui.Theme, t vcsTool, st vcsToolState) D {
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, t.label).Weight(font.SemiBold).Layout)}
	if st.version != "" {
		name = append(name, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, st.version).Mono().Muted().Layout))
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, name...)
	})}
	if st.detail != "" {
		text := th.Text(ui.TextXS, st.detail).Muted().Truncated()
		switch st.tone {
		case vcsToneSetUp:
			text = text.In(th.ToneWait)
		case vcsToneMissing:
			text = text.In(th.Destructive)
		}
		children = append(children, ui.VGap(0.5), layout.Rigid(text.Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// vcsToolActions is a missing tool's Site and Install, a CLI with no account's
// Sign in, and a ready tool's check and ⋯.
func (s *settingsScreen) vcsToolActions(th *ui.Theme, t vcsTool, st vcsToolState, run, site *ui.Clickable, m *vcsMore) []layout.FlexChild {
	button := func(c *ui.Clickable, key string, variant ui.Variant) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			b := th.Button(c, i18n.T(key))
			b.Size, b.Variant = ui.SizeSM, variant
			return b.Layout(gtx)
		})
	}
	switch {
	case st.gone && st.install != "":
		return []layout.FlexChild{ui.Gap(2), button(site, "settings.vcsToolsSetting.website", ui.VariantGhost), ui.Gap(1), button(run, "settings.vcsToolsSetting.install", ui.VariantDefault)}
	case st.gone:
		return []layout.FlexChild{ui.Gap(2), button(site, "settings.vcsToolsSetting.install", ui.VariantDefault)}
	case !st.ready && st.signIn != "":
		return []layout.FlexChild{ui.Gap(2), button(run, "settings.vcsToolsSetting.signIn", ui.VariantDefault)}
	case st.ready:
		return []layout.FlexChild{
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return icVcsReady.Layout(gtx, ui.Space(4), th.TonePass) }),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return s.vcsMoreMenu(gtx, th, t, st, m) }),
		}
	}
	return nil
}

// vcsMoreMenu is a ready tool's ⋯: Edit identity for git, Sign in with
// another account for a forge CLI, and the tool's site.
func (s *settingsScreen) vcsMoreMenu(gtx C, th *ui.Theme, t vcsTool, st vcsToolState, m *vcsMore) D {
	v := &s.vcs
	var entries []ui.MenuEntry
	if t.bin == vcsGitBin {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.vcsToolsSetting.editIdentity"), OnClick: func() {
			v.setup.form.open, v.setup.form.filled = true, false
			s.invalidate()
		}})
	} else {
		command := st.signIn
		entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.vcsToolsSetting.otherAccount"), OnClick: func() { v.runInTerminal(s, command) }})
	}
	entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.vcsToolsSetting.openSite"), Icon: icVcsExternal, OnClick: func() { s.openExternal(t.url) }})
	return th.DropdownMenu(s.root, &m.menu, entries).Layout(gtx, func(gtx C) D {
		b := th.Button(&m.button, "")
		b.Icon, b.Variant, b.Size = icExtensionMore, ui.VariantGhost, ui.SizeIconXS
		return b.Layout(gtx)
	})
}

// identityFields is the global identity's Name and Email and Save, filled
// with whatever git already has, and where they are saved.
func (s *settingsScreen) identityFields(gtx C, th *ui.Theme) D {
	v := &s.vcs
	f := &v.setup.form
	v.mu.Lock()
	if g := v.setup.global; g != nil && !f.filled {
		f.name.SetText(g.Name)
		f.email.SetText(g.Email)
		f.filled = true
	}
	saving := f.saving
	v.mu.Unlock()
	f.name.SingleLine, f.email.SingleLine = true, true
	ready := strings.TrimSpace(f.name.Text()) != "" && strings.TrimSpace(f.email.Text()) != ""
	if f.save.Clicked(gtx) && ready && !saving {
		v.saveIdentity(s)
	}
	field := func(ed *widget.Editor, key string) layout.FlexChild {
		return layout.Flexed(1, th.Input(ed, i18n.T(key)).Layout)
	}
	return layout.Inset{Left: vcsUnderInset, Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					field(&f.name, "settings.vcsToolsSetting.name"), ui.Gap(2),
					field(&f.email, "settings.vcsToolsSetting.email"), ui.Gap(2),
					layout.Rigid(func(gtx C) D {
						b := th.Button(&f.save, i18n.T("settings.vcsToolsSetting.save"))
						b.Size, b.Disabled = ui.SizeSM, !ready || saving
						return b.Layout(gtx)
					}))
			}),
			ui.VGap(1.5),
			layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.vcsToolsSetting.savedGlobally")).Muted().Layout),
		)
	})
}

// setupFailure is why the last terminal or save did not happen, "" for none.
func (v *vcsSettings) setupFailure() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.setup.failure
}
