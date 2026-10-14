package main

import (
	"log"

	"github.com/omartelo/lich/native/lichclient"
)

// patchNotesSeenKey is the web's PATCH_NOTES_SEEN_KEY: the version whose
// notes were last shown, a global setting the web window shares.
const patchNotesSeenKey = "update.patchNotesSeen"

type patchNotesAction uint8

const (
	patchNotesNone patchNotesAction = iota
	// patchNotesRecord remembers the version without a popup: a fresh
	// install is not greeted with notes on its first launch.
	patchNotesRecord
	patchNotesShow
)

// decidePatchNotes is patch-notes-gate.ts's: a build the changelog has no
// section for shows nothing, and a version is shown once.
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

// startupPatchNotes is PatchNotesGate: one decision at start.
func (s *settingsScreen) startupPatchNotes() {
	seen, err := s.client.GetSetting(s.ctx, patchNotesSeenKey, "")
	if err != nil {
		log.Printf("patch notes: last seen: %v", err)
		return
	}
	notes, err := s.client.PatchNotes(s.ctx)
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
