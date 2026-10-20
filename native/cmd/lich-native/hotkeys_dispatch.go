package main

import (
	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/native/ui"
)

// A hotkey's chord is the window's, not the terminal's: the frame polls the
// bound chords before anything is laid out, so the focused terminal's own
// filters never see a key that is one. An action this window has no screen
// for, or one that declines right now, contributes no filter at all, and its
// chord reaches the PTY as if lich had never bound it.

// hotkeyHandler is an action this window can carry out. ready is asked
// before the chord is polled, so an action that declines costs the terminal
// nothing; run is the same path the action's button takes.
type hotkeyHandler struct {
	ready func(u *window) bool // nil: always
	run   func(u *window, gtx C)
}

// hotkeyHandlers are the actions with a native screen or command. Absent,
// and so inert: delegate (no delegate flow).
var hotkeyHandlers = map[hotkeyID]hotkeyHandler{
	hotkeySplitBeside:        {(*window).hasActiveSession, func(u *window, _ C) { u.pickBeside(edgeRight) }},
	hotkeySplitBelow:         {(*window).hasActiveSession, func(u *window, _ C) { u.pickBeside(edgeBottom) }},
	hotkeyOtherPane:          {(*window).hasLayout, func(u *window, _ C) { u.model.cyclePane(u.ctx, 1); u.model.invalidate() }},
	hotkeyZoomPane:           {(*window).hasLayout, func(u *window, _ C) { u.model.toggleZoom(); u.model.invalidate() }},
	hotkeyUnsplitPane:        {(*window).hasLayout, (*window).hotkeyUnsplit},
	hotkeyRotateSplit:        {(*window).hasLayout, func(u *window, _ C) { u.model.rotateFocused(); u.model.invalidate() }},
	hotkeyNewSession:         {(*window).canNewSession, (*window).hotkeyNewSession},
	hotkeyNewWorktree:        {(*window).hasProject, func(u *window, _ C) { u.showWorktreeDialog() }},
	hotkeyRenameSession:      {(*window).hasActiveSession, (*window).hotkeyRename},
	hotkeyCloseSession:       {(*window).canCloseActive, (*window).hotkeyClose},
	hotkeyTogglePin:          {(*window).hasActiveSession, (*window).hotkeyPin},
	hotkeyOpenTerminal:       {(*window).canOpenTerminal, (*window).hotkeyTerminal},
	hotkeyNextSession:        {(*window).hasProject, func(u *window, _ C) { u.hotkeyStep(1) }},
	hotkeyPrevSession:        {(*window).hasProject, func(u *window, _ C) { u.hotkeyStep(-1) }},
	hotkeyNextWaitingSession: {(*window).hasWaiting, (*window).hotkeyWaiting},
	hotkeyFocusTerminal:      {(*window).hasActiveSession, (*window).hotkeyFocusTerminal},
	hotkeyNextProject:        {func(u *window) bool { _, ok := u.neighborTab(1); return ok }, func(u *window, _ C) { u.hotkeyTab(1) }},
	hotkeyPrevProject:        {func(u *window) bool { _, ok := u.neighborTab(-1); return ok }, func(u *window, _ C) { u.hotkeyTab(-1) }},
	hotkeyToggleSidebar:      {nil, (*window).hotkeySidebar},
	hotkeyToggleDock:         {nil, (*window).hotkeyDock},
	hotkeyToggleFullscreen:   {nil, func(u *window, _ C) { u.toggleFullscreen() }},
	hotkeySearchInFiles:      {(*window).hasProject, (*window).hotkeySearch},
	hotkeySettings:           {(*window).hasProject, (*window).hotkeyOpenSettings},
	hotkeyPulls:              {(*window).hasProject, func(u *window, _ C) { u.togglePulls() }},
	hotkeyShortcuts:          {nil, (*window).hotkeyShortcuts},
	hotkeyCommandPalette:     {nil, (*window).hotkeyPalette},
	hotkeyQuit:               {nil, func(u *window, _ C) { go u.askOrQuit(closeAsk) }},
}

// pollHotkeys is pollChords over the actions this window can carry out now,
// then the ones the command palette picked.
func (u *window) pollHotkeys(gtx C) []hotkeyID {
	fired := pollChords(gtx, &u.settings.hotkeys, func(id hotkeyID) bool {
		handler, ok := hotkeyHandlers[id]
		return ok && (handler.ready == nil || handler.ready(u))
	})
	return append(fired, u.pickedHotkeys()...)
}

// pollChords answers the actions whose bound chord was pressed this frame, in
// the order they were. It reads nothing while a capture button records, so
// the chord being recorded is not also run, and it takes only the chords of
// the actions that are available: the rest are left for whoever asks next.
func pollChords(gtx C, h *hotkeyEditor, available func(hotkeyID) bool) []hotkeyID {
	if h.recording != "" {
		return nil
	}
	bound := h.current()
	var live []hotkeyID
	var filters []event.Filter
	for _, a := range hotkeyActions {
		if bound[a.id].Key == "" || !available(a.id) {
			continue
		}
		live = append(live, a.id)
		for _, f := range comboFilters(bound[a.id]) {
			filters = append(filters, f)
		}
	}
	var fired []hotkeyID
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return fired
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		state := keyStateOf(e)
		for _, id := range live {
			if matchesCombo(state, bound[id]) {
				fired = append(fired, id)
				break
			}
		}
	}
}

// runHotkeys carries the fired actions out after the frame is laid out, as a
// click's action is, so none of them changes what the frame already drew.
func (u *window) runHotkeys(gtx C, fired []hotkeyID) {
	for _, id := range fired {
		hotkeyHandlers[id].run(u, gtx)
	}
	if len(fired) > 0 {
		u.model.invalidate()
	}
}

