package main

import driver "github.com/lichdotdev/gio-driver"

const worktreesID = "worktrees"

// worktreesBranch is the footer's checkout chip.
const worktreesBranch = "worktrees.branch"

var worktreesTour = []tourStep{
	{anchor: tourNew, side: driver.Right, until: (*window).tourInWorktree,
		text: tourText{"tourWorktrees.create.title", "tourWorktrees.create.body", "tourWorktrees.create.hint"}},
	{anchor: tourSession, side: driver.Right, text: tourText{"tourWorktrees.card.title", "tourWorktrees.card.body", ""}},
	{anchor: worktreesBranch, side: driver.Top, text: tourText{"tourWorktrees.branch.title", "tourWorktrees.branch.body", ""}},
}

func init() {
	registerGuide(&guide{id: worktreesID, order: 10, label: "tourWorktrees.label", steps: worktreesTour,
		ready: (*window).tourInRepository, unavailable: "tourWorktrees.unavailable"})
}

func (u *window) tourInRepository() bool {
	p, ok := u.model.project()
	if !ok {
		return false
	}
	g, isGit := u.model.gitStatus(p.Path)
	return footerInput{git: g, isGit: isGit}.inRepository()
}

// tourInWorktree is whether the session on screen runs in a worktree: only a
// worktree session has a Path of its own.
func (u *window) tourInWorktree() bool {
	s, ok := u.model.activeSession()
	return ok && s.Path != ""
}
