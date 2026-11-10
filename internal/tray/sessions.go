package tray

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// sessionSlots is how many sessions each group of the menu names one by one,
// for the reason serviceSlots caps the services; past it, a line says how many
// more there are.
const sessionSlots = 8

// The caps on what a session's line quotes, so a long label or a waiting
// reason (the hook caps it at 120) does not stretch the menu across the screen.
const (
	labelRunes  = 32
	detailRunes = 40
)

// The marks before a session's line: the menu is the desktop's own text, with
// no colour of lich's to tell the groups apart.
const (
	markNeedsYou = "● "
	markWorking  = "◐ "
)

// The reported states the menu sorts on (docs/hooks/session-state.md). Every
// other one, idle, interrupted and a done somebody read, counts as idle.
const (
	stateBusy       = "busy"
	stateCompacting = "compacting"
	stateWaiting    = "waiting"
	stateDone       = "done"
)

// Sessions is what the tray reads and does about the sessions. Live lists the
// sessions with a process running, in the sidebar's order; Status is the state
// each last reported, as the window was told it; Unread is the set whose last
// turn nobody has read; Focus brings the window up on a session's card.
type Sessions struct {
	Live   func() ([]api.Peer, error)
	Status func(id string) api.SessionStatus
	Unread func() (map[string]bool, error)
	Focus  func(id string)
}

// sessionRow is one session's line in the menu: title as the menu shows it,
// full uncut for a tooltip.
type sessionRow struct {
	id, title, full string
}

func newRow(id, mark string, p api.Peer, detail string) sessionRow {
	name := cmp.Or(p.Label, p.Name)
	return sessionRow{
		id:    id,
		title: sessionLine(mark, p.Project, clip(name, labelRunes), clip(detail, detailRunes)),
		full:  sessionLine(mark, p.Project, name, detail),
	}
}

// sessionView is what the menu shows of the sessions, read in one go.
type sessionView struct {
	needs, working         []sessionRow
	needsMore, workingMore int
	idle                   int
	badge                  badge
	tooltip                string
}

// any is whether there is a session to show at all.
func (v sessionView) any() bool {
	return len(v.needs)+len(v.working)+v.idle > 0
}

// readSessions reads the sessions the menu shows. Without the sessions wired,
// it shows none.
func (t *Tray) readSessions(l Labels) (sessionView, error) {
	t.mu.Lock()
	s := t.sessions
	t.mu.Unlock()
	if s.Live == nil {
		return sessionView{}, nil
	}
	live, err := s.Live()
	if err != nil {
		return sessionView{}, err
	}
	unread, err := s.Unread()
	if err != nil {
		return sessionView{}, err
	}
	v := buildSessionView(live, s.Status, unread, l)
	t.mu.Lock()
	t.needIDs, t.workIDs = slotIDs(v.needs), slotIDs(v.working)
	t.mu.Unlock()
	return v, nil
}

func buildSessionView(live []api.Peer, status func(string) api.SessionStatus, unread map[string]bool, l Labels) sessionView {
	var v sessionView
	waiting, done, busy := false, false, false
	for _, p := range live {
		st := status(p.ID)
		switch {
		case st.State == stateWaiting:
			waiting = true
			v.needs = append(v.needs, newRow(p.ID, markNeedsYou, p, cmp.Or(st.Reason, l.Waiting)))
		case st.State == stateDone && unread[p.ID]:
			done = true
			v.needs = append(v.needs, newRow(p.ID, markNeedsYou, p, l.Done))
		case st.State == stateBusy || st.State == stateCompacting:
			busy = true
			v.working = append(v.working, newRow(p.ID, markWorking, p, st.Tool))
		default:
			v.idle++
		}
	}
	v.badge = badgeOf(waiting, done, busy)
	v.tooltip = tooltip(l, v.needs, v.working, v.idle)
	v.needs, v.needsMore = capRows(v.needs)
	v.working, v.workingMore = capRows(v.working)
	return v
}

// badgeOf is the icon's badge: a session waiting on you outranks a turn left
// unread, which outranks one still running.
func badgeOf(waiting, done, busy bool) badge {
	switch {
	case waiting:
		return badgeWaiting
	case done:
		return badgeDone
	case busy:
		return badgeBusy
	}
	return badgeNone
}

// tooltip is the icon's tooltip: every session the menu groups, uncut and past
// the slots too, since the menu's host cuts a long line and a menu item has no
// tooltip of its own on Linux.
func tooltip(l Labels, needs, working []sessionRow, idle int) string {
	lines := []string{"lich"}
	group := func(heading string, rows []sessionRow) {
		if len(rows) == 0 {
			return
		}
		lines = append(lines, heading)
		for _, r := range rows {
			lines = append(lines, r.full)
		}
	}
	group(l.NeedsYou, needs)
	group(l.Working, working)
	if idle > 0 {
		lines = append(lines, withCount(l.Idle, idle))
	}
	return strings.Join(lines, "\n")
}

// sessionLine is a session's line: its mark, its project, its name and what it
// is doing. The project leads because a tray host cuts a long line at its own
// width, and the name is the part that can lose its end.
func sessionLine(mark, project, name, detail string) string {
	var parts []string
	for _, part := range []string{project, name, detail} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return mark + strings.Join(parts, " · ")
}

func capRows(rows []sessionRow) ([]sessionRow, int) {
	if len(rows) <= sessionSlots {
		return rows, 0
	}
	return rows[:sessionSlots], len(rows) - sessionSlots
}

func slotIDs(rows []sessionRow) [sessionSlots]string {
	var ids [sessionSlots]string
	for i, r := range rows {
		ids[i] = r.id
	}
	return ids
}

// focusSlot brings the window up on the session slot i of a group showed when
// the menu was last refreshed; a slot showing nothing does nothing.
func (t *Tray) focusSlot(ids *[sessionSlots]string, i int) {
	t.mu.Lock()
	id, focus := ids[i], t.sessions.Focus
	t.mu.Unlock()
	if id != "" && focus != nil {
		focus(id)
	}
}

func clip(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}

func withCount(label string, n int) string {
	return strings.ReplaceAll(label, "{count}", strconv.Itoa(n))
}

// swapBadge records b as the badge the icon shows, and reports whether that is
// a change: the icon is only sent again when it is.
func (t *Tray) swapBadge(b badge) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	changed := t.badge != b
	t.badge = b
	return changed
}
