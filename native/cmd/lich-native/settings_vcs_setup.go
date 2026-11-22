package main

import (
	"log"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
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
	glabAccounts []string          // nil until glab answered
	global       *backend.CommitIdentity
	failure      string // why the last terminal or save did not happen

	// openTerminal runs a command in a new shell card and takes Settings out
	// of the way, false with no project to open it in.
	openTerminal func(command string) bool
	run, site    map[string]*ui.Clickable
	form         identityForm
}

// identityForm is the global identity's name and email and their Save.
type identityForm struct {
	name, email widget.Editor
	save        ui.Clickable
	saving      bool
	filled      bool // the fields hold the read identity, or the user's text
}

// readSetup reads the install commands, glab's accounts and the global
// identity. A failed read leaves its part unknown, drawn as before.
func (v *vcsSettings) readSetup(s *settingsScreen) {
	installs := map[string]string{}
	for _, t := range vcsTools {
		cmd, err := s.client.ToolInstall(s.ctx, t.bin)
		if err != nil {
			log.Printf("settings: install command for %s: %v", t.bin, err)
		}
		installs[t.bin] = cmd
	}
	glab, err := s.client.GitLabAccounts(s.ctx)
	if err != nil {
		log.Printf("settings: glab accounts: %v", err)
		glab = nil
	}
	global, gerr := s.client.GlobalCommitIdentity(s.ctx)
	v.mu.Lock()
	v.setup.installs, v.setup.glabAccounts, v.setup.global = installs, glab, nil
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
			v.identities.reset()
		}
		v.mu.Unlock()
		s.invalidate()
	}()
}

// vcsToolState is what a tool's row says and offers.
type vcsToolState struct {
	gone     bool
	status   string // the line before the path, "" for none
	alarm    bool   // status is a problem
	install  string // the package manager's command for a missing tool
	signIn   string // the command that signs it in, while it has no account
	identity bool   // git without a global name or email
	resolved string // where it resolves
}

// toolState reads t's row out of the section's answers.
func (v *vcsSettings) toolState(t vcsTool) vcsToolState {
	c, known := v.check(t.bin)
	v.mu.Lock()
	defer v.mu.Unlock()
	st := vcsToolState{resolved: c.Path}
	if known && c.Failed() {
		st.gone, st.status, st.alarm, st.install = true, i18n.T("settings.vcsToolsSetting.notOnPath"), true, v.setup.installs[t.bin]
		return st
	}
	if !known {
		return st
	}
	switch t.bin {
	case vcsGitBin:
		if g := v.setup.global; g != nil {
			if g.Name == "" || g.Email == "" {
				st.status, st.alarm, st.identity = i18n.T("settings.vcsToolsSetting.noIdentity"), true, true
			} else {
				st.status = g.Name + " · " + g.Email
			}
		}
	case vcsGhBin:
		if v.accounts != nil || v.accountsErr != "" {
			st.status, st.signIn = accountStatus(v.accounts, vcsGhSignIn)
		}
	case vcsGlabBin:
		if v.setup.glabAccounts != nil {
			st.status, st.signIn = accountStatus(v.setup.glabAccounts, vcsGlabSignIn)
		}
	}
	st.alarm = st.alarm || st.signIn != ""
	return st
}

// accountStatus is the first account's login, or No account and the command
// that signs one in.
func accountStatus(accounts []string, signIn string) (status, command string) {
	if len(accounts) == 0 {
		return i18n.T("settings.vcsToolsSetting.noAccount"), signIn
	}
	_, login := splitAccount(accounts[0])
	return login, ""
}

