package main

import "github.com/lichdotdev/lich/native/backend"

// sidebarAccount is the sidebar's account row: the provider in focus, its
// profiles when it has any logged in (book nil for a provider with none), and
// the plan reading of the account its new sessions spend.
type sidebarAccount struct {
	kind    string
	profile profileFocus
	// key is the planQuota key of that reading, and readable false when lich
	// reads none for the shown profile: a provider with no per-profile reading
	// is read for the machine's login, Personal, alone.
	key      string
	readable bool
	plan     backend.QuotaPlan
	loaded   bool
}

// switches is whether the provider has logins to switch between.
func (a sidebarAccount) switches() bool {
	return a.profile.book != nil && len(a.profile.book.loggedIn()) > 1
}

// focusedAccount is the row's account before its reading: false with no
// project, or for a provider with profiles and none logged in.
func (m *model) focusedAccount() (sidebarAccount, bool) {
	p, ok := m.project()
	if !ok {
		return sidebarAccount{}, false
	}
	a := sidebarAccount{kind: m.focusedKind(), readable: true}
	book := m.profileBook(a.kind)
	if book == nil {
		return a, true
	}
	if len(book.loggedIn()) == 0 {
		return sidebarAccount{}, false
	}
	a.profile = profileFocus{kind: a.kind, book: book, project: p, choice: m.providers.projectProfile(a.kind, p.ID)}
	a.key = profileQuotaKey(book.provider, a.profile.choice.shown)
	a.readable = book.provider.quotaPrefix != "" || effectiveProfile(a.profile.choice.shown) == personalProfile
	return a, true
}

// account is the row to draw: one while there are logins to switch between or
// a plan reading to show. It keeps that reading watched while the row may
// show it, since with one login nothing else follows it.
func (pf *profileFooter) account(m *model) (sidebarAccount, bool) {
	a, ok := m.focusedAccount()
	pf.follow(m.quota, a.key, ok && a.readable)
	if !ok {
		return a, false
	}
	if a.readable {
		a.plan, a.loaded = planFor(m.quota.plans(a.key), a.kind)
	}
	return a, a.shows()
}

// shows is whether the row has anything to offer: logins to switch between or
// a plan reading.
func (a sidebarAccount) shows() bool {
	return a.switches() || (a.loaded && a.plan.Status == "ok")
}

// follow watches key's reading while on, and lets the last one go.
func (pf *profileFooter) follow(q *planQuota, key string, on bool) {
	if on && pf.unwatch != nil && key == pf.watching {
		return
	}
	if pf.unwatch != nil {
		pf.unwatch()
		pf.unwatch = nil
	}
	if on {
		pf.watching, pf.unwatch = key, q.watch(key)
	}
}
