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
	if i, ok := c.index[item.ID]; ok {
		item.Turn, item.At = c.items[i].Turn, c.items[i].At
		c.items[i] = item
	} else {
		item.Turn = c.turn
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
