package main

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/lichdotdev/lich/native/i18n"
)

// The keyboard shortcuts: the actions, their default chords and the pure
// rules over them, stored under hotkeys.bindings.

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
	hotkeyNextSession        hotkeyID = "nextSession"
	hotkeyPrevSession        hotkeyID = "prevSession"
	hotkeyNextWaitingSession hotkeyID = "nextWaitingSession"
	hotkeyFocusTerminal      hotkeyID = "focusTerminal"
	hotkeyNextProject        hotkeyID = "nextProject"
	hotkeyPrevProject        hotkeyID = "prevProject"
	hotkeyToggleSidebar      hotkeyID = "toggleSidebar"
	hotkeyToggleDock         hotkeyID = "toggleDock"
	hotkeyToggleFullscreen   hotkeyID = "toggleFullscreen"
	hotkeySearchInFiles      hotkeyID = "searchInFiles"
	hotkeySplitBeside        hotkeyID = "splitBeside"
	hotkeyOtherPane          hotkeyID = "otherPane"
	hotkeySplitBelow         hotkeyID = "splitBelow"
	hotkeyZoomPane           hotkeyID = "zoomPane"
	hotkeyUnsplitPane        hotkeyID = "unsplitPane"
	hotkeyRotateSplit        hotkeyID = "rotateSplit"
	hotkeySettings           hotkeyID = "settings"
	hotkeyPulls              hotkeyID = "pulls"
	hotkeyToggleGit          hotkeyID = "toggleGit"
	hotkeyTour               hotkeyID = "tour"
	hotkeyShortcuts          hotkeyID = "shortcuts"
	hotkeyStartServices      hotkeyID = "startServices"
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

func modChord(key string) combo   { return combo{Mod: true, Key: key} }
func shiftChord(key string) combo { return combo{Mod: true, Shift: true, Key: key} }

