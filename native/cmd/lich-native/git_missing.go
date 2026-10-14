package main

import (
	"flag"
	"log"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (shell.gitMissingGate).
const (
	gitMissingTitle   = "git is not installed"
	gitMissingDesc    = "lich reads every branch, diff and worktree through git. Sessions still run without it — the version control surfaces stay empty."
	gitMissingNotNow  = "Not now"
	gitMissingInstall = "Install git"
)

var flagGitMissing = flag.Bool("git-missing", false, "open the git-missing dialog at start, for -shot")

// gitMissing is GitMissingGate: asked once at start, dismissed for this run
// only, so a machine still without git asks again next launch.
type gitMissing struct {
	missing, checking atomic.Bool
	dialog            ui.DialogState
	notNow, again     widget.Clickable
	install           widget.Clickable
}

// checkGit asks the backend whether git resolves; with refresh it re-reads
// the login shell's $PATH first, as Check again does.
func (u *window) checkGit(refresh bool) {
	g := &u.gitMissing
	g.checking.Store(true)
	defer func() {
		g.checking.Store(false)
		u.win.Invalidate()
	}()
	if refresh {
		if err := u.client.RefreshPath(u.ctx); err != nil {
			log.Printf("git check: refresh path: %v", err)
			return
		}
	}
	check, err := u.client.Verify(u.ctx, vcsGitBin)
	if err != nil {
		log.Printf("git check: %v", err)
		return
	}
	g.missing.Store(check.Failed())
}

func (u *window) layoutGitMissing(gtx C, th *ui.Theme) {
	g := &u.gitMissing
	if g.notNow.Clicked(gtx) {
		g.missing.Store(false)
	}
	if g.again.Clicked(gtx) && !g.checking.Load() {
		go u.checkGit(true)
	}
	if g.install.Clicked(gtx) {
		g.missing.Store(false)
		go func() { u.logErr("install git", u.client.OpenExternal(u.ctx, vcsGitURL)) }()
	}
	on := th.On(th.Popover)
	notNow := on.Button(&g.notNow, gitMissingNotNow)
	notNow.Variant = ui.VariantGhost
	label := vcsCheckAgain
	if g.checking.Load() {
		label = vcsChecking
	}
	again := on.Button(&g.again, label)
	again.Variant, again.Disabled = ui.VariantOutline, g.checking.Load()
	d := th.Dialog(&g.dialog, g.missing.Load(), gitMissingTitle, gitMissingDesc)
	d.MaxWidth = optInWidth
	d.HideClose = true
	d.Actions = []layout.Widget{notNow.Layout, again.Layout, on.Button(&g.install, gitMissingInstall).Layout}
	if d.Layout(gtx) {
		g.missing.Store(false)
	}
}
