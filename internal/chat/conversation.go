package chat

import (
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
)

// itemGap is the shortest time between two ChatItemEvents of one item: a
// provider streams a reply a few characters per message, and the window
// redraws the whole item each time, so twenty a second is as smooth as the eye
// reads and a fraction of the traffic.
const itemGap = 50 * time.Millisecond

// conversation is one chat's items in the order they began, and the window's
// copy of them: every change to an item reaches the window whole, no more often
// than itemGap.
type conversation struct {
	mu      sync.Mutex
	items   []api.ChatItem
	index   map[string]int
	turn    int
	notices int
	// command is the notice of the last local command, which its output
	// completes if it comes next; any other item ends it.
	command string
	// compacting is the compaction item what the provider says next belongs
	// to (compaction.go), "" for none; compactions counts them.
	compacting  string
	compactions int
	// compactMoved hears a compaction start running live, true, and stop
	// running, false (chatSession.compactMoved). Never for a replayed one.
	compactMoved func(running bool)
	// quiet holds the window's copy back while a resumed conversation is
	// replayed: those items are the history, which the window asks for once.
	quiet bool
	// ended hears a tool call end live, once, after the item is stored
	// (chatSession.callEnded). Never for a replayed one.
	ended func(api.ChatItem)

	emit    func(api.ChatItem)
	now     func() time.Time
	sent    map[string]time.Time
	pending map[string]*time.Timer
}

func newConversation(emit func(api.ChatItem)) *conversation {
	return &conversation{
		index:   map[string]int{},
		emit:    emit,
		now:     time.Now,
		sent:    map[string]time.Time{},
		pending: map[string]*time.Timer{},
	}
}

// userMessage opens a turn with the person's text and the files it attached,
// and returns that turn.
func (c *conversation) userMessage(text string, files []string) int {
	c.mu.Lock()
	c.turn++
	turn := c.turn
	c.mu.Unlock()
	c.put(api.ChatItem{ID: "user-" + strconv.Itoa(turn), Kind: api.ItemUser, Text: text, Files: files}, true)
	return turn
}

// turnText is the turn in flight and the person's text that opened it.
func (c *conversation) turnText() (int, string) {
	c.mu.Lock()
	turn := c.turn
	c.mu.Unlock()
	it, _ := c.get("user-" + strconv.Itoa(turn))
	return turn, it.Text
}

// notice adds something lich or the provider says about the session.
func (c *conversation) notice(text string) {
	c.mu.Lock()
	c.notices++
	id := "notice-" + strconv.Itoa(c.notices)
	c.mu.Unlock()
	c.put(api.ChatItem{ID: id, Kind: api.ItemNotice, Text: text}, true)
}

// get is the item id, false for none.
func (c *conversation) get(id string) (api.ChatItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, ok := c.index[id]
	if !ok {
		return api.ChatItem{}, false
	}
	return c.items[i], true
}

// update changes item id in place through fn, or begins it from fn applied to
// a zero item of kind, and tells the window. final skips the gap: the change
// ends the item, and the window must not wait for it.
func (c *conversation) update(id, kind string, final bool, fn func(*api.ChatItem)) {
	c.mu.Lock()
	var it api.ChatItem
	if i, ok := c.index[id]; ok {
		it = c.items[i]
	} else {
		it = api.ChatItem{ID: id, Kind: kind}
	}
	c.mu.Unlock()
	// fn changes a copy of the call: on the stored one, put would find the
	// change already made and miss the call ending.
	if it.Tool != nil {
		t := *it.Tool
		it.Tool = &t
	}
	fn(&it)
	c.put(it, final)
}

// put stores item, keeping the turn and start time an earlier copy set, and
// tells the window now or once the gap is up.
func (c *conversation) put(item api.ChatItem, final bool) {
	if c.putLocked(item, final) && c.ended != nil {
		c.ended(item)
	}
}

// putLocked is put under the lock, reporting whether item is a call seen
// ending live for the first time.
func (c *conversation) putLocked(item api.ChatItem, final bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ends := !c.quiet && toolEnded(item) && !c.endedBefore(item.ID)
	c.stampEnd(&item)
	if i, ok := c.index[item.ID]; ok {
		item.Turn, item.At = c.items[i].Turn, c.items[i].At
		keepFiles(&item, c.items[i])
		c.items[i] = item
	} else {
		item.Turn = c.turn
		c.command = ""
		if item.At == 0 {
			item.At = c.now().UnixMilli()
		}
		c.index[item.ID] = len(c.items)
		c.items = append(c.items, item)
	}
	if c.quiet {
		return ends
	}
	wait := itemGap - c.now().Sub(c.sent[item.ID])
	if final || wait <= 0 {
		c.sendLocked(item.ID)
		return ends
	}
	if _, queued := c.pending[item.ID]; !queued {
		id := item.ID
		c.pending[id] = time.AfterFunc(wait, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if _, still := c.pending[id]; still {
				c.sendLocked(id)
			}
		})
	}
	return ends
}

