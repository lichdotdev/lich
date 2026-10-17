package main

import (
	"context"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// The command palette, frontend/src/components/CommandPalette.tsx: the
// app-wide quick switcher. One chord opens it from anywhere, to jump to any
// session of any open project, to a project open or closed, to what was said
// inside a session or to a session parked in the history, or, behind a
// leading ">", to run an action. This file is its state and its searches; the
// panel is palette_view.go and what a row does is palette_window.go.

// paletteBackfillPoll is how long the History list waits before asking again
// while the store is still indexing parked conversations. The backfill reads
// one session every tenth of a second, so a second between polls is a handful
// of new rows per ask.
const paletteBackfillPoll = time.Second

// closedAnswer is the store's page of closed projects matching a term, with
// every match counted and the directories that are gone.
type closedAnswer struct {
	rows    []paletteProject
	total   int
	missing map[string]bool
}

// historyAnswer is the store's page of parked sessions matching a term and
// what the store has not indexed yet.
type historyAnswer struct {
	rows     []paletteHistory
	total    int
	indexing int
}

type commandPalette struct {
	open   bool
	dialog ui.DialogState
	query  widget.Editor
	tab    paletteTab
	// cursor is the row the arrow keys are on, clamped to the list drawn.
	cursor int
	// typed and typedTab are what the cursor was placed for: another query or
	// tab starts it from the top.
	typed    string
	typedTab paletteTab
	// reveal asks the list to scroll the cursor's row into view.
	reveal bool
	// focus asks for the field's focus on the frame the palette opens.
	focus bool
	// total is the rows the last frame drew, which the arrow keys clamp to.
	total   int
	hovered int
	settle  settler
	list    widget.List
	tabs    map[paletteTab]*widget.Clickable
	rows    []widget.Clickable
	// opened counts openings: an answer never outlives the opening it was
	// asked in, so a reopened palette does not flash the last one's rows.
	opened int
	// running is the sessions holding a turn, sampled once per opening: an
	// order that moved under the cursor while a row is being aimed at costs
	// more than the freshness is worth.
	running map[string]bool
	polls   int
	pollAt  time.Time
	// dropped is the parked sessions forgotten from the list in place.
	dropped map[string]bool
	// picked is the actions Enter chose, run once the frame is over.
	picked []hotkeyID
	// last is the list the last open frame drew, which the closing fade keeps
	// drawing.
	last paletteView

	// closed is the window's store of closed projects, shared with the
	// open-project menu. closedTerm is the settled search it answers for and
	// closedOff is set while the palette is on actions, which search nothing.
	closed     *closedProjects
	closedTerm string
	closedOff  bool

	history  remote[historyAnswer]
	external remote[[]lichclient.ExternalSession]
	said     remote[[]lichclient.TranscriptMatch]
}

func newCommandPalette(invalidate func()) *commandPalette {
	p := &commandPalette{tab: tabAll, tabs: map[paletteTab]*widget.Clickable{}, dropped: map[string]bool{}}
	p.history.invalidate = invalidate
	p.external.invalidate, p.said.invalidate = invalidate, invalidate
	p.list.Axis = layout.Vertical
	return p
}

// show opens the palette on the plain list with the field focused.
func (p *commandPalette) show(running map[string]bool) {
	p.open, p.focus = true, true
	p.opened++
	if p.closed != nil {
		// A directory deleted since the last opening is not an event the
		// backend sends, so each opening asks again.
		p.closed.stale()
	}
	p.running = running
	p.dropped = map[string]bool{}
	p.reset()
}

// hide closes the palette; the next one opens on the plain list again.
func (p *commandPalette) hide() {
	p.open = false
	p.reset()
}

func (p *commandPalette) reset() {
	p.query.SetText("")
	p.tab, p.cursor, p.reveal = tabAll, 0, true
	p.polls, p.pollAt = 0, time.Time{}
}

func (p *commandPalette) tabClick(tab paletteTab) *widget.Clickable {
	if p.tabs[tab] == nil {
		p.tabs[tab] = new(widget.Clickable)
	}
	return p.tabs[tab]
}

// active is the cursor inside the list drawn: a list that shrank under it
// leaves the raw index past its end.
func (p *commandPalette) active(total int) int {
	return min(p.cursor, max(total-1, 0))
}

// keys carries out the keys the field takes while the palette is open: the
// arrows walk the rows, Tab the filters, Enter runs the row under the cursor,
// which it reports. Escape is the dialog's.
func (p *commandPalette) keys(gtx C) (enter bool) {
	names := []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter, key.NameTab}
	filters := make([]event.Filter, len(names))
	for i, name := range names {
		filters[i] = key.Filter{Focus: &p.query, Name: name, Optional: key.ModShift}
	}
	_, actions := actionTerm(p.query.Text())
	at := p.active(p.total)
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return enter
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameDownArrow:
			at = min(at+1, max(p.total-1, 0))
			p.cursor, p.reveal = at, true
		case key.NameUpArrow:
			at = max(at-1, 0)
			p.cursor, p.reveal = at, true
		case key.NameReturn, key.NameEnter:
			enter = true
		case key.NameTab:
			// The panel traps the focus and has nothing else worth tabbing to,
			// so Tab walks the filters.
			if !actions {
				step := 1
				if e.Modifiers.Contain(key.ModShift) {
					step = -1
				}
				p.tab = nextTab(p.tab, step)
			}
		}
	}
}

