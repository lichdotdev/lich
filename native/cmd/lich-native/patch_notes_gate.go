package main

import (
	"log"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
)

// patchNotesSeenKey is the version whose notes were last shown, a global
// setting.
const patchNotesSeenKey = "update.patchNotesSeen"

type patchNotesAction uint8

const (
	patchNotesNone patchNotesAction = iota
	// patchNotesRecord remembers the version without a popup: a fresh
	// install is not greeted with notes on its first launch.
	patchNotesRecord
	patchNotesShow
)

// decidePatchNotes says what to do with a build's notes: a build the changelog
// has no section for shows nothing, and a version is shown once.
func decidePatchNotes(notes lichclient.PatchNotes, lastSeen string) patchNotesAction {
	switch {
	case notes.Version == "" || len(notes.Groups) == 0:
		return patchNotesNone
	case lastSeen == "":
		return patchNotesRecord
	case lastSeen == notes.Version:
		return patchNotesNone
	}
	return patchNotesShow
}

// startupPatchNotes makes one decision at start.
func (s *settingsScreen) startupPatchNotes() {
	seen, err := s.client.GetSetting(s.ctx, patchNotesSeenKey, "")
	if err != nil {
		log.Printf("patch notes: last seen: %v", err)
		return
	}
	notes, err := s.client.PatchNotes(s.ctx, string(i18n.Current()))
	if err != nil {
		log.Printf("patch notes: %v", err)
		return
	}
	switch decidePatchNotes(notes, seen) {
	case patchNotesRecord:
		s.recordPatchNotes(notes.Version)
	case patchNotesShow:
		u := &s.updates
		u.mu.Lock()
		u.notes, u.startupNotes = &notes, true
		u.mu.Unlock()
		s.invalidate()
	}
}

func (s *settingsScreen) recordPatchNotes(version string) {
	go func() {
		if err := s.client.SetSetting(s.ctx, patchNotesSeenKey, "", version); err != nil {
			log.Printf("patch notes: record %s: %v", version, err)
		}
	}()
}
