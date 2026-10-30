package main

import (
	"maps"
	"slices"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

// exitDuration is how long a closed card stays drawn, animating out. The card's
// exit animation is timed from it.
const exitDuration = 180 * time.Millisecond

// closingCard is a session that left the list and the slot it left behind.
type closingCard struct {
	session sessionView
	index   int
}

// departed is the sessions of previous that live no longer holds, with their
// slots.
func departed(previous, live []sessionView) []closingCard {
	var gone []closingCard
	for i, s := range previous {
		if !holds(live, s.ID) {
			gone = append(gone, closingCard{session: s, index: i})
		}
	}
	return gone
}

// withClosing is live with each closing session put back where it was, in
// slot order whatever order they closed in.
func withClosing(live []sessionView, closing []closingCard) []sessionView {
	shown := slices.Clone(live)
	sorted := slices.Clone(closing)
	slices.SortStableFunc(sorted, func(a, b closingCard) int { return a.index - b.index })
	for _, c := range sorted {
		shown = slices.Insert(shown, min(c.index, len(shown)), c.session)
	}
	return shown
}

// holdDeparted is held plus what left between previous and live, or held
// itself (false) when nothing did. Slots are read off the list as drawn, held
// cards included, or a card closing behind one still on its way out is put
// back in front of it. A card closed twice is held once, at its new slot.
func holdDeparted(held []closingCard, previous, live []sessionView) ([]closingCard, bool) {
	var away []closingCard
	for _, c := range held {
		if !holds(previous, c.session.ID) {
			away = append(away, c)
		}
	}
	var gone []closingCard
	for _, c := range departed(withClosing(previous, away), live) {
		if holds(previous, c.session.ID) {
			gone = append(gone, c)
		}
	}
	if len(gone) == 0 {
		return held, false
	}
	kept := slices.DeleteFunc(slices.Clone(held), func(c closingCard) bool {
		return slices.ContainsFunc(gone, func(g closingCard) bool { return g.session.ID == c.session.ID })
	})
	return append(kept, gone...), true
}

// closingCards holds the closed cards of one block, with the clock passed in.
// Every held card is dropped exitDuration after the held set last changed.
type closingCards struct {
	previous []sessionView
	held     []closingCard
	since    time.Time
}

// shown is live with the cards still closing put back, and which ids those
// are. A session back while its card was going counts as live again.
func (c *closingCards) shown(live []sessionView, now time.Time) ([]sessionView, map[string]bool) {
	if len(c.held) > 0 && now.Sub(c.since) >= exitDuration {
		c.held = nil
	}
	if next, changed := holdDeparted(c.held, c.previous, live); changed {
		c.held, c.since = next, now
	}
	c.previous = slices.Clone(live)
	closing := map[string]bool{}
	var still []closingCard
	for _, h := range c.held {
		if !holds(live, h.session.ID) {
			still = append(still, h)
			closing[h.session.ID] = true
		}
	}
	if len(still) == 0 {
		return live, closing
	}
	return withClosing(live, still), closing
}

// expiry is when the held cards drop, the zero time with none held: the frame
// to schedule a redraw for.
func (c *closingCards) expiry() time.Time {
	if len(c.held) == 0 {
		return time.Time{}
	}
	return c.since.Add(exitDuration)
}

func holds(list []sessionView, id string) bool {
	return slices.ContainsFunc(list, func(s sessionView) bool { return s.ID == id })
}

// blockPresence is the block list's motion: a block that arrives after its
// project's list first drew grows in, and one that leaves is drawn as it last
// was while it shrinks out. Another project starts it over: its blocks are
// not news.
type blockPresence struct {
	project string
	motion  ui.Presence
	last    map[string]blockView
	// A checkout block draws its header as a tree row, so it comes in and
	// leaves by the tree's motion instead (blockMove): moving holds those,
	// live the keys of the last frame and drawn its order.
	moving map[string]blockMove
	live   map[string]bool
	drawn  []string
}

// shown is live with the blocks still leaving put back in their slots.
func (p *blockPresence) shown(now time.Time, project string, live []blockView) []blockView {
	if p.last == nil || project != p.project {
		*p = blockPresence{project: project, last: map[string]blockView{}, moving: map[string]blockMove{}}
	}
	keys := make([]string, len(live))
	for i, b := range live {
		keys[i] = b.key
		p.last[b.key] = b
	}
	p.move(now, live)
	// Presence plays its own exit for a block that left; a moving one is
	// drawn by its motion instead, past that exit, in the slot it left.
	shownKeys := p.motion.Update(now, keys)
	for key, m := range p.moving {
		if m.leaving && !slices.Contains(shownKeys, key) {
			at := 0
			if i := slices.Index(p.drawn, key); i > 0 {
				at = slices.Index(shownKeys, p.drawn[i-1]) + 1
			}
			shownKeys = slices.Insert(shownKeys, at, key)
		}
	}
	maps.DeleteFunc(p.last, func(key string, _ blockView) bool { return !slices.Contains(shownKeys, key) })
	p.drawn = shownKeys
	shown := make([]blockView, len(shownKeys))
	for i, key := range shownKeys {
		shown[i] = p.last[key]
		if m, ok := p.moving[key]; ok {
			shown[i].move = &m
			if !m.leaving {
				p.motion.Settle(key)
			}
		}
	}
	return shown
}

// leaving reports whether key is drawn only for its exit.
func (p *blockPresence) leaving(key string) bool {
	m, ok := p.moving[key]
	return ok && m.leaving || p.motion.Leaving(key)
}

// move starts the checkout blocks that came in or left since the last frame
// on their motion, and drops the ones whose motion has played. The list's
// first frame is not news.
func (p *blockPresence) move(now time.Time, live []blockView) {
	settled := p.live != nil
	still := map[string]bool{}
	for _, b := range live {
		still[b.key] = true
		if settled && !p.live[b.key] && b.under != "" {
			p.moving[b.key] = blockMove{start: now, rows: len(blockRows(b, b.sessions))}
		}
	}
	for key := range p.live {
		if last := p.last[key]; !still[key] && last.under != "" {
			p.moving[key] = blockMove{start: now, leaving: true, rows: len(blockRows(last, last.sessions))}
		}
	}
	maps.DeleteFunc(p.moving, func(_ string, m blockMove) bool { return !now.Before(m.end()) })
	p.live = still
}