// ask points the four searches at what the palette shows now, and drops what
// it is not showing. The term goes to the store rather than filtering rows in
// hand: the store answers with one page, so a session parked or a project
// closed further back than that page is only reachable if the match happens
// there.
func (p *commandPalette) ask(gtx C, c *lichclient.Client, typed string, searching, onHistory bool, sessions []paletteSession) {
	reset := strconv.Itoa(p.opened)
	if !searching {
		p.pollAt = time.Time{}
		p.closedOff = true
		p.history.want("", reset, nil)
		p.external.want("", reset, nil)
		p.said.want("", reset, nil)
		return
	}
	term, next := p.settle.at(typed, gtx.Now)
	if !next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	search := "q:" + term
	p.closedTerm, p.closedOff = term, false
	p.history.want(search+"#"+strconv.Itoa(p.pollHistory(gtx.Now, term)), reset, func(ctx context.Context) (historyAnswer, error) {
		return fetchHistory(ctx, c, term)
	})
	if onHistory {
		p.external.want("external", reset, c.ExternalSessions)
	} else {
		p.external.want("", reset, nil)
	}
	if utf8.RuneCountInString(strings.TrimSpace(term)) < paletteMessageMinQuery {
		p.said.want("", reset, nil)
		return
	}
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.sessionID
	}
	p.said.want(term+"\x00"+strings.Join(ids, ","), reset, func(ctx context.Context) ([]lichclient.TranscriptMatch, error) {
		matches, err := c.SearchTranscripts(ctx, ids, term)
		if err != nil {
			// A search that only ever adds: a failed one reads as no rows and
			// leaves the name-matched groups alone.
			log.Printf("search transcripts: %v", err)
			return nil, nil
		}
		return matches, nil
	})
}

// pollHistory is the counter that re-asks the history while the store is
// still indexing. Only behind a term: the store starts its backfill on a term
// and on nothing else, so with the palette on the plain list the count never
// moves and polling it would report nothing.
func (p *commandPalette) pollHistory(now time.Time, term string) int {
	answer, _, _ := p.history.get()
	if answer.indexing == 0 || strings.TrimSpace(term) == "" {
		p.pollAt = time.Time{}
		return p.polls
	}
	if p.pollAt.IsZero() {
		p.pollAt = now.Add(paletteBackfillPoll)
	}
	if !now.Before(p.pollAt) {
		p.polls++
		p.pollAt = time.Time{}
	}
	return p.polls
}

// wake asks for the frame the next poll is due on.
func (p *commandPalette) wake(gtx C) {
	if !p.pollAt.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: p.pollAt})
	}
}

