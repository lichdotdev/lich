package relay

import (
	"fmt"
	"strings"
	"time"

	"github.com/lichdotdev/lich/internal/prompt"
)

// The briefing lich hands a provider at spawn, the notes it composes, and the
// keystrokes that put a note at a terminal's prompt.

// SpawnBriefing is what a provider is told about lich when lich starts it: two
// sentences appended to its system prompt, for the providers whose command line
// accepts one (internal/terminal, briefingFlags).
//
// It exists because the agent's own harness offers subagents of its own, and
// those are described to it at length, in its system prompt, from its first
// turn. lich's tools are described in a server block far below that — so an
// agent asked to "fan this out across worktrees" reaches for the subagent it was
// told about, and the user watches work they meant to supervise disappear into a
// process they cannot open, on a checkout that does not exist. Reported twice by
// the same user before this text existed.
//
// It draws the line rather than forbidding anything: a subagent is still the
// right tool for a throwaway read. What it fixes is the one case the agent
// cannot get right on its own, because nothing in its prompt says lich sessions
// are visible and steerable and subagents are not.
//
// lang is the session's prompt language; every locale keeps to the quoting
// rule below (see prompt.Catalog).
//
// hasTools is whether this provider was handed lich's MCP server at spawn. The
// command line works everywhere and is named for the ones that were not:
// naming a tool a session does not have is worse than naming the command.
//
// The example spells its placeholders in capitals and quotes with ' rather than
// the usual <angle brackets> and ": this string is passed as one argv entry, and
// a provider shipped as a .cmd is spawned through `cmd.exe /c`
// (internal/terminal, wrapArgv). Windows escapes a double quote as \" for
// CommandLineToArgvW, cmd.exe does not read that as an escape, and the < and >
// left outside quotes by it are redirection.
//
// route is where this spawn's own subagents run (SubagentRoute), which decides
// what the agent is sent to for fanning work out.
//
// The artifact sentence is here, and not only in lich's MCP instructions,
// because it competes with the harness's own way of publishing a page: Claude
// Code's system prompt sends the agent to a hosted artifact, and Codex and Kiro
// CLI never read the MCP instructions at all. It names the variable rather than
// the folder, which the MCP instructions spell out, so the argv stays the same
// for every session. No single quote either: Codex takes this inside one
// (internal/terminal, codexBriefingArgs).
func SpawnBriefing(lang prompt.Lang, hasTools bool, route SubagentRoute) string {
	text := prompt.For(lang)
	command, opener, reply, collect := text.BriefingCommandCLI, briefingOpenerCLI, briefingReplyCLI, briefingCollectCLI
	if hasTools {
		command, opener, reply, collect = text.BriefingCommandTools, briefingOpenerTool, ToolReply, ToolCollect
	}
	artifacts := fmt.Sprintf(text.BriefingEnvelope, reply, collect) + fmt.Sprintf(text.BriefingArtifacts, opener)
	switch route {
	case RouteCards:
		return text.BriefingIntro + text.BriefingCards + command + artifacts
	case RouteNative:
		return text.BriefingIntro + text.BriefingNative + artifacts
	default:
		return text.BriefingIntro + text.BriefingSessions + command + artifacts
	}
}

// How the briefing says to open a page, reply and collect, by the same rule as
// the command: the tool for a session handed lich's MCP server, the CLI for the
// rest.
const (
	briefingOpenerTool = "browser_open"
	briefingOpenerCLI  = "`lich browser open PATH`"
	briefingReplyCLI   = "`lich reply TICKET ANSWER`"
	briefingCollectCLI = "`lich wait`"
)

// BriefingEnv carries the spawn briefing to a provider whose agent config prints
// it from a hook (Kiro CLI).
const BriefingEnv = "LICH_BRIEFING"

// SubagentRoute is where a spawned session's own subagents run, as far as its
// briefing is concerned.
type SubagentRoute int

const (
	// RouteSessions is every session whose subagents stay inside its harness and
	// that may open lich sessions to fan out.
	RouteSessions SubagentRoute = iota
	// RouteCards is a session whose provider's lich-plugin hook asks /subagent,
	// which runs a subagent as one of those cards (an exploration type the
	// provider keeps native stays inside the session). There the agent's own
	// Agent tool is the better route (in the background, its report back on its
	// own, stopped with TaskStop), so the briefing sends it there and keeps
	// lich's own route for what a subagent cannot be.
	RouteCards
	// RouteNative is a card whose own subagents stay native, because
	// it runs as deep as lich nests cards. lich's server instructions still say
	// a subagent becomes a card in a session with the plugin, so the briefing
	// says it does not here, and keeps the fan-out under this card rather than
	// sending it to new sessions, which would nest cards past the limit by
	// another door.
	RouteNative
)

// pickTicketNudge is what a worker is told at its own prompt after a turn that
// ended with none of its errands answered. Those errands are over by the time it
// reads this — every one the turn could have been went home unanswered, which is
// the same true thing about each of them — so they are named as history and not
// as somewhere to reply: a ticket the relay has closed answers "unknown ticket",
// and a note that invites that is worse than no note. What it asks for is the
// next answer, which is the one that can still name its ticket.
func pickTicketNudge(lang prompt.Lang, count int, errands string) string {
	return noticeEnvelope(TopicUnanswered, fmt.Sprintf(prompt.For(lang).PickTicketNudge, count, errands), "").String()
}

// compactReminder is what a worker is told after its conversation was
// compacted while it still owed answers: each errand with the reply that
// closes it, the same lines a refused ticketless answer lists.
func compactReminder(lang prompt.Lang, count int, errands string) string {
	text := prompt.For(lang)
	return noticeEnvelope(TopicCompacted, fmt.Sprintf(text.Pick(count, text.CompactReminder), count, errands), "").String()
}

// QuotedList words a list of names (session labels, folders) for a message:
// each one quoted, so a name with a comma or a trailing space still reads as
// one.
func QuotedList(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	return strings.Join(quoted, ", ")
}

// paste wraps text in bracketed paste, which is how a multi-line message
// reaches a TUI prompt as one prompt instead of as one submission per newline.
// Every provider lich spawns runs a TUI that enables bracketed paste; one that
// did not would read the newlines as submissions.
//
// It does not submit. See submitDelay.
func paste(text string) string {
	return "\x1b[200~" + text + "\x1b[201~"
}

// submit is the Enter that sends what paste put at the prompt.
const submit = "\r"

// defaultSubmitDelay is how long a target's PTY has to stay quiet before the
// relay presses the Enter that sends what it pasted (see awaitSettled).
//
// Everything else lich pastes into a prompt is left for the user to send, so
// this is the only place that presses Enter itself — and a carriage return
// arriving while the TUI is still taking the paste in is swallowed. Claude Code
// collapses a multi-line paste into a "[Pasted text #2 +7 lines]" placeholder
// and the Enter goes into building the placeholder rather than sending it;
// Codex, on a terminal that gave it no paste event, spends 120ms deciding
// whether an Enter belongs inside the burst. Both were the same bug on screen:
// the message sitting unsent at the target's prompt, seen only when someone
// opened that session by hand.
//
// It has to outlast the longest of those windows, and it is what a settled
// terminal is measured against, so it is the whole instrument in both roles.
const defaultSubmitDelay = 150 * time.Millisecond

// sanitize strips the control characters that would either break out of the
// bracketed paste framing or drive the target's terminal, keeping the
// whitespace a prompt legitimately contains. ESC is the one that matters: text
// carrying "\x1b[201~" would end the paste early and leave the rest of itself
// running as keystrokes.
func sanitize(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
}
