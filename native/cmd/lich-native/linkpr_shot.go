package main

import (
	"flag"
	"image"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

var flagLinkPRShot = flag.String("link-pr-shot", "", "for -shot: panel opens Link pull request… for the active session, linked links #1214 to it")

// linkPRShotAt is where -link-pr-shot opens the panel, in window pixels: by
// the sidebar's cards, where the menu row that opens it stands.
var linkPRShotAt = image.Pt(300, 160)

func (u *window) linkPRShot() {
	sv, ok := u.model.activeSession()
	if !ok {
		return
	}
	switch *flagLinkPRShot {
	case "panel":
		u.sidebar.linkPR.open(sv)
		u.sidebar.linkPR.panel.OpenAt(linkPRShotAt)
	case "linked":
		url := "https://github.com/lichdotdev/lich/pull/1214"
		pr := &backend.PullRequest{Number: 1214, URL: url, Title: "Fix the downloads race", State: "OPEN"}
		u.model.setSessionPRs([]backend.SessionPullRequest{{Session: sv.ID, URL: url, Linked: true, Forge: api.ForgeGitHub, Here: true, PR: pr}})
	}
}
