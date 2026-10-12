package main

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// The keyboard shortcuts of frontend/src/lib/hotkeys.ts: the actions, their
// default chords and the pure rules over them. The stored value is the web's
// own (hotkeys.bindings), so the two windows share their bindings.

// ponytail: English until the native window has i18n (hotkeys.group,
// hotkeys.action, hotkeys.terminalCost, hotkeys.unassigned).
const (
	hotkeysSettingKey = "hotkeys.bindings"
	hotkeyUnassigned  = "Unassigned"
	hotkeyKeyPlaceTag = "{key}"
)

type hotkeyID string

const (
	hotkeyCommandPalette     hotkeyID = "commandPalette"
	hotkeyQuit               hotkeyID = "quit"
	hotkeyNewSession         hotkeyID = "newSession"
	hotkeyNewWorktree        hotkeyID = "newWorktree"
	hotkeyRenameSession      hotkeyID = "renameSession"
	hotkeyCloseSession       hotkeyID = "closeSession"
	hotkeyTogglePin          hotkeyID = "togglePin"
	hotkeyOpenTerminal       hotkeyID = "openTerminal"
	hotkeyDelegate           hotkeyID = "delegate"
	hotkeyNextSession        hotkeyID = "nextSession"
	hotkeyPrevSession        hotkeyID = "prevSession"
	hotkeyNextWaitingSession hotkeyID = "nextWaitingSession"
	hotkeyFocusTerminal      hotkeyID = "focusTerminal"
	hotkeyNextProject        hotkeyID = "nextProject"
	hotkeyPrevProject        hotkeyID = "prevProject"
	hotkeyToggleSidebar      hotkeyID = "toggleSidebar"
	hotkeyToggleDock         hotkeyID = "toggleDock"
	hotkeySearchInFiles      hotkeyID = "searchInFiles"
	hotkeySplitBeside        hotkeyID = "splitBeside"
	hotkeyOtherPane          hotkeyID = "otherPane"
	hotkeySettings           hotkeyID = "settings"
	hotkeyPulls              hotkeyID = "pulls"
	hotkeyShortcuts          hotkeyID = "shortcuts"
)

// hotkeyGroup is which part of the app an action acts on.
type hotkeyGroup string

const (
	hotkeyGroupSessions hotkeyGroup = "sessions"
	hotkeyGroupView     hotkeyGroup = "view"
	hotkeyGroupApp      hotkeyGroup = "app"
)

var hotkeyGroups = []struct {
	id    hotkeyGroup
	label string
}{
	{hotkeyGroupSessions, "Sessions"},
	{hotkeyGroupView, "View"},
	{hotkeyGroupApp, "App"},
}

// combo is a chord. Mod is the platform's primary modifier, Ctrl or Cmd, so
// one stored value serves both. An empty Key is an action bound to nothing.
type combo struct {
	Mod   bool   `json:"mod"`
	Shift bool   `json:"shift"`
	Alt   bool   `json:"alt"`
	Key   string `json:"key"`
}

// comboUnassigned never matches: no keypress produces an empty key.
var comboUnassigned = combo{}

type hotkeyAction struct {
	id    hotkeyID
	label string
	group hotkeyGroup
	combo combo
}

func shiftChord(key string) combo { return combo{Mod: true, Shift: true, Key: key} }

