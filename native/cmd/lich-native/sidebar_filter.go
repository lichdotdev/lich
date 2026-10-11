package main

import (
	"fmt"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
)

// sessionPhase is the filter chip a session sits under (session-filter.ts).
type sessionPhase string

const (
	phaseWaiting sessionPhase = "waiting"
	phaseRunning sessionPhase = "running"
	phaseUnread  sessionPhase = "unread"
	phaseIdle    sessionPhase = "idle"
)

// sessionPhases is the chips' drawn order: what blocks the user, what is
// moving, what is new, the rest.
var sessionPhases = []sessionPhase{phaseWaiting, phaseRunning, phaseUnread, phaseIdle}

// ponytail: English until the native window has i18n.
var (
	phaseLabels = map[sessionPhase]string{
		phaseWaiting: "waiting", phaseRunning: "running", phaseUnread: "unread", phaseIdle: "idle",
	}
	phaseJoiner            = " or "
	noMatchText            = "No sessions. The active session stays."
	noMatchQueryText       = "No sessions match “%s”. The active session stays."
	noMatchPhasesText      = "No %s sessions. The active session stays."
	noMatchPhasesQueryText = "No %s sessions match “%s”. The active session stays."
)

// phaseOf files a status under one chip. A read turn is idle: its ring
// already says nothing is happening and nothing is new. A session that never
// reports (Crush, Cursor CLI, a shell) is idle for the same reason.
func phaseOf(status string, unread bool) sessionPhase {
	switch status {
	case statusWaiting:
		return phaseWaiting
	case statusBusy, statusCompacting:
		return phaseRunning
	case statusDone:
		if unread {
			return phaseUnread
		}
	}
	return phaseIdle
}

// matchesQuery is command-palette.ts matchesQuery: every whitespace token of
// the query appears in haystack, case-insensitively.
func matchesQuery(haystack, query string) bool {
	hay := strings.ToLower(haystack)
	for _, token := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(hay, token) {
			return false
		}
	}
	return true
}

// queryHit matches a session's label and checkout, which is what makes typing
// a worktree name narrow to that checkout. The branch is left out on purpose,
// as on the web: the checkout path already gives most of it.
func queryHit(query, projectPath string) func(lichclient.Session) bool {
	return func(s lichclient.Session) bool {
		path := s.Path
		if path == "" {
			path = projectPath
		}
		return matchesQuery(s.Label+" "+path, query)
	}
}

// filterSessions narrows sessions to the ones the query and the picked phases
// (none picked narrows nothing) match, in stored order. The active session
// survives whatever the filter says: it is the identity of the terminal
// beside it, and the card being answered must not vanish as it turns running.
// matched says whether anything matched, which the kept active card hides.
func filterSessions(sessions []lichclient.Session, query, projectPath, activeID string,
	phases map[sessionPhase]bool, phaseOfSession func(id string) sessionPhase,
) (kept []lichclient.Session, matched bool) {
	blank := strings.TrimSpace(query) == ""
	if blank && len(phases) == 0 {
		return sessions, true
	}
	text := queryHit(query, projectPath)
	hit := func(s lichclient.Session) bool {
		return (blank || text(s)) && (len(phases) == 0 || phases[phaseOfSession(s.ID)])
	}
	kept = []lichclient.Session{}
	for _, s := range sessions {
		h := hit(s)
		matched = matched || h
		if h || s.ID == activeID {
			kept = append(kept, s)
		}
	}
	return kept, matched
}

// phaseCounts is the number beside each chip, counted against the query
// alone so a count says what picking that chip would add.
func phaseCounts(sessions []lichclient.Session, query, projectPath string,
	phaseOfSession func(id string) sessionPhase,
) map[sessionPhase]int {
	counts := map[sessionPhase]int{phaseWaiting: 0, phaseRunning: 0, phaseUnread: 0, phaseIdle: 0}
	text := queryHit(query, projectPath)
	blank := strings.TrimSpace(query) == ""
	for _, s := range sessions {
		if blank || text(s) {
			counts[phaseOfSession(s.ID)]++
		}
	}
	return counts
}

// noMatchNotice is the sentence under an empty filter, naming the picked
// phases in chip order whatever order they were clicked in.
func noMatchNotice(query string, phases map[sessionPhase]bool) string {
	trimmed := strings.TrimSpace(query)
	var picked []string
	for _, p := range sessionPhases {
		if phases[p] {
			picked = append(picked, phaseLabels[p])
		}
	}
	named := strings.Join(picked, phaseJoiner)
	switch {
	case named == "" && trimmed == "":
		return noMatchText
	case named == "":
		return fmt.Sprintf(noMatchQueryText, trimmed)
	case trimmed == "":
		return fmt.Sprintf(noMatchPhasesText, named)
	}
	return fmt.Sprintf(noMatchPhasesQueryText, named, trimmed)
}
