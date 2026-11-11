package main

import driver "github.com/lichdotdev/gio-driver"

// basicsID is the first-run tour, the one the window opens by itself.
const basicsID = "basics"

// The widgets the Basics tour points at.
const (
	tourOpen     = "open"
	tourNewEmpty = "newEmpty"
	tourStage    = "stage"
	tourSession  = "session"
	tourNew      = "new"
	tourMenu     = "menu"
)

var basicsTour = []tourStep{
	{anchor: tourOpen, side: driver.Bottom, until: (*window).tourProjectOpen,
		text: tourText{"tour.basics.open.title", "tour.basics.open.body", "tour.basics.open.hint"}},
	{anchor: tourNewEmpty, side: driver.Bottom, until: (*window).tourSessionOpen,
		text:  tourText{"tour.basics.start.title", "tour.basics.start.body", "tour.basics.start.hint"},
		shell: &tourText{"tour.basics.startShell.title", "tour.basics.startShell.body", "tour.basics.startShell.hint"}},
	{anchor: tourStage, side: driver.Right,
		text:  tourText{"tour.basics.stage.title", "tour.basics.stage.body", ""},
		shell: &tourText{"tour.basics.stageShell.title", "tour.basics.stageShell.body", ""}},
	{anchor: tourSession, side: driver.Right, text: tourText{"tour.basics.session.title", "tour.basics.session.body", ""}},
	{anchor: tourNew, side: driver.Right, text: tourText{"tour.basics.worktree.title", "tour.basics.worktree.body", ""}},
	{anchor: tourMenu, side: driver.Bottom, text: tourText{"tour.basics.menu.title", "tour.basics.menu.body", ""}},
}

func init() {
	registerGuide(&guide{id: basicsID, label: "tabs.mainMenu.basics", steps: basicsTour})
}
