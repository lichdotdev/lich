package main

import (
	"crypto/rand"
	"math"
	"runtime"
	"slices"
	"strings"

	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// paletteWorld is the open workspace as the palette reads it: the projects,
// every session of them with what its ring says, and how many sessions each
// project holds.
type paletteWorld struct {
	projects []paletteProject
	sessions []paletteSession
	counts   map[string]int
}

func (m *model) paletteWorld() paletteWorld {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := paletteWorld{counts: map[string]int{}, sessions: paletteSessionsOf(m.projects)}
	for _, p := range m.projects {
		w.projects = append(w.projects, paletteProject{ID: p.ID, Name: p.Name, Path: p.Path})
		w.counts[p.ID] = len(p.Sessions)
	}
	for i := range w.sessions {
		s := &w.sessions[i]
		s.status, s.unread, s.agent = m.status.status(s.sessionID), m.status.unread(s.sessionID), m.agent[s.sessionID]
	}
	return w
}

// projectOpen reports whether a project is open in the strip.
func (m *model) projectOpen(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.ContainsFunc(m.projects, func(p backend.Project) bool { return p.ID == id })
}

// turnsAmong is the status of each session that holds a turn.
func turnsAmong(sessions []paletteSession) map[string]string {
	turns := map[string]string{}
	for _, s := range sessions {
		if isRunning(s.status) {
			turns[s.sessionID] = s.status
		}
	}
	return turns
}

// hotkeyPalette is the chord's toggle: the same one closes it again.
func (u *window) hotkeyPalette(C) {
	p := u.palette
	if p.open {
		p.hide()
		return
	}
	p.show(turnsAmong(u.model.paletteWorld().sessions))
}

// paletteAvailable is whether this window has a screen or a command for an
// action; the rest are left out of the list rather than listed and refused.
func (u *window) paletteAvailable(id hotkeyID) bool {
	_, ok := hotkeyHandlers[id]
	return ok
}

// layoutPalette draws the palette and carries out its keys and clicks. Call it
// before the content under it, so it takes Escape ahead of any key filter
// there.
func (u *window) layoutPalette(gtx C, th *ui.Theme) {
	p := u.palette
	if !p.open && !p.dialog.Animating(gtx) {
		return
	}
	on := th.On(th.Popover)
	// The list fades out as the frame it closed on, rather than as a query
	// that found nothing.
	if p.open {
		p.last = u.paletteFrame(gtx, p)
	}
	v := p.last
	dlg := th.Dialog(&p.dialog, p.open, "", "")
	dlg.MaxWidth, dlg.Top, dlg.Scrim = paletteWidth, paletteTop, paletteScrim
	dlg.Raw = func(gtx C) D {
		if p.focus || !gtx.Focused(&p.query) {
			gtx.Execute(key.FocusCmd{Tag: &p.query})
			p.focus = false
		}
		return p.panel(gtx, on, &v, u.sidebar.statusGlyph, gtx.Now)
	}
	if dlg.Layout(gtx) && p.open {
		p.hide()
	}
	p.wake(gtx)
}

// paletteFrame is one frame of an open palette: its keys, its searches and the
// list they make, with the clicks on the list's rows and tabs.
func (u *window) paletteFrame(gtx C, p *commandPalette) paletteView {
	for _, tab := range paletteTabs {
		if p.tabClick(tab).Clicked(gtx) {
			p.tab = tab
		}
	}
	enter := p.keys(gtx)
	w := u.model.paletteWorld()
	query := p.query.Text()
	_, actions := actionTerm(query)
	all := rankSessions(w.sessions, p.turns)
	p.ask(gtx, u.client, query, !actions, p.tab == tabHistory, all)
	v := p.view(w, u.settings.hotkeys.current(), runtime.GOOS == "darwin", u.paletteAvailable)
	p.total = len(v.rows)
	if query != p.typed || p.tab != p.typedTab {
		p.typed, p.typedTab, p.cursor, p.reveal = query, p.tab, 0, true
	}
	if len(p.rows) < len(v.rows) {
		p.rows = make([]widget.Clickable, len(v.rows))
	}
	for i := range v.rows {
		click := &p.rows[i]
		switch {
		case click.Clicked(gtx):
			p.cursor = i
			u.runPaletteRow(v.rows[i])
			return v
		case click.Hovered() && p.hovered != i:
			p.cursor = i
		}
		if click.Hovered() {
			p.hovered = i
		}
	}
	if enter && len(v.rows) > 0 {
		u.runPaletteRow(v.rows[p.active(len(v.rows))])
	}
	return v
}

// runPaletteRow does what Enter or a click does with a row. A resume navigates
// on its own, so the palette closes with it; forgetting does not, because a
// workspace with one stale row usually has several.
func (u *window) runPaletteRow(r paletteRow) {
	p := u.palette
	switch r.kind {
	case rowSession, rowMessage:
		id, projectID := r.session.sessionID, r.session.projectID
		if r.kind == rowMessage {
			id, projectID = r.message.sessionID, r.message.projectID
		}
		beside := p.beside
		p.hide()
		u.settings.hide()
		if beside != nil {
			u.besidePicked(*beside, id)
			return
		}
		u.openSession(projectID, id)
	case rowProject:
		p.hide()
		u.settings.hide()
		u.showProject(r.project.ID)
	case rowClosed:
		p.hide()
		u.settings.hide()
		go u.reopenProject(r.project)
	case rowHistory:
		if historyAction(r.history) == historyForget {
			u.forgetParked(r.history)
			return
		}
		p.hide()
		u.settings.hide()
		go u.resumeParked(r.history.ClosedSession)
	case rowExternal:
		p.hide()
		u.settings.hide()
		go u.adoptExternal(r.external)
	case rowAction:
		p.picked = append(p.picked, r.action)
		p.hide()
	}
}

// forgetParked drops a parked row from the list in place and from the store.
func (u *window) forgetParked(h paletteHistory) {
	u.palette.dropped[h.ID] = true
	u.model.invalidate()
	go func() {
		if err := u.client.ForgetSession(u.ctx, h.ID); err != nil {
			u.logErr("forget session", err)
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("palette.commandPalette.forgetFailed", i18n.Args{"label": h.Label, "error": backend.ErrorText(err)})})
		}
	}()
}

