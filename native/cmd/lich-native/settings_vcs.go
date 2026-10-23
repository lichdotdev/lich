package main

import (
	"log"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
	vcsToolsKey     = "vcs.tools" // no backend key: the row only reads the machine
	vcsAccountKey   = "vcs.account"
	vcsGitBin       = "git"
	vcsGhBin        = "gh"
	vcsGitLabel     = "git"
	vcsGhLabel      = "GitHub CLI (gh)"
	vcsGitURL       = "https://git-scm.com/downloads"
	vcsGhURL        = "https://cli.github.com"
	vcsWithoutInset = 6.5 // ml-6.5: under the label, past the icon and its gap
)

var (
	icVcsGit      = icons.Lucide("git-branch")
	icVcsGh       = icons.Lucide("git-pull-request-arrow")
	icVcsExternal = icons.Lucide("external-link")
	icVcsRecheck  = icons.Lucide("refresh-cw")
)

// vcsTool is the binary as lich spawns it, and the page a machine without it
// installs it from. without is the catalog key of what goes without it.
type vcsTool struct {
	bin, label, url, without string
	icon                     *icons.Icon
}

var (
	vcsGit = vcsTool{vcsGitBin, vcsGitLabel, vcsGitURL, "env.vcsTools.gitWithout", icVcsGit}
	vcsGh  = vcsTool{vcsGhBin, vcsGhLabel, vcsGhURL, "env.vcsTools.ghWithout", icVcsGh}
)

// vcsSettings is Settings › Version Control's reads, each nil until it
// answers, and its controls. The reads are kept across visits and
// revalidated under the drawn answer when Settings opens again.
type vcsSettings struct {
	// pathOf is an open project's checkout; onAccount makes the pull request
	// badges ask gh again once a project's account was written.
	pathOf    func(projectID string) string
	onAccount func()

	mu          sync.Mutex
	checks      map[string]backend.BinaryCheck
	accounts    []string // nil until gh answers; empty is signed in to none
	accountsErr string
	identities  askOnce[backend.CommitIdentity] // by path
	loaded      bool
	checking    bool
	recheckErr  string

	recheck widget.Clickable
	install map[string]*widget.Clickable
	picker  ui.DropdownPanelState
	options map[string]*widget.Clickable
}

// revalidate has the next draw read everything again.
func (v *vcsSettings) revalidate() {
	v.mu.Lock()
	v.loaded = false
	v.identities.reset()
	v.mu.Unlock()
}

// ensure starts the section's reads the first time it is drawn after a
// revalidate.
func (v *vcsSettings) ensure(s *settingsScreen) {
	v.mu.Lock()
	start := !v.loaded
	v.loaded = true
	v.mu.Unlock()
	if !start {
		return
	}
	go v.verifyTools(s)
	go v.listAccounts(s)
}

// verifyTools asks where git and gh resolve. A failed call leaves the tool
// unknown rather than missing: nothing answered.
func (v *vcsSettings) verifyTools(s *settingsScreen) {
	for _, t := range []vcsTool{vcsGit, vcsGh} {
		c, err := s.client.Verify(s.ctx, t.bin)
		if err != nil {
			log.Printf("settings: verify %s: %v", t.bin, err)
			continue
		}
		v.mu.Lock()
		if v.checks == nil {
			v.checks = map[string]backend.BinaryCheck{}
		}
		v.checks[t.bin] = c
		v.mu.Unlock()
	}
	s.invalidate()
}

func (v *vcsSettings) listAccounts(s *settingsScreen) {
	accounts, err := s.client.GitHubAccounts(s.ctx)
	v.mu.Lock()
	v.accountsErr = ""
	if err != nil {
		v.accountsErr = backend.ErrorText(err)
	} else {
		v.accounts = accounts
	}
	v.mu.Unlock()
	s.invalidate()
}

// check is bin's verdict, ok false until it answered.
func (v *vcsSettings) check(bin string) (backend.BinaryCheck, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.checks[bin]
	return c, ok
}

func (v *vcsSettings) failed(bin string) bool {
	c, ok := v.check(bin)
	return ok && c.Failed()
}

// recheckTools is CheckAgainButton: re-read the $PATH, then verify again.
func (v *vcsSettings) recheckTools(s *settingsScreen) {
	v.mu.Lock()
	v.checking, v.recheckErr = true, ""
	v.mu.Unlock()
	go func() {
		err := s.client.RefreshPath(s.ctx)
		if err == nil {
			v.verifyTools(s)
		}
		v.mu.Lock()
		v.checking = false
		if err != nil {
			log.Printf("settings: refresh PATH: %v", err)
			v.recheckErr = i18n.T("settings.settingsVcs.pathReread")
		}
		v.mu.Unlock()
		s.invalidate()
	}()
}