// sendLocked tells the window item id as it stands now. Called with mu held,
// which keeps two copies of one item from reaching the window out of order.
func (c *conversation) sendLocked(id string) {
	if t, ok := c.pending[id]; ok {
		t.Stop()
		delete(c.pending, id)
	}
	c.sent[id] = c.now()
	c.emit(c.items[c.index[id]])
}

// flush tells the window every item still waiting out its gap, which a turn's
// end does before it says the turn ended.
func (c *conversation) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range slices.Sorted(maps.Keys(c.pending)) {
		c.sendLocked(id)
	}
}

// toolEnded is whether item is a tool call that has ended.
func toolEnded(item api.ChatItem) bool {
	return item.Tool != nil && (item.Tool.State == api.ToolDone || item.Tool.State == api.ToolFailed)
}

// endedBefore is whether the stored copy of item id had already ended.
func (c *conversation) endedBefore(id string) bool {
	i, ok := c.index[id]
	return ok && toolEnded(c.items[i])
}

// keepFiles carries the files lich found a call changed over a copy of it the
// provider sends again, which knows nothing of them.
func keepFiles(item *api.ChatItem, stored api.ChatItem) {
	if item.Tool == nil || item.Tool.Files != nil || stored.Tool == nil || stored.Tool.Files == nil {
		return
	}
	t := *item.Tool
	t.Files = stored.Tool.Files
	item.Tool = &t
}

// attachFiles sets the files call id changed and tells the window; a call
// no longer in the conversation is left alone.
func (c *conversation) attachFiles(id string, files []api.ChatFileChange) {
	it, ok := c.get(id)
	if !ok || it.Tool == nil {
		return
	}
	t := *it.Tool
	t.Files = files
	it.Tool = &t
	c.put(it, true)
}

// stampEnd sets when a tool call ended, the first time it is seen ended live
// and its provider gave no time: a replayed one ended who knows when.
func (c *conversation) stampEnd(item *api.ChatItem) {
	t := item.Tool
	if t == nil || t.Ended != 0 || (t.State != api.ToolDone && t.State != api.ToolFailed) {
		return
	}
	if i, ok := c.index[item.ID]; ok && c.items[i].Tool != nil && c.items[i].Tool.Ended != 0 {
		t.Ended = c.items[i].Tool.Ended
		return
	}
	if !c.quiet {
		t.Ended = c.now().UnixMilli()
	}
}

// stopCalls ends, at now, the calls of the turn in flight its provider left
// unended, interrupted with the turn: their time stops where the turn did.
func (c *conversation) stopCalls() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.quiet {
		return
	}
	for i, it := range c.items {
		if t := it.Tool; it.Turn == c.turn && t != nil && t.Ended == 0 && (t.State == api.ToolPending || t.State == api.ToolRunning) {
			stopped := *t
			stopped.Ended = c.now().UnixMilli()
			c.items[i].Tool = &stopped
			c.sendLocked(it.ID)
		}
	}
}

// setQuiet holds the window's copy back, or lets it go again.
func (c *conversation) setQuiet(quiet bool) {
	c.mu.Lock()
	c.quiet = quiet
	c.mu.Unlock()
}

// replaying reports whether the window's copy is held back.
func (c *conversation) replaying() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.quiet
}

// snapshot is every item so far.
func (c *conversation) snapshot() []api.ChatItem {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.items)
}

// renameModels rewrites the model of every item that has one through name,
// without telling the window: it is for a history the window has not read.
func (c *conversation) renameModels(name func(string) string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.items {
		if c.items[i].Model != "" {
			c.items[i].Model = name(c.items[i].Model)
		}
	}
}

// attribute gives an item the model and effort that produced it, once: what
// an item began with stays, whatever the chat is set to afterwards.
func attribute(it *api.ChatItem, model, effort string) {
	if it.Model == "" && it.Effort == "" {
		it.Model, it.Effort = model, effort
	}
}

// commandNotice adds the line of a local command the agent's CLI ran.
func (c *conversation) commandNotice(cmd string) {
	c.mu.Lock()
	c.notices++
	id := "notice-" + strconv.Itoa(c.notices)
	c.mu.Unlock()
	c.put(api.ChatItem{ID: id, Kind: api.ItemNotice, Text: cmd}, true)
	c.mu.Lock()
	c.command = id
	c.mu.Unlock()
}

// commandOutput puts what a local command printed on the command's own line,
// or on a line of its own when no command came before it.
func (c *conversation) commandOutput(out string) {
	c.mu.Lock()
	id := c.command
	c.command = ""
	c.mu.Unlock()
	if out == "" {
		return
	}
	if id == "" {
		c.notice(out)
		return
	}
	c.update(id, api.ItemNotice, true, func(it *api.ChatItem) { it.Text += " · " + out })
}

// plan puts the agent's task list as it stands now on the turn's plan item:
// the window shows the latest turn's, so a list the agent leaves behind goes
// with the turn.
func (c *conversation) plan(entries []api.ChatPlanEntry) {
	c.mu.Lock()
	id := "plan-" + strconv.Itoa(c.turn)
	c.mu.Unlock()
	c.put(api.ChatItem{ID: id, Kind: api.ItemPlan, Plan: &api.ChatPlan{Entries: entries}}, true)
}
