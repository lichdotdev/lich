package api

import (
	"fmt"
	"reflect"
	"strings"
)

// Event names one message of the /events stream; T is its payload (None for
// one that carries nothing).
type Event[T any] struct {
	Name string
}

// EventFamily is a set of events named by a prefix and a session id, one per
// session: "terminal:exit:<id>". Terminal I/O is named this way so each
// terminal subscribes only to its own stream instead of filtering a global
// broadcast. Every other session event is global and carries the id in its
// payload: its consumers outlive any one card, and a per-session name can only
// reach a subscriber that exists when it is emitted.
type EventFamily[T any] struct {
	Prefix string
}

// Of is the family's event for session id.
func (f EventFamily[T]) Of(id string) Event[T] {
	return Event[T]{Name: f.Prefix + id}
}

// ID is the session an event of the family is about, false for an event of
// another name.
func (f EventFamily[T]) ID(name string) (string, bool) {
	return strings.CutPrefix(name, f.Prefix)
}

// DeclaredEvent is one Event or EventFamily with its payload as a value, for
// code that walks the whole contract (the golden test). Name ends in "<id>"
// for a family.
type DeclaredEvent struct {
	Name    string
	Payload reflect.Type
}

var declaredEvents = map[string]DeclaredEvent{}

// Events returns every Event and EventFamily this package declares.
func Events() map[string]DeclaredEvent {
	return declaredEvents
}

func declareEvent[T any](name string) {
	if _, dup := declaredEvents[name]; dup {
		panic(fmt.Sprintf("api: event %s declared twice", name))
	}
	declaredEvents[name] = DeclaredEvent{Name: name, Payload: reflect.TypeFor[T]()}
}

func defineEvent[T any](name string) Event[T] {
	declareEvent[T](name)
	return Event[T]{Name: name}
}

func defineEventFamily[T any](prefix string) EventFamily[T] {
	declareEvent[T](prefix + "<id>")
	return EventFamily[T]{Prefix: prefix}
}

