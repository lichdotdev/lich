package main

import (
	"context"
	"log"

	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icContinueIn = icons.Lucide("arrow-right-left")
)

// continueTarget is a profile a card's conversation can move to, with the
// reading of its five-hour window.
type continueTarget struct {
	slug, name string
	look       profileLook
	usage      string
	limited    bool
}

// continueRequest is the card menu's Continue in pick.
type continueRequest struct{ id, slug string }

// profileBook is the book of the provider kind names, nil for a provider with
// no profiles.
func (m *model) profileBook(kind string) *profileBook {
	switch kind {
	case claudeKind:
		return m.profiles
	case codexKind:
		return m.codexProfiles
	case ompKind:
		return m.ompProfiles
	}
	return m.xdgProfiles[kind]
}

// continueTargets are the other logged-in profiles of a session of a provider
// with profiles, none while there is no other to move to.
func (m *model) continueTargets(kind, profile string) []continueTarget {
	book := m.profileBook(kind)
	if book == nil || book.provider.noContinue {
		return nil
	}
	logged := book.loggedIn()
	if len(logged) < 2 {
		return nil
	}
	current := effectiveProfile(profile)
	var out []continueTarget
	for _, p := range logged {
		if p.Slug == current {
			continue
		}
		plan, ok := planFor(m.quota.plans(profileQuotaKey(book.provider, p.Slug)), kind)
		usage, limited := profileUsage(plan, ok)
		out = append(out, continueTarget{slug: p.Slug, name: p.Name, look: book.look(p.Slug), usage: usage, limited: limited})
	}
	return out
}

// profileNameOf is the profile the exit toast names for a session's resume:
// Personal is the default and says nothing.
func (m *model) profileNameOf(kind, profile string) string {
	book := m.profileBook(kind)
	if book == nil || effectiveProfile(profile) == personalProfile {
		return ""
	}
	return book.nameOf(profile)
}

// continueInItem is the card menu's Continue in: each other login with its
// five-hour usage, a spent one in the destructive color and still clickable,
// since moving to it is what the user may want while it resets.
func continueInItem(th *ui.Theme, sv *sessionView, act *sidebarAction) ui.MenuEntry {
	sub := []ui.MenuEntry{{Kind: ui.MenuLabel, Label: i18n.T("sidebar.sidebarProfile.continueInHint")}}
	for _, t := range sv.continueIn {
		sub = append(sub, ui.MenuEntry{
			Label: t.name, Shortcut: t.usage, ShortcutDestructive: t.limited,
			Lead:    func(gtx C, size unit.Dp) D { return profileAvatar(gtx, th, t.look, size) },
			OnClick: func() { act.continueIn = continueRequest{id: sv.ID, slug: t.slug} },
		})
	}
	return ui.MenuEntry{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.sidebarProfile.continueIn"), Icon: icContinueIn, Sub: sub}
}

// applyContinueIn moves the conversation of the card the menu was opened on,
// and says why when the backend refuses.
func (u *window) applyContinueIn(act sidebarAction) {
	req := act.continueIn
	if req.id == "" {
		return
	}
	go func() {
		if err := u.client.ContinueIn(u.ctx, req.id, req.slug); err != nil {
			u.toaster.Show(failure("%v", err))
			return
		}
		u.terms.running(req.id)
		u.terms.resync(u.ctx, req.id)
	}()
}

// resync tells a session's PTY the size of the grid again: a process spawned
// by the backend itself starts at its default size.
func (ts *terminals) resync(ctx context.Context, id string) {
	ts.mu.Lock()
	t := ts.byID[id]
	ts.mu.Unlock()
	if t == nil {
		return
	}
	cols, rows := t.vt.Size()
	if err := ts.client.Resize(ctx, id, cols, rows); err != nil {
		log.Printf("resize %s: %v", id, err)
	}
}

// loadProfiles reads the profiles once the window is up.
func (u *window) loadProfiles() {
	u.logErr("claude profiles", u.model.profiles.reload(u.ctx))
	u.logErr("codex profiles", u.model.codexProfiles.reload(u.ctx))
	u.logErr("omp profiles", u.model.ompProfiles.reload(u.ctx))
	for kind, book := range u.model.xdgProfiles {
		u.logErr(kind+" profiles", book.reload(u.ctx))
	}
}

// openLoginTerminal runs a profile's login command in a shell card of the
// project on screen and takes Settings out of the way. It reports false when
// there is no project to open the card in.
func (u *window) openLoginTerminal(command string) bool {
	if _, ok := u.model.project(); !ok {
		return false
	}
	u.settings.hide()
	go u.newSession(launchRequest{kind: "shell"}, command+"\n")
	return true
}

// resumeProfileLine tells a restored card which profile its conversation lives
// in, "" for Personal, the one nothing needs saying about.
func (m *model) resumeProfileLine(s lichclient.Session) string {
	book := m.profileBook(s.Kind)
	if book == nil || effectiveProfile(s.Profile) == personalProfile {
		return ""
	}
	p, ok := book.find(s.Profile)
	if !ok || p.Email == "" {
		return i18n.T("sidebar.sidebarProfile.resumesInNoEmail", i18n.Args{"profile": book.nameOf(s.Profile)})
	}
	return i18n.T("sidebar.sidebarProfile.resumesIn", i18n.Args{"profile": p.Name, "email": p.Email})
}

// profileGone offers a new session in the project's own profile for a card
// whose profile was deleted: the card's row keeps its mark, so it could never
// start again.
func (u *window) profileGone(s sessionView) {
	u.toaster.Show(ui.Toast{
		ID: "profile-gone-" + s.ID, Variant: ui.ToastWarning, Duration: ui.ToastForever,
		Title:       i18n.T("sidebar.sidebarProfile.goneTitle", i18n.Args{"label": s.Label}),
		Description: i18n.T("sidebar.sidebarProfile.goneDescription"),
		Action:      &ui.ToastButton{Label: i18n.T("sidebar.sidebarProfile.startNew"), OnClick: func() { go u.startNewInProjectProfile(s) }},
		Cancel:      &ui.ToastButton{Label: i18n.T("sidebar.sidebarProfile.dismiss")},
	})
}

// startNewInProjectProfile opens a session of the card's kind where it ran,
// born in whatever profile the project uses now, and parks the card it
// replaces.
func (u *window) startNewInProjectProfile(s sessionView) {
	u.newSession(launchRequest{kind: s.Kind, dir: s.path, folder: s.Folder}, "")
	u.closeSession(s.ID)
}
