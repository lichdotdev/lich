package main

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/omartelo/lich/native/i18n"
)

// The keyboard shortcuts of frontend/src/lib/hotkeys.ts: the actions, their
// default chords and the pure rules over them. The stored value is the web's
// own (hotkeys.bindings), so the two windows share their bindings.

const hotkeysSettingKey = "hotkeys.bindings"

// hotkeyUnassigned is what a chord bound to nothing reads as, in the
// interface language localizeHotkeys last put it in.
var hotkeyUnassigned = i18n.T("hotkeys.unassigned")

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

// hotkeyGroups hold each heading's catalog key; label is its text in the
// interface language localizeHotkeys last put it in (English at start, since
// the tables are built before the window reads the language), for the
// screens that read the field.
var hotkeyGroups = labelGroups([]hotkeyGroupHeading{
	{id: hotkeyGroupSessions, labelKey: "hotkeys.group.sessions"},
	{id: hotkeyGroupView, labelKey: "hotkeys.group.view"},
	{id: hotkeyGroupApp, labelKey: "hotkeys.group.app"},
})

type hotkeyGroupHeading struct {
	id       hotkeyGroup
	labelKey string
	label    string
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

// hotkeyAction is a bindable action. labelKey is its catalog key; label is
// the text in the interface language localizeHotkeys last put it in.
type hotkeyAction struct {
	id       hotkeyID
	labelKey string
	label    string
	group    hotkeyGroup
	combo    combo
}

func shiftChord(key string) combo { return combo{Mod: true, Shift: true, Key: key} }

// hotkeyActions drives the defaults, the settings list and its search, in
// this order. Why each default is free is hotkeys.ts's to say.
var hotkeyActions = labelActions([]hotkeyAction{
	{id: hotkeyNewSession, labelKey: "hotkeys.action.newSession", group: hotkeyGroupSessions, combo: shiftChord("t")},
	{id: hotkeyNewWorktree, labelKey: "hotkeys.action.newWorktree", group: hotkeyGroupSessions, combo: shiftChord("b")},
	{id: hotkeyRenameSession, labelKey: "hotkeys.action.renameSession", group: hotkeyGroupSessions, combo: shiftChord("e")},
	{id: hotkeyCloseSession, labelKey: "hotkeys.action.closeSession", group: hotkeyGroupSessions, combo: shiftChord("x")},
	{id: hotkeyTogglePin, labelKey: "hotkeys.action.togglePin", group: hotkeyGroupSessions, combo: shiftChord("k")},
	{id: hotkeyOpenTerminal, labelKey: "hotkeys.action.openTerminal", group: hotkeyGroupSessions, combo: shiftChord("l")},
	{id: hotkeyDelegate, labelKey: "hotkeys.action.delegate", group: hotkeyGroupSessions, combo: shiftChord("h")},
	{id: hotkeyNextSession, labelKey: "hotkeys.action.nextSession", group: hotkeyGroupSessions, combo: shiftChord("ArrowDown")},
	{id: hotkeyPrevSession, labelKey: "hotkeys.action.prevSession", group: hotkeyGroupSessions, combo: shiftChord("ArrowUp")},
	{id: hotkeyNextWaitingSession, labelKey: "hotkeys.action.nextWaitingSession", group: hotkeyGroupSessions, combo: shiftChord("y")},
	{id: hotkeyFocusTerminal, labelKey: "hotkeys.action.focusTerminal", group: hotkeyGroupSessions, combo: shiftChord("Enter")},
	{id: hotkeyNextProject, labelKey: "hotkeys.action.nextProject", group: hotkeyGroupView, combo: shiftChord("ArrowRight")},
	{id: hotkeyPrevProject, labelKey: "hotkeys.action.prevProject", group: hotkeyGroupView, combo: shiftChord("ArrowLeft")},
	{id: hotkeyToggleSidebar, labelKey: "hotkeys.action.toggleSidebar", group: hotkeyGroupView, combo: shiftChord("s")},
	{id: hotkeyToggleDock, labelKey: "hotkeys.action.toggleDock", group: hotkeyGroupView, combo: shiftChord("d")},
	{id: hotkeySearchInFiles, labelKey: "hotkeys.action.searchInFiles", group: hotkeyGroupView, combo: comboUnassigned},
	{id: hotkeySplitBeside, labelKey: "hotkeys.action.splitBeside", group: hotkeyGroupView, combo: shiftChord("g")},
	{id: hotkeyOtherPane, labelKey: "hotkeys.action.otherPane", group: hotkeyGroupView, combo: shiftChord("f")},
	{id: hotkeyCommandPalette, labelKey: "hotkeys.action.commandPalette", group: hotkeyGroupApp, combo: combo{Mod: true, Key: "k"}},
	{id: hotkeySettings, labelKey: "hotkeys.action.settings", group: hotkeyGroupApp, combo: combo{Mod: true, Key: ","}},
	{id: hotkeyPulls, labelKey: "hotkeys.action.pulls", group: hotkeyGroupApp, combo: shiftChord("p")},
	{id: hotkeyShortcuts, labelKey: "hotkeys.action.shortcuts", group: hotkeyGroupApp, combo: combo{Mod: true, Key: "/"}},
	{id: hotkeyQuit, labelKey: "hotkeys.action.quit", group: hotkeyGroupApp, combo: comboUnassigned},
})

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
			return i18n.T(a.labelKey)
		}
	}
	return string(id)
}

// localizeHotkeys puts the group headings, the action labels and the
// unassigned chord's text in the interface language. The tables are read
// as fields by the Settings, palette and shortcuts screens, so they are
// renewed when the language changes (window.followLanguage) rather than
// resolved at every read.
func localizeHotkeys() {
	labelGroups(hotkeyGroups)
	labelActions(hotkeyActions)
	hotkeyUnassigned = i18n.T("hotkeys.unassigned")
}

func labelGroups(groups []hotkeyGroupHeading) []hotkeyGroupHeading {
	for i := range groups {
		groups[i].label = i18n.T(groups[i].labelKey)
	}
	return groups
}

func labelActions(actions []hotkeyAction) []hotkeyAction {
	for i := range actions {
		actions[i].label = i18n.T(actions[i].labelKey)
	}
	return actions
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

// terminalChordCosts is the catalog key of what the terminal side already
// spends a chord on, keyed by the combo's key. Ctrl+letter and nothing else:
// Ctrl+Shift+anything reaches no TUI at all.
var terminalChordCosts = map[string]string{
	"a":         "hotkeys.terminalCost.a",
	"c":         "hotkeys.terminalCost.c",
	"d":         "hotkeys.terminalCost.d",
	"e":         "hotkeys.terminalCost.e",
	"k":         "hotkeys.terminalCost.k",
	"l":         "hotkeys.terminalCost.l",
	"q":         "hotkeys.terminalCost.q",
	"r":         "hotkeys.terminalCost.r",
	"s":         "hotkeys.terminalCost.s",
	"u":         "hotkeys.terminalCost.u",
	"w":         "hotkeys.terminalCost.w",
	"z":         "hotkeys.terminalCost.z",
	"f":         "hotkeys.terminalCost.f",
	"v":         "hotkeys.terminalCost.v",
	"Backspace": "hotkeys.terminalCost.Backspace",
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
	return i18n.T(cost, i18n.Args{"key": formatKey(c.Key)})
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
		return i18n.T("hotkeys.unassigned")
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
