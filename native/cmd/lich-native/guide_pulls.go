package main

import driver "github.com/lichdotdev/gio-driver"

const pullsGuideID = "pulls"

// The widgets the Pull requests guide points at.
const (
	tourPullsButton  = "pulls.button"
	tourPullsFilter  = "pulls.filter"
	tourPullsList    = "pulls.list"
	tourPullsSession = "pulls.session"
)

var pullsTour = []tourStep{
	{anchor: tourPullsButton, side: driver.Bottom, until: func(u *window) bool { return u.pulls.open },
		text: tourText{"tourPulls.open.title", "tourPulls.open.body", "tourPulls.open.hint"}},
	{anchor: tourPullsFilter, side: driver.Bottom, text: tourText{"tourPulls.filter.title", "tourPulls.filter.body", ""}},
	{anchor: tourPullsList, side: driver.Top, until: func(u *window) bool { return u.pulls.selected != 0 },
		text: tourText{"tourPulls.pick.title", "tourPulls.pick.body", "tourPulls.pick.hint"}},
	{anchor: tourPullsSession, side: driver.Bottom, text: tourText{"tourPulls.review.title", "tourPulls.review.body", ""}},
}

func init() {
	registerGuide(&guide{id: pullsGuideID, order: 30, label: "tourPulls.label", steps: pullsTour,
		ready: (*window).tourProjectOpen, unavailable: "tourPulls.unavailable"})
}
