package main

import (
	"log"
	"sync"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// hotkeyEditor is Settings › Hotkeys' state and the bindings the window
// obeys. The bindings are read at connect and written at once on every
// change, under the key the web writes, global scope: a rebind here is a
// rebind there.
type hotkeyEditor struct {
	bindings hotkeyBindings // nil until loaded: the defaults
	rows     map[hotkeyID]*hotkeyRow
	// recording is the action whose capture button is waiting for a chord,
	// "" for none. While it is set no hotkey fires, so the chord being
	// recorded is not also run.
	recording hotkeyID

	saveMu  sync.Mutex
	pending string // the newest value not written yet, "" for none
	writing bool   // a goroutine is draining pending
}

// hotkeyRow is one action's controls.
type hotkeyRow struct {
	hover, capture, unassign, reset widget.Clickable
	unassignTip, resetTip           ui.Tooltip
	tag                             bool // the capture button's key event tag
	// held is whether the capture button has had the focus since the
	// recording began: losing it after that is the blur that cancels.
	held bool
}

func (h *hotkeyEditor) current() hotkeyBindings {
	if h.bindings == nil {
		h.bindings = defaultHotkeys()
	}
	return h.bindings
}

func (h *hotkeyEditor) row(id hotkeyID) *hotkeyRow {
	r, ok := h.rows[id]
	if !ok {
		if h.rows == nil {
			h.rows = map[hotkeyID]*hotkeyRow{}
		}
		r = new(hotkeyRow)
		h.rows[id] = r
	}
	return r
}

// loadHotkeys reads the stored bindings before the first frame.
func (s *settingsScreen) loadHotkeys() error {
	raw, err := s.client.GetSetting(s.ctx, hotkeysSettingKey, "")
	s.hotkeys.bindings = parseHotkeys(raw)
	return err
}

// setHotkey stores id's chord, in effect from the next frame.
func (s *settingsScreen) setHotkey(id hotkeyID, c combo) {
	next := s.hotkeys.current()
	next[id] = c
	raw, err := stringifyHotkeys(next)
	if err != nil {
		log.Printf("settings: hotkeys: %v", err)
		return
	}
	s.hotkeys.save(raw, func(raw string) {
		if err := s.client.SetSetting(s.ctx, hotkeysSettingKey, "", raw); err != nil {
			log.Printf("settings: save hotkeys: %v", err)
		}
	})
}

// save writes raw off the frame. Writes go one at a time and a value
// superseded before its turn is skipped, so two quick rebinds cannot land out
// of order and leave the older one stored.
func (h *hotkeyEditor) save(raw string, write func(string)) {
	h.saveMu.Lock()
	h.pending = raw
	if h.writing {
		h.saveMu.Unlock()
		return
	}
	h.writing = true
	h.saveMu.Unlock()
	go func() {
		for {
			h.saveMu.Lock()
			next := h.pending
			h.pending = ""
			if next == "" {
				h.writing = false
				h.saveMu.Unlock()
				return
			}
			h.saveMu.Unlock()
			write(next)
		}
	}()
}

func (s *settingsScreen) resetHotkey(id hotkeyID) { s.setHotkey(id, defaultHotkeys()[id]) }

const allModifiers = key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper

// captureFilters are the keys a recording button takes: every key but the
// system ones, which Tab is, by name.
func captureFilters(tag event.Tag) []event.Filter {
	return []event.Filter{
		key.FocusFilter{Target: tag},
		key.Filter{Focus: tag, Optional: allModifiers},
		key.Filter{Focus: tag, Name: key.NameTab, Optional: allModifiers},
	}
}

// capture reads a recording button's frame: a chord stores it, Escape or the
// focus leaving cancels. Every key is taken while it records, so none of them
// is also the shortcut it is.
func (s *settingsScreen) capture(gtx C, id hotkeyID, r *hotkeyRow) {
	h := &s.hotkeys
	for {
		ev, ok := gtx.Event(captureFilters(&r.tag)...)
		if !ok {
			break
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		if e.Name == key.NameEscape {
			h.recording = ""
			return
		}
		if c, ok := comboFromEvent(keyStateOf(e)); ok {
			s.setHotkey(id, c)
			h.recording = ""
			return
		}
	}
	switch {
	case gtx.Focused(&r.tag):
		r.held = true
	case !r.held:
		gtx.Execute(key.FocusCmd{Tag: &r.tag})
	default:
		h.recording = ""
	}
}

// startRecording is a click on the capture button. A second click on the
// recording one is its own blur: it keeps recording.
func (h *hotkeyEditor) startRecording(id hotkeyID) {
	if h.recording != id {
		h.recording = id
		h.row(id).held = false
	}
}

// hotkeyActionText and hotkeyGroupText are the action's and the group's
// names in the interface language, the catalog's hotkeys.action.<id> and
// hotkeys.group.<id>.
func hotkeyActionText(id hotkeyID) string { return i18n.T("hotkeys.action." + string(id)) }

func hotkeyGroupText(id hotkeyGroup) string { return i18n.T("hotkeys.group." + string(id)) }

func hotkeyLabels(ids []hotkeyID) []string {
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = hotkeyActionText(id)
	}
	return labels
}

// hotkeyEntries are the pane's searchable rows, one per action: the label and
// the part of the app it acts on, as settings-index.ts hotkeyEntries.
func hotkeyEntries() []settingEntry {
	entries := make([]settingEntry, len(hotkeyActions))
	for i, a := range hotkeyActions {
		entries[i] = settingEntry{
			titleKey: "hotkeys.action." + string(a.id),
			groupKey: "hotkeys.group." + string(a.group),
			key:      "hotkeys." + string(a.id),
		}
	}
	return entries
}
