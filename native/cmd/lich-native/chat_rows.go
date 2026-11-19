package main

import (
	"slices"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

type chatRowKind uint8

const (
	chatRowItem chatRowKind = iota // one item as it is
	chatRowWork                    // a turn's tool calls, folded into one line
	chatRowLive                    // the turn in flight, unless a permission says it waits
	chatRowDay                     // the day a turn starts on, when it is not the day of the turn before
)

// chatRow is one line of the chat view.
type chatRow struct {
	kind chatRowKind
	item backend.ChatItem // chatRowItem
	// day is a day row's moment, unix ms: when the turn it starts began.
	day int64
	// turn, tools and took are a work row's: its tool calls and how long the
	// turn has run.
	turn  int
	tools []backend.ChatItem
	took  time.Duration
}

type chatTurnSpan struct {
	start, end int64
	tools      []backend.ChatItem
	// began is the first time any item of the turn carries, 0 for none: a
	// conversation read back from the provider may leave the person's message
	// without one, and its answer still tells the day.
	began int64
}

// chatRows lays items out as the view draws them: each turn's tool calls
// folded into one work row where its first one was, and while running, a
// live row last, which a waiting permission's box stands in for. A turn has run from its first item to its last, or to now
// while it is the one in flight: the history carries no durations.
func chatRows(items []backend.ChatItem, running bool, now time.Time) []chatRow {
	spans := map[int]*chatTurnSpan{}
	last := 0
	for _, it := range items {
		sp := spans[it.Turn]
		if sp == nil {
			sp = &chatTurnSpan{start: it.At, end: it.At}
			spans[it.Turn] = sp
		}
		sp.start, sp.end = min(sp.start, it.At), max(sp.end, it.At)
		if it.At > 0 && (sp.began == 0 || it.At < sp.began) {
			sp.began = it.At
		}
		if it.Kind == api.ItemTool {
			sp.tools = append(sp.tools, it)
		}
		last = max(last, it.Turn)
	}
	var rows []chatRow
	folded := map[int]bool{}
	var turnDay time.Time
	for _, it := range items {
		if began := spans[it.Turn].began; it.Kind == api.ItemUser && began > 0 {
			day := time.UnixMilli(began).In(now.Location())
			if !turnDay.IsZero() && !sameDay(turnDay, day) {
				rows = append(rows, chatRow{kind: chatRowDay, day: began})
			}
			turnDay = day
		}
		if it.Kind != api.ItemTool {
			rows = append(rows, chatRow{kind: chatRowItem, item: it})
			continue
		}
		if folded[it.Turn] {
			continue
		}
		folded[it.Turn] = true
		sp := spans[it.Turn]
		end := sp.end
		if running && it.Turn == last {
			end = now.UnixMilli()
		}
		rows = append(rows, chatRow{kind: chatRowWork, turn: it.Turn, tools: sp.tools, took: time.Duration(max(end-sp.start, 0)) * time.Millisecond})
	}
	if running && permissionWaiting(items) == nil {
		rows = append(rows, chatRow{kind: chatRowLive})
	}
	return rows
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// permissionWaiting is the permission item still unanswered, the last one,
// nil for none.
func permissionWaiting(items []backend.ChatItem) *backend.ChatItem {
	for _, it := range slices.Backward(items) {
		if it.Kind == api.ItemPermission && it.Permission != nil && it.Permission.Chosen == "" {
			return &it
		}
	}
	return nil
}

// permitOption is p's option of the first of kinds it offers, "" for none:
// Enter allows once before always, Esc rejects.
func permitOption(p *backend.ChatPermission, kinds ...string) string {
	for _, k := range kinds {
		if i := slices.IndexFunc(p.Options, func(o api.ChatPermitOption) bool { return o.Kind == k }); i >= 0 {
			return p.Options[i].ID
		}
	}
	return ""
}

// chatSwitchBlock is why the GUI | TUI selector takes no click.
type chatSwitchBlock uint8

const (
	switchFree    chatSwitchBlock = iota
	switchWorking                 // the agent is in a turn
	switchPending                 // a switch is on its way
)

// chatWorking is whether session sv's agent is in a turn: a chat's turn in
// flight, or the status a terminal's hooks report.
func chatWorking(sv sessionView, running bool) bool {
	return running || spins(sv.status)
}

// chatSwitchState is whether sv gets the selector, and what keeps it from a
// click.
func chatSwitchState(offers bool, sv sessionView, running, switching bool) (shown bool, block chatSwitchBlock) {
	switch {
	case !offers:
		return false, switchFree
	case switching:
		return true, switchPending
	case chatWorking(sv, running):
		return true, switchWorking
	}
	return true, switchFree
}