// hotkeyActions drives the defaults, the settings list and its search, in
// this order. Why each default is free is hotkeys.ts's to say.
var hotkeyActions = []hotkeyAction{
	{hotkeyNewSession, "New session", hotkeyGroupSessions, shiftChord("t")},
	{hotkeyNewWorktree, "New worktree session", hotkeyGroupSessions, shiftChord("b")},
	{hotkeyRenameSession, "Rename the active session", hotkeyGroupSessions, shiftChord("e")},
	{hotkeyCloseSession, "Close the active session", hotkeyGroupSessions, shiftChord("x")},
	{hotkeyTogglePin, "Pin or unpin the active session", hotkeyGroupSessions, shiftChord("k")},
	{hotkeyOpenTerminal, "Open a terminal in the session's directory", hotkeyGroupSessions, shiftChord("l")},
	{hotkeyDelegate, "Delegate to another session", hotkeyGroupSessions, shiftChord("h")},
	{hotkeyNextSession, "Next session", hotkeyGroupSessions, shiftChord("ArrowDown")},
	{hotkeyPrevSession, "Previous session", hotkeyGroupSessions, shiftChord("ArrowUp")},
	{hotkeyNextWaitingSession, "Next session waiting for input", hotkeyGroupSessions, shiftChord("y")},
	{hotkeyFocusTerminal, "Focus the session terminal", hotkeyGroupSessions, shiftChord("Enter")},
	{hotkeyNextProject, "Next project", hotkeyGroupView, shiftChord("ArrowRight")},
	{hotkeyPrevProject, "Previous project", hotkeyGroupView, shiftChord("ArrowLeft")},
	{hotkeyToggleSidebar, "Toggle the session sidebar", hotkeyGroupView, shiftChord("s")},
	{hotkeyToggleDock, "Toggle the right dock", hotkeyGroupView, shiftChord("d")},
	{hotkeySearchInFiles, "Search in files", hotkeyGroupView, comboUnassigned},
	{hotkeySplitBeside, "Show another session beside this one", hotkeyGroupView, shiftChord("g")},
	{hotkeyOtherPane, "Focus the next pane", hotkeyGroupView, shiftChord("f")},
	{hotkeyCommandPalette, "Command palette", hotkeyGroupApp, combo{Mod: true, Key: "k"}},
	{hotkeySettings, "Settings", hotkeyGroupApp, combo{Mod: true, Key: ","}},
	{hotkeyPulls, "Pull requests", hotkeyGroupApp, shiftChord("p")},
	{hotkeyShortcuts, "Keyboard shortcuts", hotkeyGroupApp, combo{Mod: true, Key: "/"}},
	{hotkeyQuit, "Quit lich", hotkeyGroupApp, comboUnassigned},
}

// hotkeyBindings is every action's chord.
type hotkeyBindings map[hotkeyID]combo

func defaultHotkeys() hotkeyBindings {
	b := make(hotkeyBindings, len(hotkeyActions))
	for _, a := range hotkeyActions {
		b[a.id] = a.combo
	}
	return b
}

func hotkeyLabel(id hotkeyID) string {
	for _, a := range hotkeyActions {
		if a.id == id {
			return a.label
		}
	}
	return string(id)
}

// keyState is the part of a key event the matcher reads, so tests pass plain
// values. Gio's key.Event carries no auto-repeat flag, so repeat is only ever
// set by a caller that knows better.
type keyState struct {
	ctrl, meta, shift, alt bool
	key                    string
	repeat                 bool
}

// normalizeKey folds "=" into "+" (one physical key) and lowercases a single
// character, so Shift does not change a combo's identity.
func normalizeKey(key string) string {
	if key == "=" {
		return "+"
	}
	if utf8.RuneCountInString(key) == 1 {
		return strings.ToLower(key)
	}
	return key
}

func matchesCombo(e keyState, c combo) bool {
	if e.repeat || c.Key == "" {
		return false
	}
	mod := e.ctrl || e.meta
	return mod == c.Mod && e.shift == c.Shift && e.alt == c.Alt && normalizeKey(e.key) == c.Key
}

// comboFromEvent builds a combo from a captured keypress. It reports false for
// a bare modifier and for a key with neither the primary modifier nor Alt: a
// bare "t" would fire while typing.
func comboFromEvent(e keyState) (combo, bool) {
	switch e.key {
	case "Control", "Meta", "Shift", "Alt", "AltGraph":
		return combo{}, false
	}
	mod := e.ctrl || e.meta
	if !mod && !e.alt {
		return combo{}, false
	}
	return combo{Mod: mod, Shift: e.shift, Alt: e.alt, Key: normalizeKey(e.key)}, true
}

// hotkeyConflicts reports, for every action sharing its chord with another,
// the other actions holding it, in hotkeyActions order. Recording a taken
// chord is allowed; both rows say so.
func hotkeyConflicts(b hotkeyBindings) map[hotkeyID][]hotkeyID {
	byCombo := map[combo][]hotkeyID{}
	for _, a := range hotkeyActions {
		if c := b[a.id]; c.Key != "" {
			byCombo[c] = append(byCombo[c], a.id)
		}
	}
	conflicts := map[hotkeyID][]hotkeyID{}
	for _, ids := range byCombo {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			for _, other := range ids {
				if other != id {
					conflicts[id] = append(conflicts[id], other)
				}
			}
		}
	}
	return conflicts
}

