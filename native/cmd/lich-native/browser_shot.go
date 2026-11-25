package main

import (
	"flag"
	"fmt"
)

var flagBrowserPick = flag.String("browser-pick", "", "for -shot: turn Select on in each browser pane on screen and point at x,y (CSS pixels) of its page, as x,y")

// shotPick does -browser-pick once for the pane.
func (u *window) shotPick(p *browserPaneUI, session string) {
	if *flagBrowserPick == "" || p.shotPicked {
		return
	}
	p.shotPicked = true
	var x, y float64
	if _, err := fmt.Sscanf(*flagBrowserPick, "%g,%g", &x, &y); err != nil {
		return
	}
	p.notes.setPicking(true)
	u.probeElement(p, session, x, y, false)
}
