package relay

import (
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/providers"
)

// A lich session has two names, and until this file they were two address
// spaces. The card carries a label the user chose ("docs"); Claude Code's peer
// roster carries the name lich passes at a fork ("skipo-3939") or the one a
// `/rename` gave it. Both name the same session, and nothing in an agent's world
// said so — so an agent handed one of them reached for whichever tool it had
// and, on the first real run, used both channels at once.
//
// The relay therefore answers to either. Deriving the roster name is how lich
// mints one, and RosterNameOf is how it reads one back: a session that renamed
// itself answers to the new name and to nothing lich could have derived, so the
// derivation is the fallback rather than the answer.

// rosterIDChars is how much of the session id trails the directory name. Four
// separates the handful of sessions one checkout holds and stays readable in a
// roster row.
const rosterIDChars = 4

// RosterNameOf is the name a session answers to in the peer roster now.
// recorded is what its agent has on record (terminal.AgentName) and wins
// outright: a `/rename` typed inside a session changes the name the harness
// answers to, and no derivation can see that. Falling back to the derived one
// covers every provider that is never handed a roster name.
//
// Claude Code with nothing recorded answers to no name lich can know: it is born
// without --name (terminal.nameArgs), names itself after its directory and a
// random suffix, and writes that name nowhere lich reads. Empty says so, where a
// derived name would be a second name for one session that its own roster
// never shows.
func RosterNameOf(kind, recorded, cwd, id string) string {
	if recorded = strings.TrimSpace(recorded); recorded != "" {
		return recorded
	}
	if kind == providers.Claude {
		return ""
	}
	return RosterName(cwd, id)
}

// RosterName builds the name lich gives a session at birth: the last element of
// its working directory, then four characters of its id. cwd is the session's
// own directory when it has one (a worktree) and its project's otherwise.
//
// Exported for the one caller outside this package that mints a name rather
// than resolving one: internal/terminal, which hands it to a Claude Code fork.
func RosterName(cwd, id string) string {
	dir := rosterDir(cwd)
	tail := make([]rune, 0, rosterIDChars)
	for _, r := range id {
		if len(tail) == rosterIDChars {
			break
		}
		if isRosterChar(r) {
			tail = append(tail, r)
		}
	}
	if len(tail) == 0 {
		return dir
	}
	return dir + "-" + string(tail)
}

// IsRosterName reports whether name has the shape RosterName gives a session
// born in cwd, whatever its id: how a Claude Code conversation lich opened is
// told from one somebody started by hand, after lich has lost the row
// (terminal.Conversations). A name the user typed into `/rename` matches only
// by coincidence.
func IsRosterName(name, cwd string) bool {
	tail, ok := strings.CutPrefix(name, rosterDir(cwd)+"-")
	if !ok || len(tail) != rosterIDChars {
		return false
	}
	for _, r := range tail {
		if !isRosterChar(r) {
			return false
		}
	}
	return true
}

func rosterDir(cwd string) string {
	dir := filepath.Base(strings.TrimRight(strings.ReplaceAll(cwd, "\\", "/"), "/"))
	// filepath.Base answers "." for an empty path and "/" for a bare root;
	// neither names anything, and Claude Code's own fallback is the app name.
	if dir == "" || dir == "." || dir == "/" {
		return "lich"
	}
	return dir
}

func isRosterChar(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