// hotkeyActions drives the defaults, the settings list and its search, in
// this order. A letter default is Ctrl+letter, which a focused terminal takes
// as Ctrl+Shift+letter (hotkeyFilters); the rest are one chord everywhere.
// Ctrl+Shift+letter is never a default of its own when Ctrl+letter is one,
// and A, C, V, X, Z and Y stay with text editing, Q with XON, S with the
// editor's save and U with IBus, whose Ctrl+Shift+U starts Unicode entry.
// None is Ctrl+Alt: on Windows that is AltGr, which types text.
var hotkeyActions = labelActions([]hotkeyAction{
	{id: hotkeyNewSession, labelKey: "hotkeys.action.newSession", group: hotkeyGroupSessions, combo: modChord("n")},
	{id: hotkeyNewWorktree, labelKey: "hotkeys.action.newWorktree", group: hotkeyGroupSessions, combo: modChord("b")},
	{id: hotkeyRenameSession, labelKey: "hotkeys.action.renameSession", group: hotkeyGroupSessions, combo: comboUnassigned},
	{id: hotkeyCloseSession, labelKey: "hotkeys.action.closeSession", group: hotkeyGroupSessions, combo: modChord("w")},
	{id: hotkeyTogglePin, labelKey: "hotkeys.action.togglePin", group: hotkeyGroupSessions, combo: comboUnassigned},
	{id: hotkeyOpenTerminal, labelKey: "hotkeys.action.openTerminal", group: hotkeyGroupSessions, combo: modChord("t")},
	{id: hotkeyNextSession, labelKey: "hotkeys.action.nextSession", group: hotkeyGroupSessions, combo: modChord("Tab")},
	{id: hotkeyPrevSession, labelKey: "hotkeys.action.prevSession", group: hotkeyGroupSessions, combo: shiftChord("Tab")},
	{id: hotkeyNextWaitingSession, labelKey: "hotkeys.action.nextWaitingSession", group: hotkeyGroupSessions, combo: modChord("i")},
	{id: hotkeyFocusTerminal, labelKey: "hotkeys.action.focusTerminal", group: hotkeyGroupSessions, combo: comboUnassigned},
	{id: hotkeyNextProject, labelKey: "hotkeys.action.nextProject", group: hotkeyGroupView, combo: modChord("PageDown")},
	{id: hotkeyPrevProject, labelKey: "hotkeys.action.prevProject", group: hotkeyGroupView, combo: modChord("PageUp")},
	// S is the editor's save, so the sidebar keeps Shift: Ctrl+S is no
	// window action, and Ctrl+Shift+S is the chord everywhere.
	{id: hotkeyToggleSidebar, labelKey: "hotkeys.action.toggleSidebar", group: hotkeyGroupView, combo: shiftChord("s")},
	{id: hotkeyToggleDock, labelKey: "hotkeys.action.toggleDock", group: hotkeyGroupView, combo: modChord("j")},
	{id: hotkeyToggleGit, labelKey: "hotkeys.action.toggleGit", group: hotkeyGroupView, combo: modChord("g")},
	{id: hotkeyToggleFullscreen, labelKey: "hotkeys.action.toggleFullscreen", group: hotkeyGroupView, combo: combo{Key: "F11"}},
	{id: hotkeySearchInFiles, labelKey: "hotkeys.action.searchInFiles", group: hotkeyGroupView, combo: shiftChord("f")},
	{id: hotkeySplitBeside, labelKey: "hotkeys.action.splitBeside", group: hotkeyGroupView, combo: modChord("d")},
	{id: hotkeySplitBelow, labelKey: "hotkeys.action.splitBelow", group: hotkeyGroupView, combo: modChord("e")},
	{id: hotkeyOtherPane, labelKey: "hotkeys.action.otherPane", group: hotkeyGroupView, combo: modChord("o")},
	{id: hotkeyZoomPane, labelKey: "hotkeys.action.zoomPane", group: hotkeyGroupView, combo: shiftChord("Enter")},
	{id: hotkeyUnsplitPane, labelKey: "hotkeys.action.unsplitPane", group: hotkeyGroupView, combo: comboUnassigned},
	{id: hotkeyRotateSplit, labelKey: "hotkeys.action.rotateSplit", group: hotkeyGroupView, combo: comboUnassigned},
	{id: hotkeyCommandPalette, labelKey: "hotkeys.action.commandPalette", group: hotkeyGroupApp, combo: modChord("p")},
	{id: hotkeySettings, labelKey: "hotkeys.action.settings", group: hotkeyGroupApp, combo: modChord(",")},
	{id: hotkeyPulls, labelKey: "hotkeys.action.pulls", group: hotkeyGroupApp, combo: comboUnassigned},
	{id: hotkeyTour, labelKey: "hotkeys.action.tour", group: hotkeyGroupApp, combo: comboUnassigned},
	// F1, not Ctrl+/: in a terminal Ctrl+/ is 0x1f, the undo of readline and
	// Claude Code, and Codex's side conversation.
	{id: hotkeyShortcuts, labelKey: "hotkeys.action.shortcuts", group: hotkeyGroupApp, combo: combo{Key: "F1"}},
	{id: hotkeyStartServices, labelKey: "hotkeys.action.startServices", group: hotkeyGroupApp, combo: comboUnassigned},
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

// hotkeyConflicts reports, for every action sharing a chord with another,
// the other actions holding it, in hotkeyActions order. Ctrl+letter shares
// Ctrl+Shift+letter, which a focused terminal takes for it. Recording a taken
// chord is allowed; both rows say so.
func hotkeyConflicts(b hotkeyBindings) map[hotkeyID][]hotkeyID {
	conflicts := map[hotkeyID][]hotkeyID{}
	for _, a := range hotkeyActions {
		for _, other := range hotkeyActions {
			if other.id != a.id && sharesChord(b[a.id], b[other.id]) {
				conflicts[a.id] = append(conflicts[a.id], other.id)
			}
		}
	}
	return conflicts
}

func sharesChord(x, y combo) bool {
	if x.Key == "" || y.Key == "" {
		return false
	}
	return x == y || shownCombo(x, false) == shownCombo(y, false)
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
	// Command+Tab is the system's app switcher: Mod+Tab is Control there.
	mod := "⌘"
	if c.Key == "Tab" {
		mod = "⌃"
	}
	var parts []string
	for _, m := range []struct {
		on        bool
		mac, text string
	}{{c.Mod, mod, "Ctrl"}, {c.Shift, "⇧", "Shift"}, {c.Alt, "⌥", "Alt"}} {
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

// inTerminalHint is the line under a Settings row naming the chord a focused
// terminal takes for c, "" when that is c itself.
func inTerminalHint(c combo, goos string) string {
	t, ok := inTerminalCombo(c, goos)
	if !ok {
		return ""
	}
	return i18n.T("hotkeys.inTerminal", i18n.Args{"keys": formatCombo(t, false)})
}

// shownCombo is the chord a list that teaches the keys spells for c: the one
// that answers whatever holds the keyboard.
func shownCombo(c combo, isMac bool) combo {
	if !isMac && spendsControlCode(c) {
		c.Shift = true
	}
	return c
}

// comboOf reads a decoded JSON value as a combo: all three flags booleans
// and the key a string, or nothing.
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

// stringifyHotkeys is the stored value: every action's chord.
func stringifyHotkeys(b hotkeyBindings) (string, error) {
	out, err := json.Marshal(b)
	return string(out), err
}