func fetchHistory(ctx context.Context, c *lichclient.Client, term string) (historyAnswer, error) {
	h, err := c.ClosedSessions(ctx, term)
	if err != nil {
		return historyAnswer{}, err
	}
	// The branches come in one batch rather than by subscribing each row to
	// the git poll a live card uses: that is three calls per path a second.
	var paths []string
	for _, s := range h.Sessions {
		if s.Path != "" {
			paths = append(paths, s.Path)
		}
	}
	missing := map[string]bool{}
	gone, err := c.MissingPaths(ctx, paths)
	if err != nil {
		log.Printf("check parked session paths: %v", err)
	}
	for _, path := range gone {
		missing[path] = true
	}
	branches, err := c.BranchesOf(ctx, paths)
	if err != nil {
		log.Printf("read parked session branches: %v", err)
	}
	return historyAnswer{rows: historyRows(h.Sessions, branches, missing), total: h.Total, indexing: h.Indexing}, nil
}

// paletteView is one frame's list: the groups and their rows flattened, with
// what the rows and the tabs report about themselves.
type paletteView struct {
	groups []paletteGroup
	rows   []paletteRow
	// offsets is the flat index of each group's first row.
	offsets []int
	// counts is what each tab holds; absent for All.
	counts     map[paletteTab]int
	actionMode bool
	// emptyHistory is the History tab of a workspace that has closed nothing,
	// which says so rather than that a query found nothing.
	emptyHistory bool
	missing      map[string]bool
	sessionCount map[string]int
	query        string
}

// view joins the open workspace to the searches' answers into the list this
// frame draws.
func (p *commandPalette) view(w paletteWorld, bound hotkeyBindings, isMac bool, available func(hotkeyID) bool) paletteView {
	query := p.query.Text()
	closed, history, external, said := p.answers()
	v := paletteView{query: strings.TrimSpace(query), missing: closed.missing, sessionCount: w.counts, counts: map[paletteTab]int{}}
	all := rankSessions(w.sessions, p.running)
	rows := history.rows
	total := history.total
	if len(p.dropped) > 0 {
		rows = slices.DeleteFunc(slices.Clone(rows), func(h paletteHistory) bool {
			if p.dropped[h.ID] {
				total--
				return true
			}
			return false
		})
	}
	results := filterPalette(query, all, w.projects, closed.rows, rows, closed.total, external)
	messages := paletteMessagesOf(said, all)
	// The indexing note waits for a term: with none there is no search for it
	// to be incomplete, and the backfill it reports is not running either.
	backlog := history.indexing
	if v.query == "" {
		backlog = 0
	}
	term, actions := actionTerm(query)
	v.actionMode = actions
	if actions {
		v.groups = actionGroups(term, bound, isMac, available)
	} else {
		v.groups = paletteGroupsOf(p.tab, results, messages, max(total, 0), backlog)
		for _, tab := range paletteTabs {
			if n, ok := paletteTabCount(tab, results, messages); ok {
				v.counts[tab] = n
			}
		}
	}
	for _, g := range v.groups {
		v.offsets = append(v.offsets, len(v.rows))
		v.rows = append(v.rows, g.rows...)
	}
	v.emptyHistory = p.tab == tabHistory && v.query == "" && len(rows) == 0
	return v
}

// answers are the searches' latest answers, empty while an opening has none.
func (p *commandPalette) answers() (closedAnswer, historyAnswer, []lichclient.ExternalSession, []lichclient.TranscriptMatch) {
	var closed closedAnswer
	if p.closed != nil && !p.closedOff {
		closed = p.closed.read(p.closedTerm)
	}
	history, _, _ := p.history.get()
	external, _, _ := p.external.get()
	said, _, _ := p.said.get()
	return closed, history, external, said
}

// rowVerb is what Enter says it does with a row, for the hint bar.
func rowVerb(r paletteRow) string {
	switch r.kind {
	case rowHistory:
		if historyAction(r.history) == historyForget {
			return i18n.T("palette.palette.verbForget")
		}
		return i18n.T("palette.palette.verbResume")
	case rowExternal:
		return i18n.T("palette.palette.verbResume")
	case rowAction:
		return i18n.T("palette.palette.verbRun")
	}
	return i18n.T("palette.palette.verbOpen")
}
