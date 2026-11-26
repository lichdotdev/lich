package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// cardSelection is the cards picked with the shortcut modifier held, in the
// order they were picked.
type cardSelection struct{ ids []string }

func (c *cardSelection) has(id string) bool { return slices.Contains(c.ids, id) }

func (c *cardSelection) toggle(id string) {
	if i := slices.Index(c.ids, id); i >= 0 {
		c.ids = slices.Delete(c.ids, i, i+1)
		return
	}
	c.ids = append(c.ids, id)
}

func (c *cardSelection) clear() { c.ids = nil }

// keep drops the picked cards live no longer holds: a session closed from
// anywhere, or one of a project no longer shown.
func (c *cardSelection) keep(live []backend.Session) {
	c.ids = slices.DeleteFunc(c.ids, func(id string) bool {
		return !slices.ContainsFunc(live, func(s backend.Session) bool { return s.ID == id })
	})
}

// menu is the menu a selected card opens while more than one is selected:
// what acts on all of them at once. nil for the card's own menu.
func (c *cardSelection) menu(id string, act *sidebarAction) []ui.MenuEntry {
	if len(c.ids) < 2 || !c.has(id) {
		return nil
	}
	entry := closeAllEntry(slices.Clone(c.ids), act)
	pick := entry.OnClick
	entry.OnClick = func() { pick(); c.clear() }
	return []ui.MenuEntry{entry}
}

// descendantsOf is every session under id in sessions, children before their
// own children, id itself left out even where an origin loops back to it.
func descendantsOf(sessions []sessionView, id string) []string {
	var out []string
	seen := map[string]bool{id: true}
	var walk func(up string)
	walk = func(up string) {
		for _, sv := range sessions {
			if sv.OriginSessionID == up && !seen[sv.ID] {
				seen[sv.ID] = true
				out = append(out, sv.ID)
				walk(sv.ID)
			}
		}
	}
	walk(id)
	return out
}