// vcsToolRow is a tool's line: its mark and name, what it lacks or who it
// runs as, where it resolves, and the action that fixes what it lacks; under
// it, what that action runs and the identity form.
func (s *settingsScreen) vcsToolRow(gtx C, th *ui.Theme, t vcsTool) D {
	v := &s.vcs
	st := v.toolState(t)
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return s.vcsToolLine(gtx, th, t, st) })}
	var note string
	switch {
	case st.gone && st.install != "":
		note = i18n.T("settings.vcsToolsSetting.runsInTerminal", i18n.Args{"command": st.install}) + " " + i18n.T(t.without)
	case st.gone:
		note = i18n.T(t.without)
	case st.signIn != "":
		note = i18n.T("settings.vcsToolsSetting.runsInTerminal", i18n.Args{"command": st.signIn})
	}
	if st.identity {
		children = append(children, layout.Rigid(func(gtx C) D { return s.identityFields(gtx, th) }))
		note = i18n.T("settings.vcsToolsSetting.savedGlobally")
	}
	if note != "" {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(vcsWithoutInset)}.Layout(gtx, th.Text(ui.TextXS, note).Muted().Layout)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *settingsScreen) vcsToolLine(gtx C, th *ui.Theme, t vcsTool, st vcsToolState) D {
	v := &s.vcs
	if v.setup.run == nil {
		v.setup.run, v.setup.site = map[string]*ui.Clickable{}, map[string]*ui.Clickable{}
	}
	run, site := clickableOf(v.setup.run, t.bin), clickableOf(v.setup.site, t.bin)
	if command := st.install + st.signIn; run.Clicked(gtx) && command != "" {
		v.runInTerminal(s, command)
	}
	if site.Clicked(gtx) {
		s.openExternal(t.url)
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return t.icon.Layout(gtx, ui.Space(4), th.MutedForeground) }),
		ui.Gap(2.5),
		layout.Rigid(th.Text(ui.TextSM, t.label).Layout),
		layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
	}
	if st.status != "" {
		text := th.Text(ui.TextXS, st.status).Muted().Mono()
		if st.alarm {
			text = th.Text(ui.TextXS, st.status).In(th.Destructive)
		}
		children = append(children, layout.Rigid(text.Layout), ui.Gap(2.5))
	}
	children = append(children, s.vcsToolActions(th, t, st, run, site)...)
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// vcsToolActions is a missing tool's Install, in a terminal when a package
// manager has it and on its page otherwise, a signed-out CLI's Sign in, and
// the path of one that resolves.
func (s *settingsScreen) vcsToolActions(th *ui.Theme, t vcsTool, st vcsToolState, run, site *ui.Clickable) []layout.FlexChild {
	button := func(c *ui.Clickable, key string, variant ui.Variant) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			b := th.Button(c, i18n.T(key))
			b.Size, b.Variant = ui.SizeSM, variant
			if c == site {
				b.Icon = icVcsExternal
			}
			return b.Layout(gtx)
		})
	}
	switch {
	case st.gone && st.install != "":
		return []layout.FlexChild{button(run, "settings.vcsToolsSetting.install", ui.VariantDefault), ui.Gap(1), button(site, "settings.vcsToolsSetting.website", ui.VariantGhost)}
	case st.gone:
		return []layout.FlexChild{button(site, "settings.vcsToolsSetting.install", ui.VariantDefault)}
	}
	out := []layout.FlexChild{layout.Rigid(th.Text(ui.TextXS, st.resolved).Muted().Mono().Layout)}
	if st.signIn != "" {
		out = append([]layout.FlexChild{button(run, "settings.vcsToolsSetting.signIn", ui.VariantOutline), ui.Gap(2.5)}, out...)
	}
	return out
}

// identityFields is the global identity's Name and Email and Save, filled
// with whatever half git already has.
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
	return layout.Inset{Left: ui.Space(vcsWithoutInset), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			field(&f.name, "settings.vcsToolsSetting.name"), ui.Gap(2),
			field(&f.email, "settings.vcsToolsSetting.email"), ui.Gap(2),
			layout.Rigid(func(gtx C) D {
				b := th.Button(&f.save, i18n.T("settings.vcsToolsSetting.save"))
				b.Size, b.Disabled = ui.SizeSM, !ready || saving
				return b.Layout(gtx)
			}))
	})
}

// setupFailure is why the last terminal or save did not happen, "" for none.
func (v *vcsSettings) setupFailure() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.setup.failure
}
