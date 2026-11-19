package main

import (
	"slices"
	"strconv"
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
	chatRowDone                    // the line that closes a finished turn
)

// chatRow is one line of the chat view.
type chatRow struct {
	kind chatRowKind
	item backend.ChatItem // chatRowItem
	// day is a day row's moment, unix ms: when the turn it starts began.
	day int64
	// lead marks the first answer of a turn, which opens with who speaks. When
	// a work row came before it in the turn, the avatar is on that row and
	// bare says the answer's header leaves it out; avatar marks the work row
	// that carries it.
	lead, bare, avatar bool
	// live marks the row's item as alive: the thought being thought, the
	// answer being written, a turn's tool calls while one of them runs.
	live bool
	// ended is when a work row's turn last did something, unix ms.
	ended int64
	// turn, tools and took are a work row's: its tool calls and how long the
	// turn has run; a live row has the turn and how long it has run.
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
	asked := map[int]bool{}
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
		asked[it.Turn] = asked[it.Turn] || it.Kind == api.ItemUser
		last = max(last, it.Turn)
	}
	var rows []chatRow
	// A finished turn that began with the person's message ends in a line of
	// its own, after its last item.
	closing := func(turn int) {
		if asked[turn] {
			rows = append(rows, chatRow{kind: chatRowDone, turn: turn})
		}
	}
	folded := map[int]bool{}
	led := map[int]bool{}
	// opener is what first speaks in a turn, "work" or "agent".
	opener := map[int]string{}
	var turnDay time.Time
	// The item being produced is the conversation's last, while it runs.
	liveID := ""
	if running && len(items) > 0 {
		liveID = items[len(items)-1].ID
	}
	for i, it := range items {
		if i > 0 && items[i-1].Turn != it.Turn {
			closing(items[i-1].Turn)
		}
		if began := spans[it.Turn].began; it.Kind == api.ItemUser && began > 0 {
			day := time.UnixMilli(began).In(now.Location())
			if !turnDay.IsZero() && !sameDay(turnDay, day) {
				rows = append(rows, chatRow{kind: chatRowDay, day: began, turn: it.Turn})
			}
			turnDay = day
		}
		if it.Kind != api.ItemTool {
			r := chatRow{kind: chatRowItem, item: it, live: it.ID == liveID && (it.Kind == api.ItemAgent || it.Kind == api.ItemThought)}
			if it.Kind == api.ItemAgent && !led[it.Turn] {
				led[it.Turn], r.lead = true, true
				r.bare = opener[it.Turn] == "work"
				if opener[it.Turn] == "" {
					opener[it.Turn] = "agent"
				}
			}
			rows = append(rows, r)
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
		work := chatRow{kind: chatRowWork, avatar: opener[it.Turn] == "", turn: it.Turn, tools: sp.tools, ended: sp.end, live: running && toolRunning(sp.tools),
			took: time.Duration(max(end-sp.start, 0)) * time.Millisecond}
		if opener[it.Turn] == "" {
			opener[it.Turn] = "work"
		}
		rows = append(rows, work)
	}
	if !running && len(items) > 0 {
		closing(items[len(items)-1].Turn)
	}
	if running && permissionWaiting(items) == nil {
		live := chatRow{kind: chatRowLive, turn: last}
		if sp := spans[last]; sp != nil && sp.start > 0 && !now.IsZero() {
			live.took = time.Duration(max(now.UnixMilli()-sp.start, 0)) * time.Millisecond
		}
		rows = append(rows, live)
	}
	return rows
}

// toolRunning is whether any of calls has not ended.
func toolRunning(calls []backend.ChatItem) bool {
	return slices.ContainsFunc(calls, func(it backend.ChatItem) bool {
		return it.Tool != nil && (it.Tool.State == api.ToolPending || it.Tool.State == api.ToolRunning)
	})
}

// chatRowKey names a row for its entry: an item by its id, the others by the
// turn they belong to.
func chatRowKey(r chatRow) string {
	switch r.kind {
	case chatRowWork:
		return "work:" + strconv.Itoa(r.turn)
	case chatRowDay:
		return "day:" + strconv.Itoa(r.turn)
	case chatRowDone:
		return "done:" + strconv.Itoa(r.turn)
	case chatRowLive:
		return "live"
	}
	return r.item.ID
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
