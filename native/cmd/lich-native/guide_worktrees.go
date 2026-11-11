package main

import driver "github.com/lichdotdev/gio-driver"

const worktreesID = "worktrees"

// The Worktrees guide's anchors: the New Session menu's Worktree item, the
// worktree dialog's name, base and Create, and the footer's checkout chip.
const (
	worktreesItem   = "worktrees.item"
	worktreesName   = "worktrees.name"
	worktreesFrom   = "worktrees.from"
	worktreesCreate = "worktrees.create"
	worktreesBranch = "worktrees.branch"
)

var worktreesTour = []tourStep{
	{anchor: tourNew, side: driver.Right, until: (*window).tourNewMenuOpen,
		text: tourText{"tourWorktrees.open.title", "tourWorktrees.open.body", "tourWorktrees.open.hint"}},
	{anchor: worktreesItem, side: driver.Right, layer: 1, until: (*window).tourWorktreeDialogOpen,
		text: tourText{"tourWorktrees.pick.title", "tourWorktrees.pick.body", "tourWorktrees.pick.hint"}},
	{anchor: worktreesName, side: driver.Left, layer: 1,
		text: tourText{"tourWorktrees.name.title", "tourWorktrees.name.body", ""}},
	{anchor: worktreesFrom, side: driver.Left, layer: 1,
		text: tourText{"tourWorktrees.from.title", "tourWorktrees.from.body", ""}},
	{anchor: worktreesCreate, side: driver.Bottom, layer: 1, until: (*window).tourInWorktree,
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

func (u *window) tourNewMenuOpen() bool { return u.sidebar.header.newMenu.Opened() }

func (u *window) tourWorktreeDialogOpen() bool { return u.worktree.open }
