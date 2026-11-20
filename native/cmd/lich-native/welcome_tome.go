package main

import (
	"time"

	"github.com/lichdotdev/lich/native/grimoire"
	"github.com/lichdotdev/lich/native/ui"
)

// tomeState is where a provider is on the Welcome's journey.
type tomeState uint8

const (
	tomeNotInstalled tomeState = iota
	tomeInstalling
	tomeFailed
	tomeSignedOut
	tomeSigningIn
	tomeReady
)

// welcomeJourneyStep is how long a frame of a journey holds (the mockup's 85ms).
const welcomeJourneyStep = 85 * time.Millisecond

// tomeFacts is what the stores say about one provider, which its state is
// read from.
type tomeFacts struct {
	installed bool
	install   installStep
	// needsLogin is a provider whose login lich reads; known is that reading
	// in, and loggedIn its answer. signingIn is a login lich runs for it now.
	needsLogin, known, loggedIn, signingIn bool
}

func tomeStateOf(f tomeFacts) tomeState {
	switch {
	case f.install.running:
		return tomeInstalling
	case !f.installed && f.install.failure != "":
		return tomeFailed
	case !f.installed:
		return tomeNotInstalled
	case !f.needsLogin || f.loggedIn:
		return tomeReady
	case f.signingIn:
		return tomeSigningIn
	}
	return tomeSignedOut
}

func (s tomeState) installed() bool { return s >= tomeSignedOut }

// journeyBetween is what plays when a provider moves from one state to
// another: the binding once it installed, the clasps opening once it signed
// in, nothing for any other move.
func journeyBetween(id string, from, to tomeState) []grimoire.Grid {
	switch {
	case !from.installed() && to == tomeReady:
		return grimoire.BindAwake(id)
	case !from.installed() && to.installed():
		return grimoire.Bind(id)
	case from.installed() && from != tomeReady && to == tomeReady:
		return grimoire.Unseal(id)
	}
	return nil
}

// restSprite is what a provider shows while nothing plays.
func restSprite(id string, s tomeState, progress float64) grimoire.Grid {
	switch s {
	case tomeInstalling:
		return grimoire.Box(grimoire.Written(id, progress))
	case tomeSignedOut, tomeSigningIn:
		return grimoire.Sprite(id, grimoire.SignedOut)
	case tomeReady:
		return grimoire.Sprite(id, grimoire.Ready)
	}
	return grimoire.Box(grimoire.Sprite(id, grimoire.NotInstalled))
}

// tome is one provider's cell as the Welcome last drew it.
type tome struct {
	action ui.Clickable
	// row is the whole cell, a click on a ready book making it the default.
	row     ui.Clickable
	tip     ui.Tooltip
	seen    bool
	last    tomeState
	known   bool
	journey []grimoire.Grid
	since   time.Time
	step    time.Duration
}

// play starts frames at now, a frame every step.
func (t *tome) play(frames []grimoire.Grid, step time.Duration, now time.Time) {
	t.journey, t.since, t.step = frames, now, step
}

// observe takes the state the stores say now: a move seen from a known state
// starts its journey at now. The first look, and a move from a login not yet
// read, only take the state.
func (t *tome) observe(id string, s tomeState, known bool, now time.Time) {
	if t.seen && t.known && s != t.last {
		if j := journeyBetween(id, t.last, s); j != nil {
			t.play(j, welcomeJourneyStep, now)
		}
	}
	t.seen, t.last, t.known = true, s, known
}

// frame is the journey's frame at now and when the next starts; ok is false
// once it is over.
func (t *tome) frame(now time.Time) (g grimoire.Grid, next time.Time, ok bool) {
	if t.journey == nil {
		return nil, time.Time{}, false
	}
	i := int(now.Sub(t.since) / t.step)
	if i >= len(t.journey) {
		t.journey = nil
		return nil, time.Time{}, false
	}
	return t.journey[max(i, 0)], t.since.Add(time.Duration(i+1) * t.step), true
}

// flameTick is the flame's frame at now, and when the next starts: one clock
// for the window, so a book chosen now joins the flame where it is.
func flameTick(now time.Time) (int, time.Time) {
	tick := now.UnixNano() / int64(grimoire.FlameStep)
	return int(tick), time.Unix(0, (tick+1)*int64(grimoire.FlameStep))
}

// observeMain plays the change of default: the old one's flame goes out, the
// new one's lights. A first look plays nothing.
func (w *welcome) observeMain(id string, now time.Time) {
	if w.mainSeen && id != w.main {
		tick, _ := flameTick(now)
		if w.main != "" {
			w.tome(w.main).play(grimoire.Snuff(w.main, tick), grimoire.SnuffStep, now)
		}
		if id != "" {
			w.tome(id).play(grimoire.Choose(id, tick), grimoire.ChooseStep, now)
		}
	}
	w.main, w.mainSeen = id, true
}
