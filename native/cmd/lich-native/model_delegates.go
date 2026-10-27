package main

import (
	"context"
	"slices"
)

// The delegates of a session are the sessions it opened, in any project:
// the backend keeps the opener on each one (OriginSessionID).

// delegatesOf is every live session orchestrator opened, in project order
// and each project's own session order.
func (m *model) delegatesOf(orchestrator string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.delegatesLocked(orchestrator)
}

func (m *model) delegatesLocked(orchestrator string) []string {
	var ids []string
	for _, p := range m.projects {
		for _, s := range p.Sessions {
			if s.OriginSessionID == orchestrator && s.ID != orchestrator {
				ids = append(ids, s.ID)
			}
		}
	}
	return ids
}

// delegateSlot is where orchestrator's next delegates go in its layout:
// under the last of its delegates there, else in a column beside
// orchestrator itself. below tells the two apart.
func (m *model) delegateSlot(orchestrator string, delegates []string) (target string, below bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.delegateSlotLocked(orchestrator, delegates)
}

func (m *model) delegateSlotLocked(orchestrator string, delegates []string) (target string, below bool) {
	i := m.layouts.of(orchestrator)
	if i < 0 {
		return orchestrator, false
	}
	here := slices.DeleteFunc(m.layouts.tabs[i].sessions(), func(id string) bool { return !slices.Contains(delegates, id) })
	if len(here) == 0 {
		return orchestrator, false
	}
	return here[len(here)-1], true
}

// showDelegates lays orchestrator out with ids in a column at its right:
// in a new layout named after it when it is in none, else in its own layout
// at delegateSlot. A delegate in another layout is taken out of it; asking
// first is the caller's. It reports false when nothing was laid out.
func (m *model) showDelegates(ctx context.Context, orchestrator string, ids []string) bool {
	m.mu.Lock()
	if len(ids) == 0 || !m.liveLocked(orchestrator) || slices.ContainsFunc(ids, func(id string) bool { return !m.liveLocked(id) }) {
		m.mu.Unlock()
		return false
	}
	if m.layouts.locked(m.layouts.of(orchestrator)) {
		m.mu.Unlock()
		return false
	}
	target, below := m.delegateSlotLocked(orchestrator, m.delegatesLocked(orchestrator))
	for _, id := range ids {
		m.layouts.take(id)
	}
	switch i := m.layouts.of(orchestrator); {
	case i < 0:
		p, at := m.sessionAt(orchestrator)
		m.layouts.openBeside(p.Sessions[at].Label, orchestrator, ids)
	case below:
		m.layouts.tabs[i].stackUnder(target, ids)
	default:
		m.layouts.tabs[i].columnBeside(target, ids)
	}
	m.mu.Unlock()
	m.setActive(ctx, orchestrator)
	m.saveLayouts()
	return true
}

// delegateOffer is what orchestrator's delegates chip offers: how many
// delegates it has, how many of them sit in its layout, and the rest.
type delegateOffer struct {
	all     []string
	here    int
	missing []string
}

func (m *model) delegateOffer(orchestrator string) delegateOffer {
	m.mu.Lock()
	defer m.mu.Unlock()
	o := delegateOffer{all: m.delegatesLocked(orchestrator)}
	var layout *layoutTab
	if i := m.layouts.of(orchestrator); i >= 0 {
		layout = &m.layouts.tabs[i]
	}
	for _, id := range o.all {
		if layout != nil && layout.holds(id) {
			o.here++
			continue
		}
		o.missing = append(o.missing, id)
	}
	return o
}

// inOtherLayouts counts the ids that sit in a layout other than
// orchestrator's, which laying them out beside it would take them out of.
func (m *model) inOtherLayouts(orchestrator string, ids []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	own, n := m.layouts.of(orchestrator), 0
	for _, id := range ids {
		if at := m.layouts.of(id); at >= 0 && at != own {
			n++
		}
	}
	return n
}
