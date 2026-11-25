package main

import (
	"log"
	"sync"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
	vcsToolsKey   = "vcs.tools" // no backend key: the row only reads the machine
	vcsAccountKey = "vcs.account"
	vcsGitBin     = "git"
	vcsGhBin      = "gh"
	vcsGlabBin    = "glab"
	vcsGitLabel   = "git"
	vcsGhLabel    = "GitHub CLI"
	vcsGlabLabel  = "GitLab CLI"
	vcsGitURL     = "https://git-scm.com/downloads"
	vcsGhURL      = "https://cli.github.com"
	vcsGlabURL    = "https://gitlab.com/gitlab-org/cli"
)

var (
	icVcsGit      = icons.Brand("git")
	icVcsGh       = icGitHub
	icVcsGlab     = icGitLab
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
	vcsGit  = vcsTool{vcsGitBin, vcsGitLabel, vcsGitURL, "env.vcsTools.gitWithout", icVcsGit}
	vcsGh   = vcsTool{vcsGhBin, vcsGhLabel, vcsGhURL, "env.vcsTools.ghWithout", icVcsGh}
	vcsGlab = vcsTool{vcsGlabBin, vcsGlabLabel, vcsGlabURL, "env.vcsTools.glabWithout", icVcsGlab}
	// vcsTools are the rows of the tools block, in order.
	vcsTools = []vcsTool{vcsGit, vcsGh, vcsGlab}
)

// vcsSettings is Settings › Version Control's reads, each nil until it
// answers, and its controls. The reads are kept across visits and
// revalidated under the drawn answer when Settings opens again.
type vcsSettings struct {
	// pathOf is an open project's checkout; onAccount makes the pull request
	// badges ask gh again once a project's account was written; gitLab is
	// whether a checkout is a GitLab one, which gh's accounts do not serve.
	pathOf    func(projectID string) string
	onAccount func()
	gitLab    func(path string) bool

	mu          sync.Mutex
	checks      map[string]backend.BinaryCheck
	accounts    []string // nil until gh answers; empty is signed in to none
	accountsErr string
	identities  askOnce[backend.CommitIdentity] // by path
	loaded      bool
	checking    bool
	recheckErr  string

	recheck ui.Clickable
	install map[string]*ui.Clickable
	picker  ui.DropdownPanelState
	options map[string]*ui.Clickable
	setup   vcsSetup
}

// revalidate has the next draw read everything again.
func (v *vcsSettings) revalidate() {
	v.mu.Lock()
	v.loaded = false
	v.identities.reset()
	v.mu.Unlock()
	v.setup.form.filled = false
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
	go v.readSetup(s)
}

// verifyTools asks where git, gh and glab resolve. A failed call leaves the tool
// unknown rather than missing: nothing answered.
func (v *vcsSettings) verifyTools(s *settingsScreen) {
	for _, t := range vcsTools {
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

// vcsToolsBlock is VcsToolsSetting: where git, gh and glab resolve, and the
// way to any one that does not.
func (s *settingsScreen) vcsToolsBlock(gtx C, th *ui.Theme, r settingRow, first bool) D {
	r.description = i18n.T("settings.vcsToolsSetting.description")
	v := &s.vcs
	if v.recheck.Clicked(gtx) {
		v.recheckTools(s)
	}
	under := func(gtx C, th *ui.Theme) D {
		var children []layout.FlexChild
		failed := false
		for i, t := range vcsTools {
			if i > 0 {
				children = append(children, layout.Rigid(th.Separator))
			}
			children = append(children, layout.Rigid(func(gtx C) D { return s.vcsToolRow(gtx, th, t) }))
			failed = failed || v.failed(t.bin)
		}
		if failure := v.setupFailure(); failure != "" {
			children = append(children, ui.VGap(1), layout.Rigid(th.Text(ui.TextXS, failure).In(th.Destructive).Layout))
		}
		if failed {
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