// vcsRow draws the Version Control rows, false for any other key.
func (s *settingsScreen) vcsRow(gtx C, th *ui.Theme, e settingEntry, r settingRow, first bool) (D, bool) {
	switch e.key {
	case vcsToolsKey:
		s.vcs.ensure(s)
		return s.vcsToolsBlock(gtx, th, r, first), true
	case vcsAccountKey:
		s.vcs.ensure(s)
		return s.vcsAccountBlock(gtx, th, r, first), true
	}
	return D{}, false
}

// vcsToolsBlock is VcsToolsSetting: where git and gh resolve, and the way to
// either one that does not.
func (s *settingsScreen) vcsToolsBlock(gtx C, th *ui.Theme, r settingRow, first bool) D {
	r.description = i18n.T("settings.vcsToolsSetting.description")
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	v := &s.vcs
	if v.recheck.Clicked(gtx) {
		v.recheckTools(s)
	}
	under := func(gtx C, th *ui.Theme) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return s.vcsToolRow(gtx, th, vcsGit) }),
			ui.VGap(1),
			layout.Rigid(func(gtx C) D { return s.vcsToolRow(gtx, th, vcsGh) }),
		}
		if v.failed(vcsGitBin) || v.failed(vcsGhBin) {
			children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: ui.Space(1)}.Layout(gtx, func(gtx C) D { return v.recheckButton(gtx, th) })
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return r.layoutBlock(gtx, th, first, under)
}

func (v *vcsSettings) recheckButton(gtx C, th *ui.Theme) D {
	v.mu.Lock()
	checking, failure := v.checking, v.recheckErr
	v.mu.Unlock()
	b := th.Button(&v.recheck, i18n.T("common.checkAgainButton.check"))
	b.Variant, b.Size, b.Icon, b.Disabled = ui.VariantGhost, ui.SizeSM, icVcsRecheck, checking
	if checking {
		b.Text = i18n.T("common.checkAgainButton.checking")
	}
	if failure == "" {
		return b.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(b.Layout),
		ui.VGap(1),
		layout.Rigid(th.Text(ui.TextXS, failure).In(th.Destructive).Layout),
	)
}

// vcsToolRow is ToolRow: the tool's icon and name, then its resolved path
// or, when missing, the verdict, its Install and what goes without it.
func (s *settingsScreen) vcsToolRow(gtx C, th *ui.Theme, t vcsTool) D {
	v := &s.vcs
	c, known := v.check(t.bin)
	gone := known && c.Failed()
	if v.install == nil {
		v.install = map[string]*widget.Clickable{}
	}
	install := clickableOf(v.install, t.bin)
	if install.Clicked(gtx) {
		s.openExternal(t.url)
	}
	verdict := th.Text(ui.TextXS, c.Path).Muted().Mono()
	if gone {
		verdict = th.Text(ui.TextXS, i18n.T("settings.vcsToolsSetting.notOnPath")).In(th.Destructive)
	}
	line := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return t.icon.Layout(gtx, ui.Space(4), th.MutedForeground) }),
			ui.Gap(2.5),
			layout.Rigid(th.Text(ui.TextSM, t.label).Layout),
			layout.Flexed(1, func(gtx C) D { return layout.E.Layout(gtx, verdict.Layout) }),
		}
		if gone {
			b := th.Button(install, i18n.T("settings.vcsToolsSetting.install"))
			b.Size, b.Icon = ui.SizeSM, icVcsExternal
			children = append(children, ui.Gap(2.5), layout.Rigid(b.Layout))
		}
		return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}
	if !gone {
		return line(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(line),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(vcsWithoutInset)}.Layout(gtx, th.Text(ui.TextXS, i18n.T(t.without)).Muted().Layout)
		}),
	)
}

func (s *settingsScreen) openExternal(url string) {
	go func() {
		if err := s.client.OpenExternal(s.ctx, url); err != nil {
			log.Printf("settings: open %s: %v", url, err)
		}
	}()
}

// projectPath is the checkout of the open project id, "" if none is.
func (m *model) projectPath(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projects {
		if p.ID == id {
			return p.Path
		}
	}
	return ""
}

// accountChanged has every pull request badge ask gh again, as the account
// that answers it just changed.
func (m *model) accountChanged() {
	m.mu.Lock()
	m.stalePullRequests()
	m.mu.Unlock()
}
