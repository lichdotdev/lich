package chat

import (
	"slices"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// compactCommands are the commands a provider compacts its conversation on
// when the person types one: every provider's /compact, and opencode's and
// Cursor's /summarize. One counts only where the provider lists it.
var compactCommands = []string{"/compact", "/summarize"}

// compactStart opens a running compaction, unless one still runs: Claude Code
// says it is compacting again every 30 seconds, and a provider's own start
// follows the one a typed command opened.
func (c *conversation) compactStart(auto bool, before int) {
	if cc, ok := c.openCompaction(); ok && cc.State == api.CompactionRunning {
		return
	}
	c.compactOpen(func(cc *api.ChatCompaction) { cc.Auto, cc.Before = auto, before })
}

// compactOpen begins a compaction item, running unless fn says otherwise, and
// leaves it open in place of the one before.
func (c *conversation) compactOpen(fn func(*api.ChatCompaction)) {
	c.mu.Lock()
	c.compactions++
	id := compactionID(c.compactions)
	c.compacting = id
	c.mu.Unlock()
	cc := &api.ChatCompaction{State: api.CompactionRunning}
	fn(cc)
	c.put(api.ChatItem{ID: id, Kind: api.ItemCompaction, Compaction: cc}, true)
}

// compaction changes the open compaction through fn, beginning one where none
// is open: a provider says a compaction's end, tokens, summary and hook output
// in lines of their own, in an order none of them promises.
func (c *conversation) compaction(fn func(*api.ChatCompaction)) {
	c.mu.Lock()
	id := c.compacting
	c.mu.Unlock()
	if id == "" {
		c.compactOpen(fn)
		return
	}
	c.change(id, fn)
}

// change rewrites compaction item id through fn, on a copy: the window may
// still hold the one sent before.
func (c *conversation) change(id string, fn func(*api.ChatCompaction)) {
	c.update(id, api.ItemCompaction, true, func(it *api.ChatItem) {
		cc := api.ChatCompaction{State: api.CompactionRunning}
		if it.Compaction != nil {
			cc = *it.Compaction
		}
		fn(&cc)
		it.Compaction = &cc
	})
}

// openCompaction is the compaction what the provider says next belongs to,
// false for none.
func (c *conversation) openCompaction() (api.ChatCompaction, bool) {
	c.mu.Lock()
	id := c.compacting
	c.mu.Unlock()
	return c.compactionItem(id)
}

func (c *conversation) compactionItem(id string) (api.ChatCompaction, bool) {
	it, ok := c.get(id)
	if !ok || it.Compaction == nil {
		return api.ChatCompaction{}, false
	}
	return *it.Compaction, true
}

// compactClose ends the open compaction, one still running in state: nothing
// the provider says after belongs to it.
func (c *conversation) compactClose(state string) {
	if cc, ok := c.openCompaction(); ok && cc.State == api.CompactionRunning {
		c.compaction(func(cc *api.ChatCompaction) { cc.State = state })
	}
	c.mu.Lock()
	c.compacting = ""
	c.mu.Unlock()
}

// compactAfter gives the latest compaction the tokens a provider counted once
// it was over, the first count under its before: a count taken as it started
// still holds the conversation it folded.
func (c *conversation) compactAfter(tokens int) {
	c.mu.Lock()
	id := compactionID(c.compactions)
	c.mu.Unlock()
	if cc, ok := c.compactionItem(id); ok && cc.Before > 0 && cc.After == 0 && tokens > 0 && tokens < cc.Before {
		c.change(id, func(cc *api.ChatCompaction) { cc.After = tokens })
	}
}

func compactionID(n int) string { return "compaction-" + strconv.Itoa(n) }

// compactState is how a compaction its provider left running ended, by the
// turn that held it.
func compactState(reason string) string {
	switch reason {
	case stopEndTurn:
		return api.CompactionDone
	case stopCancelled:
		return api.CompactionCancelled
	}
	return api.CompactionFailed
}

// reading takes the conversation's tokens as the provider last counted them,
// the before of the next compaction and maybe the after of the last.
func (c *chatSession) reading(tokens int) {
	c.mu.Lock()
	c.tokens = tokens
	c.mu.Unlock()
	c.conv.compactAfter(tokens)
}

// counted is the provider's last count of the conversation's tokens, 0 for
// none.
func (c *chatSession) counted() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens
}

// compactTyped opens the compaction a message asks for, where its first word
// is a compaction command the provider lists: the turn it starts compacts.
func (c *chatSession) compactTyped(text string) {
	word, _, _ := strings.Cut(strings.TrimSpace(text), " ")
	if !slices.Contains(compactCommands, word) {
		return
	}
	c.mu.Lock()
	listed := slices.ContainsFunc(c.cmds, func(cmd api.ChatCommand) bool { return cmd.Text == word })
	c.mu.Unlock()
	if listed {
		c.conv.compactStart(false, c.counted())
	}
}

// failure says why the turn failed: on the compaction running in it, where
// one is, and otherwise in a notice.
func (c *chatSession) failure(text string) {
	if cc, ok := c.conv.openCompaction(); ok && cc.State == api.CompactionRunning {
		c.conv.compaction(func(cc *api.ChatCompaction) { cc.State, cc.Error = api.CompactionFailed, text })
		return
	}
	c.conv.notice(text)
}
