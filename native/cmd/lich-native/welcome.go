package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/grimoire"
	"github.com/lichdotdev/lich/native/ui"
)

type welcomeDecision uint8

const (
	// welcomeWait is detection not having answered yet, not a bare machine.
	welcomeWait welcomeDecision = iota
	welcomeSkip
	welcomeShow
)

// decideWelcome decides whether the Welcome opens by itself: the first launch,
// which no default provider was chosen in. It preselects the first provider
// installed, "" when there is none.
func decideWelcome(m providersMachine) (welcomeDecision, string) {
	if !m.loaded || len(m.list) == 0 {
		return welcomeWait, ""
	}
	if m.globals[providerDefaultKey] != "" {
		return welcomeSkip, ""
	}
	for _, p := range m.list {
		if p.installed {
			return welcomeShow, p.id
		}
	}
	return welcomeShow, ""
}

// welcome is the first screen of the first launch, and the menu's Welcome
// after it: every provider, installed and signed in from one place.
type welcome struct {
	// decided latches the first launch's decision, and firstRun is that it
	// opened the Welcome: closing it then stores the default. installed is
	// the shape, once a provider was found.
	decided, firstRun, installed bool
	open                         bool
	dialog                       ui.DialogState
	cont                         ui.Clickable
	tomes                        map[string]*tome
	// main is the default provider the screen last showed, the book with the
	// flame; mainSeen that it showed one at all.
	main     string
	mainSeen bool
	// shot pins providers to a state or a journey's frame, for a screenshot.
	shot map[string]tomeShot
}

func (w *welcome) tome(id string) *tome {
	if w.tomes == nil {
		w.tomes = map[string]*tome{}
	}
	if w.tomes[id] == nil {
		w.tomes[id] = new(tome)
	}
	return w.tomes[id]
}

// decide opens the Welcome on the first launch, once detection answered. The
// first time a provider is installed, at the decision or later, it switches
// the absent providers off and the preselected one on, as the dialog it
// replaces did, so the launch menus list what is there.
func (u *window) decideWelcome() {
	w, s := &u.welcome, u.settings
	if !w.decided {
		decision, _ := decideWelcome(s.providers.snapshot())
		if decision == welcomeWait {
			return
		}
		w.decided, w.firstRun, w.open = true, decision == welcomeShow, decision == welcomeShow
	}
	if !w.firstRun || w.installed {
		return
	}
	_, preselect := decideWelcome(s.providers.snapshot())
	if preselect == "" {
		return
	}
	w.installed = true
	for _, p := range s.providers.snapshot().list {
		if !p.installed && p.enabled {
			s.setProviderEnabled(p.id, false)
		}
	}
	s.setProviderEnabled(preselect, true)
}

// bindWelcome switches on a provider the Welcome installed, so the launch
// menus list it.
func (u *window) bindWelcome() {
	u.model.providerInstalled = func(id string) { u.settings.setProviderEnabled(id, true) }
}

// reopenWelcome is the menu's Welcome. The login states are read again: a
// login finished in a terminal since shows.
func (u *window) reopenWelcome() {
	u.welcome.open = true
	go u.loadProfiles()
}

// closeWelcome is Continue, Escape and a login that needs the screen. On the
// first launch it stores the default provider; the menu's Welcome leaves the
// one chosen alone.
func (u *window) closeWelcome() {
	w, s := &u.welcome, u.settings
	w.open = false
	m := s.providers.snapshot()
	if !w.firstRun || m.globals[providerDefaultKey] != "" {
		return
	}
	id := w.main
	if id == "" {
		id = resolveProjectDefaultProvider(m.launchList(), "", "")
	}
	s.writeProviderKey(providerDefaultKey, "", id, s.providersChanged)
}

// welcomeMain is the book that shows as the default: the stored default
// while it is ready, else the first ready provider, "" while none is.
func welcomeMain(stored string, ready []string) string {
	if slices.Contains(ready, stored) {
		return stored
	}
	if len(ready) > 0 {
		return ready[0]
	}
	return ""
}

// chooseMain makes id the default new sessions start with: the setting
// Settings › Providers writes, shown at once.
func (u *window) chooseMain(id string) {
	s := u.settings
	s.writeProviderKey(providerDefaultKey, "", id, s.providersChanged)
}

// welcomeFacts reads one provider's facts off the window's stores.
func (u *window) welcomeFacts(p providerInfo) tomeFacts {
	f := tomeFacts{installed: p.installed, install: u.model.installs.of(p.id)}
	c := u.settings.providers.profiles(p.id)
	if c == nil || c.book == nil {
		return f
	}
	f.needsLogin = true
	profile, known := u.welcomeProfile(c)
	f.known, f.loggedIn = known, profile.LoggedIn
	d := c.flow.view()
	f.signingIn = d.open && d.kind == flowLogin && d.step != stepResult && d.target.Slug == profile.Slug
	return f
}

// welcomeProfile is the profile the provider's new sessions are born in, the
// one its Sign in logs in, and whether the book has read who is logged in.
func (u *window) welcomeProfile(c *profileSettings) (backend.Profile, bool) {
	slug := effectiveProfile(u.model.providers.snapshot().projectProfile(c.provider.kind, "").global)
	_, loaded := c.book.snapshot()
	if p, ok := c.book.find(slug); ok {
		return p, loaded
	}
	return backend.Profile{Slug: slug, Name: c.book.nameOf(slug)}, loaded
}

// welcomeAction is a click on a provider's button: install it, retry the
// install, open its install page where lich has none, or sign in.
func (u *window) welcomeAction(p providerInfo, s tomeState) {
	switch s {
	case tomeNotInstalled, tomeFailed:
		if !p.installable {
			u.settings.openExternal(p.docs)
			return
		}
		u.model.installProvider(u.ctx, u.client, p.id)
	case tomeSignedOut:
		c := u.settings.providers.profiles(p.id)
		profile, _ := u.welcomeProfile(c)
		if c.provider.terminalLogin {
			// The login runs in a shell card, which needs the screen.
			u.closeWelcome()
		}
		u.settings.startLogin(c, profile, nil)
	}
}

// signInBlocked reports a signed-out provider that cannot sign in from here:
// a terminal login needs a project to open its card in.
func (u *window) signInBlocked(p providerInfo) bool {
	c := u.settings.providers.profiles(p.id)
	if c == nil || !c.provider.terminalLogin {
		return false
	}
	_, ok := u.model.project()
	return !ok
}

// welcomeProviders are the providers the Welcome lists: every one with a book.
func welcomeProviders(m providersMachine) []providerInfo {
	var list []providerInfo
	for _, p := range m.list {
		if grimoire.Has(p.id) {
			list = append(list, p)
		}
	}
	return list
}