// resumeParked resumes a session picked out of the history, wherever it was
// parked. The project comes back first when its tab is gone, and a cancelled
// relocate stops the resume rather than reopening the session into nowhere. The
// backend's own resume runs it afterwards, in the profile that holds the
// conversation.
func (u *window) resumeParked(c backend.ClosedSession) {
	if !u.model.projectOpen(c.ProjectID) {
		u.reopenProject(backend.RecentProject{ID: c.ProjectID, Name: c.ProjectName, Path: c.ProjectPath})
		if !u.model.projectOpen(c.ProjectID) {
			return
		}
	}
	restored, err := u.client.ReopenSession(u.ctx, c.ID, strings.ToLower(rand.Text()))
	switch {
	case err != nil:
		u.logErr("reopen session", err)
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("palette.commandPalette.resumeFailed", i18n.Args{"label": c.Label, "error": backend.ErrorText(err)})})
		return
	case restored == nil:
		// Another window resumed it, or its worktree was removed since the
		// list was drawn: not an error, but the row vanishing unexplained
		// reads as one.
		u.toaster.Show(ui.Toast{Title: i18n.T("shell.projects.noLongerAvailable", i18n.Args{"label": c.Label})})
		return
	}
	if _, ok := u.model.restoreSession(c.ProjectID, *restored, math.MaxInt); !ok {
		return
	}
	u.openSession(c.ProjectID, restored.ID)
}

// adoptExternal files a conversation started outside lich as a parked session
// and resumes it through the history's own door, which reopens a closed
// project first.
func (u *window) adoptExternal(e backend.ExternalSession) {
	id := strings.ToLower(rand.Text())
	label := externalLabel(e)
	if err := u.client.AdoptExternalSession(u.ctx, e.ProjectID, id, e.Kind, e.Path, e.ProviderSessionID, label); err != nil {
		u.logErr("adopt external session", err)
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("palette.commandPalette.resumeFailed", i18n.Args{"label": label, "error": backend.ErrorText(err)})})
		return
	}
	u.resumeParked(adoptedSession(e, id))
}

// pickedHotkeys are the actions the palette's Enter chose, once it has closed:
// a declined one is said rather than dropped, since a chord that does nothing
// falls through to the terminal but a row that does nothing just looks broken.
func (u *window) pickedHotkeys() []hotkeyID {
	picked := u.palette.picked
	u.palette.picked = nil
	var run []hotkeyID
	for _, id := range picked {
		h := hotkeyHandlers[id]
		if h.ready == nil || h.ready(u) {
			run = append(run, id)
			continue
		}
		u.toaster.Show(ui.Toast{Title: i18n.T("palette.commandPalette.actionUnavailable", i18n.Args{"label": hotkeyLabel(id)})})
	}
	return run
}

// paletteShot opens the palette at start with the query and tab the -palette
// flags name, for -shot.
func (u *window) paletteShot() {
	if !*flagPalette {
		return
	}
	u.palette.show(turnsAmong(u.model.paletteWorld().sessions))
	u.palette.query.SetText(*flagPaletteQuery)
	if tab := paletteTab(*flagPaletteTab); slices.Contains(paletteTabs, tab) {
		u.palette.tab = tab
	}
}
