package main

import (
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/extension"
)

// parseShortcut reads an extension's keys, "r", "shift+g" or "ctrl+k", as a
// combo; ctrl, cmd and mod are all the primary modifier, as the window's own
// hotkeys spell it. It reports false for a key this window cannot name.
func parseShortcut(keys string) (combo, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(keys)), "+")
	var c combo
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "ctrl", "cmd", "mod", "meta":
			c.Mod = true
		case "shift":
			c.Shift = true
		case "alt":
			c.Alt = true
		default:
			return combo{}, false
		}
	}
	c.Key = shortcutKey(parts[len(parts)-1])
	if len(gioNames(c.Key)) == 0 {
		return combo{}, false
	}
	return c, true
}

// shortcutKey is the KeyboardEvent.key name a lowered key spells: "enter" is
// "Enter", a single character stays itself.
func shortcutKey(k string) string {
	if k == "space" {
		return " "
	}
	for _, n := range namedKeys {
		if strings.EqualFold(n.web, k) {
			return n.web
		}
	}
	return normalizeKey(k)
}

// pollShortcuts answers the handlers whose keys were pressed this frame. While
// typing, only a chord with the primary modifier is live: a plain key belongs
// to the field.
func pollShortcuts(gtx C, list []extension.Shortcut, typing bool) []extension.Handler {
	type bound struct {
		c combo
		h extension.Handler
	}
	var live []bound
	var filters []event.Filter
	for _, sc := range list {
		c, ok := parseShortcut(sc.Keys)
		if !ok || (typing && !c.Mod) {
			continue
		}
		live = append(live, bound{c, sc.Handler})
		for _, f := range comboFilters(c) {
			filters = append(filters, f)
		}
	}
	if len(filters) == 0 {
		return nil
	}
	var fired []extension.Handler
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return fired
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		state := keyStateOf(e)
		for _, b := range live {
			if matchesCombo(state, b.c) {
				fired = append(fired, b.h)
				break
			}
		}
	}
}