var (
	// SessionStatusEvent carries a session's processing state ({id, state, tool,
	// detail, reason} — "busy"/"done"/"waiting"/"idle"/"compacting", plus the tool
	// a pre-tool report names and what a waiting one is blocked on), reported by
	// the lich hooks running inside the PTY (see transport.hook and
	// docs/hooks/session-state.md). "interrupted" is the one value lich raises
	// itself, for a turn the user stopped at the PTY. The window keeps it in a
	// store keyed by id (statusStore, native/cmd/lich- native/status.go).
	SessionStatusEvent = defineEvent[SessionStatus]("session-status")

	// SessionUsageEvent carries a session's context-window usage ({id, percent,
	// tokens}), emitted after a turn ends. Global like the other session events.
	SessionUsageEvent = defineEvent[SessionUsage]("session-usage")

	// SessionTitleEvent carries a session's new name ({id, label}), so every card
	// takes it without a reload: the auto-applied ai-title, and a rename the store
	// has written, whoever asked (the window, `lich rename` or the MCP tool). One
	// event for both because the payload is the same pair and the window's answer
	// to both is the same: relabel that card.
	SessionTitleEvent = defineEvent[SessionTitle]("session-title")

	// SessionTouchedEvent carries the id of a session that likely changed files on
	// disk, nudging an immediate git-status refresh ahead of the steady poll.
	SessionTouchedEvent = defineEvent[SessionTouched]("session-touched")

	// SessionAgentEvent carries which provider CLI is live inside a session's PTY
	// ({id, agent}) — reported through the session-start hook by whichever
	// provider ships it, so a shell card shows that provider's mark while its CLI
	// runs in it. An empty agent clears the mark; every PTY spawn emits that clear
	// so a respawned session never wears a dead agent's icon.
	SessionAgentEvent = defineEvent[SessionAgent]("session-agent")

	// SessionConversationEvent carries the provider conversation a session's row
	// now names ({id, providerSessionId}, "" once it names none), emitted after
	// the row is written: the window offers that conversation back when the
	// session's process exits.
	SessionConversationEvent = defineEvent[SessionConversation]("session-conversation")

	// SessionSandboxEvent carries whether a session's PTY runs confined ({id,
	// confined, skippedLinks}), emitted by every spawn. The card marks a confined
	// session, and the answer is the spawn's own — it takes the provider's rung,
	// the checkout and a per-session override to reach, so the window is told
	// rather than asked to work it out again. skippedLinks is what that sandbox
	// left out of the private home for being a symlink, which is the one absence a
	// session has no other way of learning about. Both persisted with the row
	// (store.Session's Sandbox and SandboxSkippedLinks), which is what a reopened
	// window hydrates from.
	SessionSandboxEvent = defineEvent[SessionSandbox]("session-sandbox")

	// SessionMCPEvent carries the MCP servers a session's provider could reach at
	// its spawn ({id, servers}), emitted by every spawn. The card divides a tool
	// name against them — two harnesses spell an MCP tool with a single underscore
	// between the server and the tool, which nothing in the string divides
	// (docs/hooks/session-state.md). Persisted with the row too (store.Session's
	// MCPServers), which is what a reopened window hydrates from: the PTY outlives
	// the window, so there is no second spawn to report it again.
	SessionMCPEvent = defineEvent[SessionMCP]("session-mcp")

	// SessionDuplicateEvent carries a session that just reported a conversation
	// another open card already runs ({id, heldBy}): a resume typed in the
	// provider's own TUI, which lich cannot stop, only point out. Emitted once the
	// report is stored, so the window can offer to switch to heldBy.
	SessionDuplicateEvent = defineEvent[SessionDuplicate]("session-duplicate")

	// SessionCwdEvent carries a session's live working directory ({id, cwd,
	// host}), emitted once with the directory the PTY starts in and again whenever
	// the child process moves (the user runs `cd`). A non-empty host names a
	// foreground process the directory cannot be read through (see shellHosts) and
	// comes with an empty cwd: the session is somewhere, but not somewhere this
	// machine can name. Global like the other session events.
	SessionCwdEvent = defineEvent[SessionCwd]("session-cwd")

	// SessionLimitEvent carries a turn a plan's usage limit ended ({id, window,
	// resetsAt}). Nothing clears it; the window reads plan quota again when it
	// arrives (planQuota, native/cmd/lich-native/footer_quota.go).
	SessionLimitEvent = defineEvent[SessionLimit]("session-limit")

	// SessionProfileEvent carries the profile a session moved to: Continue in, or
	// a resume that found its conversation in another profile.
	SessionProfileEvent = defineEvent[SessionProfile]("session-profile")

	// SessionTodoEvent carries how far a session got through the task list its
	// agent wrote for itself ({id, done, total}), emitted whenever that pair
	// changes. Global like the other session events.
	//
	// The pair is what the agent last wrote, not what is true: an agent that
	// abandons a list without closing its items leaves the card reporting the
	// count it was abandoned at, until it writes another one.
	SessionTodoEvent = defineEvent[SessionTodo]("session-todo")

	// PluginIncompatibleEvent carries a session whose hooks report from a plugin
	// release outside the range this lich speaks ({id, version}). The window
	// answers it by asking for the plugin status again, which is where the fix is
	// offered: an install moved by the harness's own marketplace shows up here
	// first.
	PluginIncompatibleEvent = defineEvent[PluginIncompatible]("plugin-incompatible")

	// TerminalDataEvents carries base64-encoded PTY output. Output is
	// base64-encoded because raw PTY bytes may split a multi-byte UTF-8 sequence
	// mid-read, which the JSON event bridge would otherwise corrupt.
	TerminalDataEvents = defineEventFamily[string]("terminal:data:")

	// TerminalExitEvents is emitted once when a session's shell process exits,
	// carrying the status it exited with (see SessionExit).
	TerminalExitEvents = defineEventFamily[SessionExit]("terminal:exit:")

	// SessionFocusEvent asks the window to open a session's card, the way a click
	// on it does. Global like the other session events: the card it names may live
	// in a project whose tab is not the one on screen.
	SessionFocusEvent = defineEvent[SessionFocus]("session-focus")

	// SessionInboxEvent carries how many uncollected results a session has
	// waiting, so its card can say so — the nudge tells the agent, and this tells
	// the person. Emitted whenever a sender's inbox changes size; zero clears the
	// mark. Global for the same reason the relay event is.
	SessionInboxEvent = defineEvent[SessionInbox]("session-inbox")

	// SessionScheduleEvent is emitted when a session's scheduled prompt changes
	// without the window having written it: delivered, or parked and dropped by
	// lich itself around a usage limit (resume.go). Payload: SessionSchedule.
	SessionScheduleEvent = defineEvent[SessionSchedule]("session-schedule")

	// SessionRelayEvent carries which session is waiting on which, so the sidebar
	// can say it. Global rather than per-session because its consumer — one store
	// keyed by id — outlives any one card, exactly as the status event's does.
	SessionRelayEvent = defineEvent[SessionRelay]("session-relay")

	// RelayStalledEvent carries a request whose target ended its turn without
	// answering through lich. The window turns it into a toast that opens the
	// target's card, because the answer — if there is one — is on that screen and
	// the person who asked has no other way to know where it went.
	RelayStalledEvent = defineEvent[RelayStalled]("session-relay-stalled")

	// SessionOpenedEvent carries a session opened outside the window, so the card
	// appears without a reload. Global, like every other session event: its
	// consumer is the workspace state, which outlives any one card.
	SessionOpenedEvent = defineEvent[OpenedSession]("session-opened")

	// SessionClosedEvent carries a session closed outside the window, so the card
	// goes away without a reload. Its payload is SessionClosed.
	SessionClosedEvent = defineEvent[SessionClosed]("session-closed")

	// SessionsFiledEvent carries sessions whose folder changed outside the window,
	// so their cards move into or out of the folder's block without a reload. One
	// event for a card filed and for a folder renamed, because the window's answer
	// to both is the same: set these cards' folder.
	SessionsFiledEvent = defineEvent[SessionsFiled]("sessions-filed")

	// SessionsColoredEvent carries sessions painted outside the window, so their
	// cards take the colour without a reload.
	SessionsColoredEvent = defineEvent[SessionsColored]("sessions-colored")

	// ProjectOpenedEvent carries a project opened outside the window — an agent
	// naming a directory rather than a project already on screen — so its tab
	// appears without a reload. Its payload is the whole Project, sessions
	// included: a reopened project brings back the cards it was closed with, and
	// the session this open is about to announce lands in a tab the window has to
	// be holding already.
	ProjectOpenedEvent = defineEvent[Project]("project-opened")

	// ProjectAddedEvent is emitted once AddProject has committed: a project opened
	// new, reopened from the history, or reopened at the directory it moved to —
	// the same id under another path is how a relocate reaches the store. Payload:
	// ProjectAdded. It is not project-opened, which an agent's open announces with
	// the whole project and which the web workspace adds a tab from; this one only
	// says the row changed, and the window reads it back.
	ProjectAddedEvent = defineEvent[ProjectAdded]("project-added")

	// ProjectClosedEvent is emitted once CloseProject has marked a project closed,
	// whoever asked: the window's own tab, an agent, the CLI. Payload:
	// ProjectClosed.
	ProjectClosedEvent = defineEvent[ProjectClosed]("project-closed")

	// ProjectsReorderedEvent is emitted once ReorderProjects has written the tab
	// order. Payload: ProjectsReordered.
	ProjectsReorderedEvent = defineEvent[ProjectsReordered]("projects-reordered")

	// ScheduleForfeitedEvent is emitted when a scheduled prompt was lost with the
	// session it was parked on, so the person who parked it is told rather than
	// left to find out at a time that never comes. Payload: ScheduleForfeited.
	ScheduleForfeitedEvent = defineEvent[ScheduleForfeited]("session-schedule-forfeited")

	// ClaudeProfileLoginEvent is the event a login reports its progress on.
	ClaudeProfileLoginEvent = defineEvent[ProfileLogin]("claude-profile-login")

	// CodexProfileLoginEvent is the event a login reports its progress on.
	CodexProfileLoginEvent = defineEvent[ProfileLogin]("codex-profile-login")

	// ProfileChangedEvent is the event a profile's update emits once it is
	// written: its name, color or image changed.
	ProfileChangedEvent = defineEvent[ProfileChanged]("profile-changed")

	// PluginProfilesEvent is the event a pass over a provider's profiles reports
	// on, once it has installed into at least one.
	PluginProfilesEvent = defineEvent[PluginProfiles]("lich-plugin-profiles")

	// AppUpdateProgressEvent is the hub event Apply's phases ride on, for the
	// toast.
	AppUpdateProgressEvent = defineEvent[AppUpdateProgress]("appupdate-progress")

	// ThemeSelectionEvent is pushed on /events with a ThemeSelection once a change
	// to it is stored.
	ThemeSelectionEvent = defineEvent[ThemeSelection]("theme-selection")

	// PullRequestChangedEvent is what a pull request write through lich reports
	// once GitHub took it, so every window's pull request store reads it again
	// instead of the screen that made the change telling the others.
	PullRequestChangedEvent = defineEvent[PRChanged]("pull-request-changed")

	// BranchPullRequestEvent is every read of a checkout's branch pull request,
	// changed or not: the backend is the one reader of it, and a window that
	// asked for a recheck hears its answer here.
	BranchPullRequestEvent = defineEvent[BranchPullRequest]("branch-pull-request")

	// ServicesEvent carries every project service and its state, emitted once a
	// change to either is true: a save, a delete, a start, an exit.
	ServicesEvent = defineEvent[[]Service]("services")

	// ServiceRevealEvent asks the window to open a service's console, which the
	// tray's Show output does from outside it.
	ServiceRevealEvent = defineEvent[ServiceIDOptions]("service-reveal")

	// WindowCloseEvent asks the native window to close, lich quitting.
	WindowCloseEvent = defineEvent[None]("window-close")

	// WindowRaiseEvent asks the native window to come forward.
	WindowRaiseEvent = defineEvent[None]("window-raise")
)
