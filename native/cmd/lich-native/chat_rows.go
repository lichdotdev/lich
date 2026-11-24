package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

type chatRowKind uint8

const (
	chatRowItem  chatRowKind = iota // one item as it is
	chatRowWork                     // a turn's tool calls, folded into one line
	chatRowShell                    // the call of the person's shell command, its output under it
	chatRowEdit                     // a call that changed a file, its diff under it, out of the turn's fold
	chatRowLive                     // the turn in flight, unless a permission or a question says it waits
	chatRowDay                      // the day a turn starts on, when it is not the day of the turn before
	chatRowDone                     // the line that closes a finished turn
)

// chatRow is one line of the chat view.
type chatRow struct {
	kind chatRowKind
	item backend.ChatItem // chatRowItem, chatRowShell, chatRowEdit
	// day is a day row's moment, unix ms: when the turn it starts began.
	day int64
	// avatar marks the work row that opens its turn, which carries the
	// provider's mark.
	avatar bool
	// more marks an answer's text another follows in the turn, with nothing
	// between them: the run reads as one answer, so only its last text keeps
	// the time and copy line, and runText is what that copy takes, every text
	// of the run joined; empty for a text alone.
	more    bool
	runText string
	// live marks the row's item as alive: the thought being thought, the
	// answer being written, a turn's tool calls while one of them runs.
	// movable marks a live row whose turn waits on a command Ctrl+B moves,
	// and compacting one whose turn is compacting.
	live, movable, compacting bool
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
// live row last, which a waiting permission's or question's box stands in for. A turn has run from its first item to its last, or to now
// while it is the one in flight, a tool call's end included: most histories
// carry no durations.
//
// In a chat of kind whose provider runs the person's shell command, the call
// right after their "!" message is that command's: a row of its own, kept out
// of the turn's work row, whose agent answers after it.
func chatRows(items []backend.ChatItem, kind string, running bool, now time.Time) []chatRow {
	if slices.ContainsFunc(items, isPlan) {
		items = slices.DeleteFunc(slices.Clone(items), isPlan)
	}
	spans := map[int]*chatTurnSpan{}
	asked := map[int]bool{}
	shell := map[string]bool{}
	runsShell := slices.Contains(chatRunShellKinds, kind)
	var prev backend.ChatItem
	last := 0
	for _, it := range items {
		if runsShell && it.Kind == api.ItemTool && prev.Kind == api.ItemUser && prev.Turn == it.Turn && strings.HasPrefix(prev.Text, chatShellMark) {
			shell[it.ID] = true
		}
		prev = it
		sp := spans[it.Turn]
		if sp == nil {
			sp = &chatTurnSpan{start: it.At, end: it.At}
			spans[it.Turn] = sp
		}
		sp.start, sp.end = min(sp.start, it.At), max(sp.end, it.At)
		if it.At > 0 && (sp.began == 0 || it.At < sp.began) {
			sp.began = it.At
		}
		if it.Kind == api.ItemTool && !shell[it.ID] {
			if !isEdit(it) {
				sp.tools = append(sp.tools, it)
			}
			if it.Tool != nil {
				sp.end = max(sp.end, it.Tool.Ended)
			}
		}
		asked[it.Turn] = asked[it.Turn] || it.Kind == api.ItemUser
		last = max(last, it.Turn)
	}
	var rows []chatRow
	folded := map[int]bool{}
	// opener is what first speaks in a turn, "work" or "agent".
	opener := map[int]string{}
	var turnDay time.Time
	// The item being produced is the conversation's last, while it runs.
	liveID := ""
	if running && len(items) > 0 {
		liveID = items[len(items)-1].ID
	}
	for _, it := range items {
		if began := spans[it.Turn].began; it.Kind == api.ItemUser && began > 0 {
			day := time.UnixMilli(began).In(now.Location())
			if !turnDay.IsZero() && !sameDay(turnDay, day) {
				rows = append(rows, chatRow{kind: chatRowDay, day: began, turn: it.Turn})
			}
			turnDay = day
		}
		if it.Kind == api.ItemCompaction && !drawnCompaction(it) || asksInComposer(it) {
			continue
		}
		if it.Kind != api.ItemTool {
			r := chatRow{kind: chatRowItem, item: it, live: it.ID == liveID && (it.Kind == api.ItemAgent || it.Kind == api.ItemThought)}
			// Thoughts back to back read as one: the first keeps its id, so
			// its fold stays put while the rest streams into its text.
			if it.Kind == api.ItemThought && len(rows) > 0 {
				prev := &rows[len(rows)-1]
				if prev.kind == chatRowItem && prev.item.Kind == api.ItemThought && prev.item.Turn == it.Turn {
					prev.item.Text += "\n\n" + it.Text
					prev.live = r.live
					continue
				}
			}
			if it.Kind == api.ItemAgent && opener[it.Turn] == "" {
				opener[it.Turn] = "agent"
			}
			rows = append(rows, r)
			continue
		}
		if shell[it.ID] {
			rows = append(rows, chatRow{kind: chatRowShell, item: it, turn: it.Turn, live: running && it.Turn == last && callRunning(it)})
			rows = append(rows, fileRows(it)...)
			continue
		}
		if isEdit(it) {
			rows = append(rows, chatRow{kind: chatRowEdit, item: it, turn: it.Turn, live: running && it.Turn == last && callRunning(it)})
			continue
		}
		if folded[it.Turn] {
			rows = append(rows, fileRows(it)...)
			continue
		}
		folded[it.Turn] = true
		sp := spans[it.Turn]
		end := sp.end
		if running && it.Turn == last {
			end = now.UnixMilli()
		}
		work := chatRow{kind: chatRowWork, avatar: opener[it.Turn] == "", turn: it.Turn, tools: sp.tools, ended: sp.end, live: running && it.Turn == last && toolRunning(sp.tools),
			took: time.Duration(max(end-sp.start, 0)) * time.Millisecond}
		if opener[it.Turn] == "" {
			opener[it.Turn] = "work"
		}
		rows = append(rows, work)
		rows = append(rows, fileRows(it)...)
	}
	// The conversation's last turn, once it has ended, closes with a line of
	// its own, unless it was the person's compaction, whose marker closes it;
	// the person's next message takes it away. A question still
	// waiting holds the turn open, whatever a window opened mid-turn heard.
	if !running && len(items) > 0 && asked[last] && questionWaiting(items) == nil && !compactTurn(items, last) {
		rows = append(rows, chatRow{kind: chatRowDone, turn: last})
	}
	if running && permissionWaiting(items) == nil && questionWaiting(items) == nil {
		live := chatRow{kind: chatRowLive, turn: last, movable: movable(items), compacting: compactRunning(items)}
		if sp := spans[last]; sp != nil && sp.start > 0 && !now.IsZero() {
			live.took = time.Duration(max(now.UnixMilli()-sp.start, 0)) * time.Millisecond
		}
		rows = append(rows, live)
	}
	markAnswerRuns(rows)
	return rows
}

// markAnswerRuns sets more and runText on each run of answer texts that follow
// one another in a turn.
func markAnswerRuns(rows []chatRow) {
	answer := func(r chatRow) bool { return r.kind == chatRowItem && r.item.Kind == api.ItemAgent }
	for i := 0; i < len(rows); {
		j := i
		for j+1 < len(rows) && answer(rows[j]) && answer(rows[j+1]) && rows[j+1].item.Turn == rows[j].item.Turn {
			j++
		}
		if j > i {
			texts := make([]string, 0, j-i+1)
			for k := i; k <= j; k++ {
				texts = append(texts, rows[k].item.Text)
				rows[k].more = k < j
			}
			rows[j].runText = strings.Join(texts, "\n\n")
		}
		i = j + 1
	}
}

// isEdit is whether it is a call that changed a file, whatever the provider
// names it: every provider's chat gives such a call a Diff, and only it.
func isEdit(it backend.ChatItem) bool { return it.Tool != nil && it.Tool.Diff != "" }

// fileRows are the files call it changed with no diff of its own, a shell
// command's edits among them, each an edit row of its own after the call, as
// an edit tool's are: the call's id and the path name it.
func fileRows(it backend.ChatItem) []chatRow {
	if it.Tool == nil || len(it.Tool.Files) == 0 {
		return nil
	}
	rows := make([]chatRow, 0, len(it.Tool.Files))
	for _, f := range it.Tool.Files {
		file := backend.ChatItem{ID: it.ID + "#" + f.Path, Kind: api.ItemTool, Turn: it.Turn, At: it.At,
			Tool: &backend.ChatToolCall{Name: it.Tool.Name, Title: f.Path, State: api.ToolDone, Diff: f.Diff}}
		rows = append(rows, chatRow{kind: chatRowEdit, item: file, turn: it.Turn})
	}
	return rows
}

// isPlan is whether it is the agent's task list, drawn over the composer
// (chat_plan.go) and not in the conversation.
func isPlan(it backend.ChatItem) bool { return it.Kind == api.ItemPlan }

// toolRunning is whether any of calls has not ended.
func toolRunning(calls []backend.ChatItem) bool { return slices.ContainsFunc(calls, callRunning) }

// movable is whether a running call of the last turn of items waits on a
// command Ctrl+B moves to the background: one an ended turn left unended is
// not alive.
func movable(items []backend.ChatItem) bool {
	if len(items) == 0 {
		return false
	}
	last := items[len(items)-1].Turn
	return slices.ContainsFunc(items, func(it backend.ChatItem) bool { return it.Turn == last && callRunning(it) && it.Tool.Movable })
}

// callRunning is whether the tool call it has not ended.
func callRunning(it backend.ChatItem) bool {
	return it.Tool != nil && (it.Tool.State == api.ToolPending || it.Tool.State == api.ToolRunning)
}

// chatLiveKey is the live line's key: there is one at most.
const chatLiveKey = "live"

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
		return chatLiveKey
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
