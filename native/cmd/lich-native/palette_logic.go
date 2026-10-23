package main

import (
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// The command palette's data join: open projects and their sessions, the closed
// projects, the parked sessions and what was said inside a session become one
// list of groups the palette draws and routes from. Pure, so the flatten, the
// filter and the ranking are tested without a frame.

// paletteMessageMinQuery mirrors the backend's minSearchQuery: under three
// characters the transcript search is refused, so it is not asked for.
const paletteMessageMinQuery = 3

// paletteAllRows is how much of a group the All tab shows: enough that a
// project with a handful of sessions is not represented by its first two, and
// short enough that the three groups fit without scrolling.
const paletteAllRows = 5

// paletteIndexCap is terminal.indexTextBytes spelled the way a reader reads it.
// The backend says whether the cap cut; how big it is belongs to the sentence
// the window writes.
const paletteIndexCap = "8 MB"

// paletteActionPrefix switches the palette from jumping to running actions.
// The mode lives in the query and nowhere else.
const paletteActionPrefix = ">"

// paletteSession is one session of an open project, flattened with it: what a
// jump-to-session row shows and routes to. path is where it runs, its own
// checkout else the project's.
type paletteSession struct {
	sessionID, projectID, projectName, label, kind, path string
	// status, unread and agent are what the row's ring and mark draw.
	status, agent string
	unread        bool
}

// paletteProject is an open or a closed project: the same three fields.
type paletteProject = backend.RecentProject

// paletteHistory is one parked session as the History tab lists it: the
// store's row plus the branch its checkout is on now, read from git. That is
// not ParkedBranch, the snapshot the close recorded for the search to match.
// branch is "" for a checkout that is gone, which is also the row that cannot
// be resumed.
type paletteHistory struct {
	backend.ClosedSession
	branch string
	// gone is the checkout no longer on disk: the row forgets rather than
	// resumes.
	gone bool
}

// paletteMessage is a session the query was talked about in: the same row a
// jump-to-session is, carrying the sentence that made it a hit.
type paletteMessage struct {
	paletteSession
	snippet string
	count   int
}

type paletteTab string

const (
	tabAll      paletteTab = "All"
	tabSessions paletteTab = "Sessions"
	tabProjects paletteTab = "Projects"
	tabMessages paletteTab = "Messages"
	tabHistory  paletteTab = "History"
)

// label is the tab named in the interface language. The tab's value stays
// English: it is the -palette-tab flag and the identity the window compares.
func (t paletteTab) label() string {
	switch t {
	case tabSessions:
		return i18n.T("palette.commandPalette.tab.Sessions")
	case tabProjects:
		return i18n.T("palette.commandPalette.tab.Projects")
	case tabMessages:
		return i18n.T("palette.commandPalette.tab.Messages")
	case tabHistory:
		return i18n.T("palette.commandPalette.tab.History")
	}
	return i18n.T("palette.commandPalette.tab.All")
}

// paletteTabs is the filter row in the order Tab walks it. History is last
// because it is the one posture the palette is not opened in: the other four
// are asked mid-work, and this one when the work is already over.
var paletteTabs = []paletteTab{tabAll, tabSessions, tabProjects, tabMessages, tabHistory}

type paletteKind int

const (
	rowSession paletteKind = iota
	rowProject
	rowClosed
	rowMessage
	rowHistory
	rowExternal
	rowAction
)

// paletteRow is one option in the list, carrying what running it needs. The
// groups are drawn in order and the rows flattened back out for the arrow
// keys, so a row's index is its position in that flat list.
type paletteRow struct {
	kind     paletteKind
	session  paletteSession
	project  paletteProject
	message  paletteMessage
	history  paletteHistory
	external backend.ExternalSession
	action   hotkeyID
	keys     string
}

type paletteGroup struct {
	label string
	rows  []paletteRow
	// total is the rows the group stands for before a cut, so its header can
	// say what it leaves out; never fewer than the rows drawn.
	total int
	// note replaces the page count when there is something more urgent to
	// say, as the parked conversations still being indexed.
	note string
}

// paletteResults is the query's answer, by kind.
type paletteResults struct {
	sessions []paletteSession
	projects []paletteProject
	// closed is one page of the closed projects the store matched, and
	// closedTotal every match, so the header reports a page that was cut.
	closed      []paletteProject
	closedTotal int
	history     []paletteHistory
	external    []backend.ExternalSession
}

// paletteSessionsOf flattens every session of the open projects.
func paletteSessionsOf(projects []backend.Project) []paletteSession {
	var rows []paletteSession
	for _, p := range projects {
		for _, s := range p.Sessions {
			rows = append(rows, paletteSession{
				sessionID: s.ID, projectID: p.ID, projectName: p.Name, label: s.Label, kind: s.Kind, path: sessionPath(p, s),
			})
		}
	}
	return rows
}

// historyRows joins the parked rows to what git says about their checkouts. A
// path with no branch and a path in missing are different failures, not a
// repository versus not there at all, but the row only acts on the second.
func historyRows(closed []backend.ClosedSession, branches map[string]string, missing map[string]bool) []paletteHistory {
	rows := make([]paletteHistory, len(closed))
	for i, s := range closed {
		rows[i] = paletteHistory{ClosedSession: s, branch: branches[s.Path], gone: missing[s.Path]}
	}
	return rows
}

// filterPalette narrows each kind to the query. A parked row the store matched
// on its conversation is kept without asking again: the snippet is one window
// onto a hit that may be a megabyte of prose, or no window at all for a hit
// found by folding an accent. Both branches are in the history haystack
// because they can disagree: the store matched the one recorded at the close,
// and the row shows the one git reads now.
func filterPalette(query string, all []paletteSession, projects, closed []paletteProject, history []paletteHistory, closedTotal int, external []backend.ExternalSession) paletteResults {
	r := paletteResults{closedTotal: closedTotal}
	for _, s := range all {
		if matchesQuery(s.label+" "+s.projectName+" "+s.path, query) {
			r.sessions = append(r.sessions, s)
		}
	}
	for _, p := range projects {
		if matchesQuery(p.Name+" "+p.Path, query) {
			r.projects = append(r.projects, p)
		}
	}
	for _, p := range closed {
		if matchesQuery(p.Name+" "+p.Path, query) {
			r.closed = append(r.closed, p)
		}
	}
	for _, h := range history {
		if h.MatchedConversation || matchesQuery(h.Label+" "+h.ProjectName+" "+h.branch+" "+h.ParkedBranch+" "+h.Path, query) {
			r.history = append(r.history, h)
		}
	}
	for _, e := range external {
		dir := e.Path
		if dir == "" {
			dir = e.ProjectPath
		}
		if matchesQuery(e.Title+" "+e.ProjectName+" "+dir, query) {
			r.external = append(r.external, e)
		}
	}
	return r
}

// paletteMessagesOf joins transcript matches back onto the sessions they were
// searched for, in the order the search returned. A match whose session closed
// since the query went out has nowhere to jump to and is dropped.
func paletteMessagesOf(matches []backend.TranscriptMatch, sessions []paletteSession) []paletteMessage {
	var rows []paletteMessage
	for _, m := range matches {
		i := slices.IndexFunc(sessions, func(s paletteSession) bool { return s.sessionID == m.ID })
		if i >= 0 {
			rows = append(rows, paletteMessage{paletteSession: sessions[i], snippet: m.Snippet, count: m.Count})
		}
	}
	return rows
}

// rankSessions puts the sessions worth jumping to first, ahead of the cut the
// All tab then makes: the tab strip's order alone would hand the whole group
// to whatever sits in the first project. turns is the status of each session
// holding a turn, taken when the palette opens. A session waiting on the user
// leads, then the rest holding a turn; a shell is last, having no turn to hold
// and no agent to come back to. The sort is stable, so sessions of equal rank
// keep the tab order.
func rankSessions(sessions []paletteSession, turns map[string]string) []paletteSession {
	rank := func(s paletteSession) int {
		status, holding := turns[s.sessionID]
		switch {
		case status == statusWaiting:
			return 0
		case holding:
			return 1
		case s.kind == "shell":
			return 3
		}
		return 2
	}
	ranked := slices.Clone(sessions)
	slices.SortStableFunc(ranked, func(a, b paletteSession) int { return rank(a) - rank(b) })
	return ranked
}

// querySegments marks in text every run the query's tokens match, the way
// the history search splits a query into words.
func querySegments(text, query string) []segment {
	segs := []segment{{text: text}}
	for _, token := range strings.Fields(query) {
		var next []segment
		for _, s := range segs {
			if s.hit {
				next = append(next, s)
				continue
			}
			next = append(next, highlightSegments(s.text, token)...)
		}
		segs = next
	}
	return segs
}

// rowKey identifies a row inside the list it is drawn in. A session and a
// message about it share an id, and they never share a group.
func rowKey(r paletteRow) string {
	switch r.kind {
	case rowSession:
		return r.session.sessionID
	case rowMessage:
		return r.message.sessionID
	case rowHistory:
		return r.history.ID
	case rowExternal:
		return "external:" + r.external.ProviderSessionID
	case rowAction:
		return "action:" + string(r.action)
	}
	return r.project.ID
}

// historyIndexNote is the line a parked session shows when its conversation
// was indexed only in part: the cap kept the newest of it, so a search that
// found nothing there found nothing in the part that was kept.
func historyIndexNote(h paletteHistory) string {
	if h.Truncated {
		return i18n.T("palette.paletteLogic.indexedNewest", i18n.Args{"size": paletteIndexCap})
	}
	return ""
}

type historyDoor int

const (
	historyResume historyDoor = iota
	historyForget
)

// historyAction is what Enter does with a history row. A checkout that is gone
// has nothing to resume into: the row is a leftover a removal outside lich
// left behind, and the only honest action is to drop it.
func historyAction(h paletteHistory) historyDoor {
	if h.gone {
		return historyForget
	}
	return historyResume
}

// externalLabel is what an external conversation is called on its row and on
// the card it becomes: a provider that never titled it still needs a name.
func externalLabel(e backend.ExternalSession) string {
	if title := strings.TrimSpace(e.Title); title != "" {
		return title
	}
	return i18n.T("session.palette.untitledConversation")
}

// adoptedSession is the parked row AdoptExternalSession files, as the
// history's resume takes it: the same door a closed session goes back through.
func adoptedSession(e backend.ExternalSession, id string) backend.ClosedSession {
	return backend.ClosedSession{
		ID: id, ProjectID: e.ProjectID, ProjectName: e.ProjectName, ProjectPath: e.ProjectPath,
		Label: externalLabel(e), Kind: e.Kind, Path: e.Path,
	}
}

func rowsOf[T any](items []T, row func(T) paletteRow) []paletteRow {
	rows := make([]paletteRow, len(items))
	for i, item := range items {
		rows[i] = row(item)
	}
	return rows
}

// group builds one group, cut to limit rows when limit is above 0. total is
// how many rows it stands for when the backend cut the list before it got
// here; the group never stands for fewer than the rows it was given.
func group(label string, rows []paletteRow, limit, total int) paletteGroup {
	total = max(total, len(rows))
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return paletteGroup{label: label, rows: rows, total: total}
}

// paletteGroupsOf lists the tab's groups, the empty ones dropped. historyTotal
// is how many parked sessions the store matched, which is more than
// results.history holds whenever its page cut; indexing is how many it has not
// read a conversation out of yet, which the header says before a page boundary.
func paletteGroupsOf(tab paletteTab, r paletteResults, messages []paletteMessage, historyTotal, indexing int) []paletteGroup {
	sessions := rowsOf(r.sessions, func(s paletteSession) paletteRow { return paletteRow{kind: rowSession, session: s} })
	open := rowsOf(r.projects, func(p paletteProject) paletteRow { return paletteRow{kind: rowProject, project: p} })
	closed := rowsOf(r.closed, func(p paletteProject) paletteRow { return paletteRow{kind: rowClosed, project: p} })
	said := rowsOf(messages, func(m paletteMessage) paletteRow { return paletteRow{kind: rowMessage, message: m} })
	history := rowsOf(r.history, func(h paletteHistory) paletteRow { return paletteRow{kind: rowHistory, history: h} })
	external := rowsOf(r.external, func(e backend.ExternalSession) paletteRow { return paletteRow{kind: rowExternal, external: e} })

	var groups []paletteGroup
	switch tab {
	case tabSessions:
		groups = []paletteGroup{group(i18n.T("session.palette.groupSessions"), sessions, 0, 0)}
	case tabProjects:
		groups = []paletteGroup{group(i18n.T("session.palette.groupOpen"), open, 0, 0), group(i18n.T("session.palette.groupClosed"), closed, 0, r.closedTotal)}
	case tabMessages:
		groups = []paletteGroup{group(i18n.T("session.palette.groupMessages"), said, 0, 0)}
	case tabHistory:
		parked := group(i18n.T("session.palette.groupClosedSessions"), history, 0, historyTotal)
		if indexing > 0 {
			parked.note = indexingNote(indexing)
		}
		groups = []paletteGroup{parked, group(i18n.T("session.palette.groupOutsideLich"), external, 0, 0)}
	default:
		// No closed projects or sessions here: bringing one back is not what
		// the palette is reached for mid-work, and the rows it costs are rows
		// the sessions and the open projects are cut to make room for.
		groups = []paletteGroup{
			group(i18n.T("session.palette.groupSessions"), sessions, paletteAllRows, 0),
			group(i18n.T("session.palette.groupProjects"), open, paletteAllRows, 0),
			group(i18n.T("session.palette.groupMessages"), said, paletteAllRows, 0),
		}
	}
	return slices.DeleteFunc(groups, func(g paletteGroup) bool { return len(g.rows) == 0 })
}

func indexingNote(n int) string {
	return i18n.T("session.palette.indexing", i18n.Args{"count": n})
}

// paletteTabCount is what a tab reports it holds; All reports nothing, being
// every other tab added up.
func paletteTabCount(tab paletteTab, r paletteResults, messages []paletteMessage) (int, bool) {
	switch tab {
	case tabSessions:
		return len(r.sessions), true
	case tabProjects:
		return len(r.projects) + len(r.closed), true
	case tabMessages:
		return len(messages), true
	case tabHistory:
		return len(r.history) + len(r.external), true
	}
	return 0, false
}

// nextTab walks the filter row and wraps at both ends, so Tab alone reaches
// every tab and Shift+Tab is never a dead key on the first one.
func nextTab(tab paletteTab, step int) paletteTab {
	n := len(paletteTabs)
	return paletteTabs[(slices.Index(paletteTabs, tab)+step+n)%n]
}

// actionTerm is the part of the query an action is searched by, and false when
// the query is not asking for actions at all.
func actionTerm(query string) (string, bool) {
	return strings.CutPrefix(query, paletteActionPrefix)
}

// actionGroups lists the actions matching term, grouped as the shortcuts
// overlay groups them, each with its chord as currently bound. The palette's
// own action is left out: run from the palette it would only close the palette
// it was run from. available is what this window can carry out.
func actionGroups(term string, bound hotkeyBindings, isMac bool, available func(hotkeyID) bool) []paletteGroup {
	var groups []paletteGroup
	for _, g := range hotkeyGroups {
		var rows []paletteRow
		for _, a := range hotkeyActions {
			if a.group != g.id || a.id == hotkeyCommandPalette || !available(a.id) || !matchesQuery(a.label, term) {
				continue
			}
			rows = append(rows, paletteRow{kind: rowAction, action: a.id, keys: formatCombo(bound[a.id], isMac)})
		}
		if len(rows) > 0 {
			groups = append(groups, group(g.label, rows, 0, 0))
		}
	}
	return groups
}

const (
	agoMinutesPerHour = 60
	agoHoursPerDay    = 24
	agoDaysPerWeek    = 7
)

// agoLabel says how long ago at (Unix seconds) was, in the coarsest unit that
// still says something, floored so it never claims time that has not passed. 0
// is a row that predates lich recording it, which is no date rather than 1970,
// and under a minute is "just now".
func agoLabel(at int64, now time.Time) string {
	if at <= 0 {
		return ""
	}
	minutes := int(max(now.Sub(time.Unix(at, 0)), 0) / time.Minute)
	switch {
	case minutes < 1:
		return i18n.T("common.time.justNow")
	case minutes < agoMinutesPerHour:
		return i18n.T("session.age.ago.minute", i18n.Args{"count": minutes})
	}
	hours := minutes / agoMinutesPerHour
	if hours < agoHoursPerDay {
		return i18n.T("session.age.ago.hour", i18n.Args{"count": hours})
	}
	days := hours / agoHoursPerDay
	if days < agoDaysPerWeek {
		return i18n.T("session.age.ago.day", i18n.Args{"count": days})
	}
	return i18n.T("session.age.ago.week", i18n.Args{"count": days / agoDaysPerWeek})
}