func (u *window) hasProject() bool {
	_, ok := u.model.project()
	return ok
}

func (u *window) hasActiveSession() bool {
	_, ok := u.model.activeSession()
	return ok
}

func (u *window) canNewSession() bool {
	return u.hasProject() && u.model.newSessionKind() != ""
}

// canCloseActive is no ×, which is what the pin withholds: the shortcut
// withholds it too.
func (u *window) canCloseActive() bool {
	sv, ok := u.model.activeSession()
	return ok && !sv.Pinned
}

// canOpenTerminal: the card menu's Open in › Terminal is not offered on a
// shell.
func (u *window) canOpenTerminal() bool {
	sv, ok := u.model.activeSession()
	return ok && sv.Kind != "shell"
}

func (u *window) hotkeyNewSession(C) {
	u.settings.hide()
	go u.newSession(launchRequest{kind: u.model.newSessionKind()}, "")
}

// hotkeyRename opens the sidebar first when it is the rail, which carries no
// card to rename.
func (u *window) hotkeyRename(gtx C) {
	sv, ok := u.model.activeSession()
	if !ok {
		return
	}
	if !u.sidebar.open {
		u.setSidebarOpen(gtx, true)
	}
	rename, ok := u.sidebar.renames[sv.ID]
	if !ok {
		rename = new(ui.InlineEdit)
		u.sidebar.renames[sv.ID] = rename
	}
	rename.Start(sv.Label)
}

func (u *window) hotkeyClose(C) {
	if sv, ok := u.model.activeSession(); ok {
		u.closeSession(sv.ID)
	}
}

func (u *window) hotkeyPin(C) {
	if sv, ok := u.model.activeSession(); ok {
		u.apply(sidebarAction{pin: sv.ID, pinTo: !sv.Pinned})
	}
}

func (u *window) hotkeyTerminal(C) {
	if sv, ok := u.model.activeSession(); ok {
		u.settings.hide()
		u.apply(sidebarAction{terminalAt: sv.shown})
	}
}

// hotkeyCards are the cards in the order the sidebar draws them, the ones in
// a folded block marked hidden.
func (u *window) hotkeyCards() []hotkeyCard {
	var cards []hotkeyCard
	for _, b := range u.model.sidebarBlocks(u.sidebar.header.filter()).blocks {
		for _, s := range b.sessions {
			cards = append(cards, hotkeyCard{id: s.ID, shown: !b.folded})
		}
	}
	return cards
}

func (u *window) activeSessionID() string {
	sv, _ := u.model.activeSession()
	return sv.ID
}

// hotkeyStep is swallowed even with nowhere to go: the chord belongs to lich.
func (u *window) hotkeyStep(step int) {
	if target := nextShownCard(u.hotkeyCards(), u.activeSessionID(), step, func(string) bool { return true }); target != "" {
		u.apply(sidebarAction{activate: target})
	}
}

func (u *window) waitingTarget() string {
	return nextShownCard(u.hotkeyCards(), u.activeSessionID(), 1, u.model.isWaiting)
}

// hasWaiting declines the chord when no other session waits: there is no move
// this press could have meant.
func (u *window) hasWaiting() bool { return u.hasProject() && u.waitingTarget() != "" }

func (u *window) hotkeyWaiting(C) {
	if target := u.waitingTarget(); target != "" {
		u.apply(sidebarAction{activate: target})
	}
}

func (m *model) isWaiting(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phaseOfSession(id) == phaseWaiting
}

// hotkeyFocusTerminal leaves Settings, which renders over the terminals, and
// drops the focus so the active session's terminal takes it back this frame.
func (u *window) hotkeyFocusTerminal(gtx C) {
	u.settings.hide()
	gtx.Execute(key.FocusCmd{})
}

// neighborTab is the project or layout the step lands on in the strip's
// order, Home first, and false when the strip has nowhere to go. "" is Home.
func (u *window) neighborTab(step int) (string, bool) {
	tabs, active := u.model.stripTabs()
	ids := make([]string, 0, len(tabs)+1)
	ids = append(ids, "")
	for _, t := range tabs {
		ids = append(ids, t.ID)
	}
	if u.model.onHome() {
		active = ""
	}
	return neighborProject(ids, active, step)
}

func (u *window) hotkeyTab(step int) {
	target, ok := u.neighborTab(step)
	if !ok {
		return
	}
	u.settings.hide()
	if target == "" {
		u.model.showHome()
		u.saveScreen("")
		return
	}
	u.showStripTab(target)
}

// setSidebarOpen is the sidebar's own toggle, stored and morphed to.
func (u *window) setSidebarOpen(gtx C, open bool) {
	var act sidebarAction
	u.sidebar.setOpen(gtx, open, &act)
	u.applySidebarWrites(act.sidebarWrites)
}

func (u *window) hotkeySidebar(gtx C) {
	if !u.sidebar.morph.active() {
		u.setSidebarOpen(gtx, !u.sidebar.open)
	}
}

// hotkeyDock toggles the dock as a whole, reopening on the tab it last
// showed.
func (u *window) hotkeyDock(C) {
	d := u.dock
	switch {
	case d.tab != dockClosed:
		d.tab = dockClosed
	case d.shown != dockClosed:
		d.tab = d.shown
	default:
		d.tab = dockCode
	}
}

// hotkeySearch opens the Code tab on its text search with the field focused.
func (u *window) hotkeySearch(C) {
	u.settings.hide()
	u.dock.tab = dockCode
	u.dock.code.requestSearch()
}

func (u *window) hotkeyOpenSettings(C) {
	p, ok := u.model.project()
	if !ok {
		return
	}
	tabs, _ := u.model.projectTabs()
	u.settings.show(p.ID, settingsProjectsOf(tabs))
}
