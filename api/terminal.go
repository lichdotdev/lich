package api

var (
	TerminalStart             = define[StartOptions, None]("terminal.Start")
	TerminalClose             = define[SessionIDOptions, None]("terminal.Close")
	TerminalWrite             = define[WriteOptions, None]("terminal.Write")
	TerminalResize            = define[ResizeOptions, None]("terminal.Resize")
	TerminalSetVisible        = define[SetVisibleOptions, None]("terminal.SetVisible")
	TerminalReplay            = define[SessionIDOptions, bool]("terminal.Replay")
	TerminalLive              = define[SessionIDOptions, bool]("terminal.Live")
	TerminalLiveCount         = define[None, int]("terminal.LiveCount")
	TerminalReady             = define[SessionIDOptions, bool]("terminal.Ready")
	TerminalHandsOn           = define[SessionIDOptions, int64]("terminal.HandsOn")
	TerminalLastTurnSaid      = define[SessionIDOptions, LastSaid]("terminal.LastTurnSaid")
	TerminalTurnStates        = define[None, []TurnState]("terminal.TurnStates")
	TerminalContinueIn        = define[ContinueInOptions, None]("terminal.ContinueIn")
	TerminalSearchTranscripts = define[SearchTranscriptsOptions, []TranscriptMatch]("terminal.SearchTranscripts")
	TerminalWorkdirMissing    = define[WorkdirOptions, bool]("terminal.WorkdirMissing")
	TerminalResumeAvailable   = define[ResumeAvailableOptions, bool]("terminal.ResumeAvailable")
)

// SessionIDOptions names a session by its id.
type SessionIDOptions struct {
	ID string `json:"id"`
}

// StartOptions spawns session ID's process at Cols x Rows. Resume is a
// provider conversation to reopen, "" to start fresh; Fork branches it into a
// conversation of its own instead.
type StartOptions struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Cwd       string `json:"cwd"`
	Kind      string `json:"kind"`
	Resume    string `json:"resume"`
	Fork      bool   `json:"fork"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// WriteOptions types Data into session ID's PTY.
type WriteOptions struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

// ResizeOptions sets session ID's PTY size.
type ResizeOptions struct {
	ID   string `json:"id"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// SetVisibleOptions says whether session ID is on screen.
type SetVisibleOptions struct {
	ID      string `json:"id"`
	Visible bool   `json:"visible"`
}

// ContinueInOptions moves session ID's conversation to Profile, a slug of
// its provider.
type ContinueInOptions struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
}

// SearchTranscriptsOptions searches the transcripts of sessions IDs for Query.
type SearchTranscriptsOptions struct {
	IDs   []string `json:"ids"`
	Query string   `json:"query"`
}

// WorkdirOptions names a session's working directory.
type WorkdirOptions struct {
	Cwd string `json:"cwd"`
}

// ResumeAvailableOptions asks whether conversation ProviderSessionID of
// provider Kind can be resumed from Cwd.
type ResumeAvailableOptions struct {
	Kind              string `json:"kind"`
	ProviderSessionID string `json:"providerSessionId"`
	Cwd               string `json:"cwd"`
}

// SessionUsage is the payload of the session-usage event. Percent is the share of the
// context window the turn left occupied (0–100); Tokens is the provider's active
// context count behind it, Window the model's effective context window, Model
// the model id, and Effort the reasoning effort level ("" when the line records
// none).
//
// All five are zero for a provider that records what a turn spent but not the
// window it spent it against — oh-my-pi, opencode and Crush, the cost-only rung
// of docs/ceilings.md (each in docs/providers/). A zero Window is the window's signal to draw the cost
// alone rather than a ring around a window nobody reported.
//
// CostUSD is what the session has cost so far, and is omitted — not zeroed —
// whenever there is no number worth showing: the setting is off, or a model in
// the transcript has no known price. The readout is for API billing, so on a
// subscription the field never appears at all and the footer shows nothing.
//
// CostMiss says why, and only when the answer is "lich cannot price this
// conversation". It stays empty for the readout being
// off, which is not a failure but the design above, and for a miss the next turn
// heals. Without it a session that cannot be priced and a session that is not
// being priced look identical, and an empty footer reads as zero spend.
type SessionUsage struct {
	ID       string   `json:"id"`
	Percent  int      `json:"percent"`
	Tokens   int      `json:"tokens"`
	Window   int      `json:"window"`
	Model    string   `json:"model"`
	Effort   string   `json:"effort"`
	CostUSD  *float64 `json:"costUsd,omitempty"`
	CostMiss string   `json:"costMiss,omitempty"`
}

// TurnState is what the window missed about one live session while it was not
// attached to /events: the last session-status report (State is empty for a
// session that never reported) and the readout the last session-usage event
// carried (nil when there is none to read). Since is when the session entered
// State, in Unix milliseconds. Cwd and Host are its last session-cwd report.
type TurnState struct {
	ID     string        `json:"id"`
	State  string        `json:"state,omitempty"`
	Tool   string        `json:"tool,omitempty"`
	Detail string        `json:"detail,omitempty"`
	Reason string        `json:"reason,omitempty"`
	Since  int64         `json:"since,omitempty"`
	Usage  *SessionUsage `json:"usage,omitempty"`
	Cwd    string        `json:"cwd,omitempty"`
	Host   string        `json:"host,omitempty"`
}

// LastSaid is the prose the agent ended its last turn with, the body of the
// window's finished-turn notification. Text is empty for every absence there is
// — a provider whose conversation lich cannot read, a turn that ended in a tool
// call, a transcript still being written — because the notification goes out
// without a body either way and a reason it cannot act on is a reason not worth
// wiring.
type LastSaid struct {
	Text string `json:"text,omitempty"`
}

// TranscriptMatch is one session whose conversation mentions the query: the
// session id to jump to, the newest matching message trimmed to a line, and how
// many of its messages matched.
type TranscriptMatch struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
	Count   int    `json:"count"`
}