// terminalChordCosts is what the terminal side already spends a chord on,
// keyed by the combo's key. Ctrl+letter and nothing else: Ctrl+Shift+anything
// reaches no TUI at all.
var terminalChordCosts = map[string]string{
	"a":         "Ctrl+{key} is the shell's move to the start of the line; sessions will no longer see it.",
	"c":         "Ctrl+{key} is the shell's interrupt; sessions will no longer see it.",
	"d":         "Ctrl+{key} is the shell's end of input; sessions will no longer see it.",
	"e":         "Ctrl+{key} is the shell's move to the end of the line; sessions will no longer see it.",
	"k":         "Ctrl+{key} is the shell's kill to the end of the line; sessions will no longer see it.",
	"l":         "Ctrl+{key} is the shell's clear screen; sessions will no longer see it.",
	"q":         "Ctrl+{key} is the shell's resume output; sessions will no longer see it.",
	"r":         "Ctrl+{key} is the shell's history search; sessions will no longer see it.",
	"s":         "Ctrl+{key} is the shell's stop output; sessions will no longer see it.",
	"u":         "Ctrl+{key} is the shell's kill to the start of the line; sessions will no longer see it.",
	"w":         "Ctrl+{key} is the shell's erase word; sessions will no longer see it.",
	"z":         "Ctrl+{key} is the shell's suspend; sessions will no longer see it.",
	"f":         "Ctrl+{key} is the session terminal's own search; sessions will no longer see it.",
	"v":         "Ctrl+{key} is the image paste lich sends the agent; sessions will no longer see it.",
	"Backspace": "Ctrl+{key} is the erase word lich sends the agent; sessions will no longer see it.",
}

// terminalCost is the line the recorder shows for a chord the terminal side
// already spends, "" for one the PTY never sees anyway. The chord is spelled
// Ctrl even on macOS: what a TUI reads is the control code.
func terminalCost(c combo) string {
	if !c.Mod || c.Shift || c.Alt {
		return ""
	}
	cost, ok := terminalChordCosts[c.Key]
	if !ok {
		return ""
	}
	return strings.ReplaceAll(cost, hotkeyKeyPlaceTag, formatKey(c.Key))
}

func formatKey(key string) string {
	switch {
	case key == " ":
		return "Space"
	case strings.HasPrefix(key, "Arrow"):
		return strings.TrimPrefix(key, "Arrow")
	case utf8.RuneCountInString(key) == 1:
		return strings.ToUpper(key)
	}
	return key
}

func formatCombo(c combo, isMac bool) string {
	if c.Key == "" {
		return hotkeyUnassigned
	}
	var parts []string
	for _, m := range []struct {
		on        bool
		mac, text string
	}{{c.Mod, "⌘", "Ctrl"}, {c.Shift, "⇧", "Shift"}, {c.Alt, "⌥", "Alt"}} {
		switch {
		case !m.on:
		case isMac:
			parts = append(parts, m.mac)
		default:
			parts = append(parts, m.text)
		}
	}
	parts = append(parts, formatKey(c.Key))
	if isMac {
		return strings.Join(parts, "")
	}
	return strings.Join(parts, "+")
}

// comboOf is hotkeys.ts isCombo over a decoded JSON value: all three flags
// booleans and the key a string, or nothing.
func comboOf(value any) (combo, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return combo{}, false
	}
	mod, okMod := m["mod"].(bool)
	shift, okShift := m["shift"].(bool)
	alt, okAlt := m["alt"].(bool)
	key, okKey := m["key"].(string)
	if !okMod || !okShift || !okAlt || !okKey {
		return combo{}, false
	}
	return combo{Mod: mod, Shift: shift, Alt: alt, Key: key}, true
}

// mergeHotkeys layers validated overrides over the defaults, dropping
// anything malformed. A stored key is normalized on the way in, and
// modifiers with no key fold into the one unassigned shape.
func mergeHotkeys(overrides any) hotkeyBindings {
	result := defaultHotkeys()
	stored, ok := overrides.(map[string]any)
	if !ok {
		return result
	}
	for _, a := range hotkeyActions {
		c, ok := comboOf(stored[string(a.id)])
		if !ok {
			continue
		}
		if c.Key == "" {
			result[a.id] = comboUnassigned
			continue
		}
		c.Key = normalizeKey(c.Key)
		result[a.id] = c
	}
	return result
}

// parseHotkeys reads one stored value. A persisted binding must never break
// a launch, so anything that does not parse reads as the defaults.
func parseHotkeys(raw string) hotkeyBindings {
	if raw == "" {
		return defaultHotkeys()
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return defaultHotkeys()
	}
	return mergeHotkeys(decoded)
}

// stringifyHotkeys is the stored value: every action's chord, as the web
// writes it.
func stringifyHotkeys(b hotkeyBindings) (string, error) {
	out, err := json.Marshal(b)
	return string(out), err
}
