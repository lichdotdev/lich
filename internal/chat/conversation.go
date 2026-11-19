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
	// quiet holds the window's copy back while a resumed conversation is
	// replayed: those items are the history, which the window asks for once.
	quiet bool

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

// userMessage opens a turn with the person's text and returns that turn.
func (c *conversation) userMessage(text string) int {
	c.mu.Lock()
	c.turn++
	turn := c.turn
	c.mu.Unlock()
	c.put(api.ChatItem{ID: "user-" + strconv.Itoa(turn), Kind: api.ItemUser, Text: text}, true)
	return turn
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
	fn(&it)
	c.put(it, final)
}

// put stores item, keeping the turn and start time an earlier copy set, and
// tells the window now or once the gap is up.
func (c *conversation) put(item api.ChatItem, final bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stampEnd(&item)
	if i, ok := c.index[item.ID]; ok {
		item.Turn, item.At = c.items[i].Turn, c.items[i].At
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
		return
	}
	wait := itemGap - c.now().Sub(c.sent[item.ID])
	if final || wait <= 0 {
		c.sendLocked(item.ID)
		return
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
