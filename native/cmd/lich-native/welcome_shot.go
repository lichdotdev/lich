package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/grimoire"
)

var flagWelcomeShot = flag.String("welcome-shot", "", "open the Welcome at start, for -shot: open, or id=state[:arg] pairs (notinstalled, installing:0.6, failed, signedout, signingin, ready, bind:N, bindawake:N, unseal:N)")

// tomeShot pins a provider to a state, or to one frame of a journey, for a
// screenshot: -welcome-shot "kiro=installing:0.6,codex=unseal:4".
type tomeShot struct {
	state    tomeState
	progress float64
	failure  string
	journey  func(string) []grimoire.Grid
	frame    int
}

var shotStates = map[string]tomeState{
	"notinstalled": tomeNotInstalled, "installing": tomeInstalling, "failed": tomeFailed,
	"signedout": tomeSignedOut, "signingin": tomeSigningIn, "ready": tomeReady,
}

// shotJourneys are the journeys a shot can stop, with the state each shows
// its labels in.
var shotJourneys = map[string]struct {
	frames func(string) []grimoire.Grid
	state  tomeState
}{
	"bind":      {grimoire.Bind, tomeSignedOut},
	"bindawake": {grimoire.BindAwake, tomeReady},
	"unseal":    {grimoire.Unseal, tomeReady},
}

// shotFailure is the line a failed install shows in a shot.
const shotFailure = "curl: (6) Could not resolve host: downloads.claude.ai"

// parseWelcomeShot reads -welcome-shot: "open" opens the Welcome on the
// stores as they are, and id=state[:arg] pins a provider.
func parseWelcomeShot(spec string) (map[string]tomeShot, error) {
	shots := map[string]tomeShot{}
	for entry := range strings.SplitSeq(spec, ",") {
		if entry == "open" || entry == "" {
			continue
		}
		id, value, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("welcome shot %q: want id=state", entry)
		}
		name, arg, _ := strings.Cut(value, ":")
		shot, err := parseTomeShot(name, arg)
		if err != nil {
			return nil, fmt.Errorf("welcome shot %q: %w", entry, err)
		}
		shots[id] = shot
	}
	return shots, nil
}

func parseTomeShot(name, arg string) (tomeShot, error) {
	if s, ok := shotStates[name]; ok {
		shot := tomeShot{state: s}
		if s == tomeFailed {
			shot.failure = shotFailure
		}
		if arg != "" {
			p, err := strconv.ParseFloat(arg, 64)
			shot.progress = p
			return shot, err
		}
		return shot, nil
	}
	j, ok := shotJourneys[name]
	if !ok {
		return tomeShot{}, fmt.Errorf("no state or journey %q", name)
	}
	frame, err := strconv.Atoi(arg)
	return tomeShot{state: j.state, journey: j.frames, frame: frame}, err
}

// apply is what the pinned provider draws: its state, its install, and the
// journey's frame when the shot stops one.
func (s tomeShot) apply(id string) (tomeState, installStep, grimoire.Grid, bool) {
	step := installStep{running: s.state == tomeInstalling, progress: s.progress, failure: s.failure}
	if s.journey == nil {
		return s.state, step, nil, false
	}
	frames := s.journey(id)
	return s.state, step, frames[min(max(s.frame, 0), len(frames)-1)], true
}

// welcomeShot opens the Welcome as -welcome-shot asks.
func (u *window) welcomeShot(spec string) error {
	if spec == "" {
		return nil
	}
	shots, err := parseWelcomeShot(spec)
	if err != nil {
		return err
	}
	u.welcome.shot, u.welcome.decided, u.welcome.open = shots, true, true
	return nil
}
