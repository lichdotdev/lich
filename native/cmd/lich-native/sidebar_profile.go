package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (claudeProfile.continueIn*).
const (
	continueInLabel = "Continue in"
	continueInHint  = "Same conversation, another login"
)

var (
	profileChipSize = unit.Sp(11) // text-[11px]
	profileChipMaxW = ui.Space(24)
	icContinueIn    = icons.Lucide("arrow-right-left")
)

// profileKeepFresh is how often the plan readings the Continue in rows quote
// are looked at; each is asked for again once the backend's cache is older.
const profileKeepFresh = time.Minute

// continueTarget is a profile a Claude Code card's conversation can move to,
// with the reading of its five-hour window.
type continueTarget struct {
	slug, name string
	usage      string
	limited    bool
}

// continueRequest is the card menu's Continue in pick.
type continueRequest struct{ id, slug string }

// continueTargets are the other logged-in profiles of a Claude Code session,
// none while there is no other to move to.
func (m *model) continueTargets(kind, profile string) []continueTarget {
	if kind != claudeKind {
		return nil
	}
	logged := m.profiles.loggedIn()
	if len(logged) < 2 {
		return nil
	}
	current := effectiveProfile(profile)
	var out []continueTarget
	for _, p := range logged {
		if p.Slug == current {
			continue
		}
		plan, ok := m.profiles.cachedPlan(p.Slug)
		usage, limited := profileUsage(plan, ok)
		out = append(out, continueTarget{slug: p.Slug, name: p.Name, usage: usage, limited: limited})
	}
	return out
}

// profileNameOf is the profile a card names beside its branch: Personal is the
// default and says nothing.
func (m *model) profileNameOf(kind, profile string) string {
	if kind != claudeKind || effectiveProfile(profile) == personalProfile {
		return ""
	}
	return m.profiles.nameOf(profile)
}

// continueInItem is the card menu's Continue in: each other login with its
// five-hour usage, a spent one in the destructive color and still clickable,
// since moving to it is what the user may want while it resets.
func continueInItem(th *ui.Theme, sv *sessionView, act *sidebarAction) ui.MenuEntry {
	sub := []ui.MenuEntry{{Kind: ui.MenuLabel, Label: continueInHint}}
	for _, t := range sv.continueIn {
		sub = append(sub, ui.MenuEntry{
			Label: t.name, Shortcut: t.usage, ShortcutDestructive: t.limited,
			Lead:    func(gtx C, size unit.Dp) D { return profileAvatar(gtx, th, t.name, t.slug, size) },
			OnClick: func() { act.continueIn = continueRequest{id: sv.ID, slug: t.slug} },
		})
	}
	return ui.MenuEntry{Kind: ui.MenuSubTrigger, Label: continueInLabel, Icon: icContinueIn, Sub: sub}
}

// profileChip is the outlined name of the profile a session runs in.
func profileChip(gtx C, th *ui.Theme, name string) D {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(profileChipMaxW))
	d := layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx,
		th.Text(profileChipSize, name).Muted().Layout)
	call := macro.Stop()
	insetBorder(gtx, d.Size, gtx.Dp(ui.RadiusSM), th.Over(th.Border))
	call.Add(gtx.Ops)
	return d
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

// cachedPlan is the last reading of a profile's plan without asking for one.
func (b *profileBook) cachedPlan(slug string) (plan lichclient.QuotaPlan, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.plans[slug]
	return r.plan, ok
}

// keepFresh reads the profiles and then keeps the readings of the logged-in
// ones current while there is more than one, the only case a reading is
// quoted anywhere but Settings.
func (b *profileBook) keepFresh(ctx context.Context) {
	if err := b.reload(ctx); err != nil {
		log.Printf("claude profiles: %v", err)
	}
	tick := time.NewTicker(profileKeepFresh)
	defer tick.Stop()
	for {
		if logged := b.loggedIn(); len(logged) > 1 {
			for _, p := range logged {
				b.plan(ctx, p.Slug)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
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

// resumeProfileLine tells a restored Claude Code card which profile its
// conversation lives in, "" for Personal, the one nothing needs saying about.
func (m *model) resumeProfileLine(s lichclient.Session) string {
	if s.Kind != claudeKind || effectiveProfile(s.Profile) == personalProfile {
		return ""
	}
	p, ok := m.profiles.find(s.Profile)
	if !ok || p.Email == "" {
		return fmt.Sprintf(resumeInProfileNoMail, m.profiles.nameOf(s.Profile))
	}
	return fmt.Sprintf(resumeInProfile, p.Name, p.Email)
}

// ponytail: English until the native window has i18n.
const (
	toastProfileGone     = "%s was started in a profile that no longer exists"
	toastProfileGoneDesc = "Its conversation cannot be resumed. Start a new session in this project's profile instead."
	toastProfileStartNew = "Start new"
	toastProfileDismiss  = "Dismiss"
)

// profileGone offers a new session in the project's own profile for a card
// whose profile was deleted: the card's row keeps its mark, so it could never
// start again.
func (u *window) profileGone(s sessionView) {
	u.toaster.Show(ui.Toast{
		ID: "profile-gone-" + s.ID, Variant: ui.ToastWarning, Duration: ui.ToastForever,
		Title:       fmt.Sprintf(toastProfileGone, s.Label),
		Description: toastProfileGoneDesc,
		Action:      &ui.ToastButton{Label: toastProfileStartNew, OnClick: func() { go u.startNewInProjectProfile(s) }},
		Cancel:      &ui.ToastButton{Label: toastProfileDismiss},
	})
}

// startNewInProjectProfile opens a session of the card's kind where it ran,
// born in whatever profile the project uses now, and parks the card it
// replaces.
func (u *window) startNewInProjectProfile(s sessionView) {
	u.newSession(launchRequest{kind: s.Kind, dir: s.path, folder: s.Folder}, "")
	u.closeSession(s.ID)
}
