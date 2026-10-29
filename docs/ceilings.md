# Known Ceilings

Deliberate limits and shortcuts, and the traps they set. A bullet earns its place by naming something that breaks
work when nobody knows it and that the call site never shows. The mechanism and the history stay in the code and
`CHANGELOG.md`; this file is the trap alone. Provider-specific behaviour is reference, not a trap, and lives in
[`providers/`](providers/).

## Spawn, start and resume

- **`LICH_WORKTREE_PORT` is reserved, never held** (`internal/terminal/worktreeport.go`): the number is a name the
  checkout owns, nothing binds it, and anything on the machine can take the port before the dev server starts.
- **A session is ready for work only once its agent reports from its own prompt, where it can**
  (`internal/terminal/startgate.go`): in a fresh worktree several providers ask a question first (trust the
  folder, initialize the project), and anything typed then (a relayed task, `lich open --prompt`, a scheduled
  prompt, an issue or pull request handed over) goes into the question, with the Enter behind it answering it
  (what each provider then does: `docs/providers/`, Spawn). Claude Code, Cursor CLI, oh-my-pi and Kiro CLI stay
  unready until their session-start report, which their hooks send only from the agent's prompt. That hold needs
  lich-plugin's hooks in the provider: without them nothing would ever release it, so nothing is held and the
  typed text lands in the question. Codex, Antigravity, Crush and opencode report nothing before their first
  turn, so a quiet screen is still all lich has for them.
- **A reasoning effort is refused where the TUI would drop it, and never validated**
  (`internal/terminal/command.go`, `effortFlags`): `lich open --effort` and `open_session`'s `effort` are refused
  for opencode, Crush and Cursor CLI, whose interactive CLI takes no level, rather than handing back a session on
  its default (why each: `docs/providers/`, Spawn). The level itself is never validated,
  so a misspelled one is each provider's to handle, and most start without a word.
- **An ultracode turned off inside a Claude Code session comes back on its next restart**
  (`internal/terminal/command.go`, `ultracodeArgs`): Claude Code drops ultracode on `--resume`, so lich passes it
  on every spawn (`docs/providers/claude.md`, Spawn), and nothing turns it off for one session while the
  provider-wide switch is on.
- **A model or effort is a birth value, and some providers forget it on resume** (`internal/terminal/command.go`,
  `modelArgs`, `effortArgs`): lich passes neither when it resumes or forks a conversation, because a `/model` or
  `/effort` typed inside the session is the user's decision and re-sending the birth value would undo it. Claude
  Code's effort and all of Antigravity resume on the saved default rather than on the conversation's (each
  measurement in `docs/providers/`, Spawn), so a change made inside the session survives while a level or model
  lich set at birth falls back to that default. Cursor CLI also writes whatever model a spawn or resume runs on into
  `cli-config.json` as the default for every later `cursor-agent`, so `lich open --kind cursor --model` changes
  the user's Cursor default too.
- **Start new leaves the old conversation reachable only through History search**
  (`native/cmd/lich-native/exit_toast.go`): once the new conversation's session-start report arrives, the backend
  writes its id over the card's `provider_session_id`, and nothing on the card points back at the old one. Start
  new asks no confirmation for that reason.
- **A provider's "trust this folder?" answer is carried only where lich can find the person's own answer,
  and a confined session is told to skip the question** (`internal/claudeprofile/trust.go`,
  `internal/terminal/cursor_trust.go`, `antigravity_trust.go`, `crush_init.go`, `confinedEnv` and
  `confinedTrustArgs` in `internal/terminal/sandbox.go`; each provider in `docs/providers/`, Spawn and Sandbox):
  before a spawn into a worktree or a profile lich writes a provider's trust for the folder only when the person
  already trusted it in Personal or trusted its main checkout, and it never trusts a folder nobody trusted. A
  confined session skips the question, because what it guards against, a folder's own settings running on the
  machine, is what the sandbox confines; on Cursor CLI and Antigravity skipping it writes the trust down, so a
  folder once opened confined stays trusted outside the sandbox too. Whether Codex's trust and Cursor CLI's
  `--trust` loosen anything beyond the question was not measured.

## Session state and hooks

- **The sidebar's state chips read the ring, nothing more** (`native/cmd/lich-native/sidebar_filter.go`):
  Waiting, Running and Unread are the card's amber, spinning and unread green ring, and Idle is everything
  else, a read turn included. A provider that cannot raise a state cannot be filtered into it: Crush
  reports no turn at all and always sits under Idle; Antigravity, oh-my-pi, Kiro CLI and Cursor CLI raise no
  permission event, so they never reach Waiting (a Kiro or Cursor session blocked on a confirmation counts as
  Running).
  There is no error chip: no provider reports one, and a process that exited is known only to its terminal,
  not to a store the sidebar reads. The chips are not persisted, like the query.
- **Next waiting session reads the same ring** (`waitingTarget`, `native/cmd/lich-native/hotkeys_dispatch.go`):
  it jumps only to a card whose last report is Waiting, so it never lands on the five providers the chips above
  never put under Waiting. It walks the order next session walks,
  the cards the sidebar shows (`hotkeyCards`): a card in a folded block or left out by the search and state
  filter is skipped rather than its block unfolded, so a waiting session folded away is reached only through
  its block's mark. The active card never counts: when it is the only one waiting, the chord does nothing and
  goes on to the terminal.
- **An interrupted turn is read off the keystrokes, not from the provider** (`internal/terminal/draft.go`,
  `hookstate.go`, `Service.noteInterrupt`): Claude Code, Codex and oh-my-pi all skip the hook that ends a turn when
  the user stops one, so lich publishes `interrupted` itself when a lone Ctrl+C or Escape reaches a session it knows
  is mid-turn. It is also raised with no keystroke, when a Claude Code mod acks an abort sent from lich and when a
  usage limit ends a Codex turn (`internal/terminal/limit.go`). It is a guess made from bytes, and it has three
  edges. A provider session running a tool that owns the terminal — an editor opened through a shell command — takes
  Escape as the interrupt and clears the ring while the turn is still running; the next report from the provider
  puts it back. A provider that reports its own abort as the turn finishing wears the solid ring a completed turn
  does, because the provider's own word outranks the keystroke (opencode, `docs/providers/opencode.md`), and one
  that reports no state at all has no turn to end (Crush). And an errand the relay delivered survives an interrupt
  on purpose: stopping a turn is not answering the request, so the sender keeps waiting for the target's next turn
  rather than being told the work is over.
- **A finished turn is unread until its own card is watched** (`native/cmd/lich-native/cardstate.go`,
  `setFocused`): the solid emerald ring means "back from the agent, not read yet", and it
  fades only for the session whose terminal is on screen **while the window has focus**. A card left focused in a
  background window keeps its ring solid until the window is touched again, which is the point, but it also means
  a desktop that reports focus oddly never fades one.
- **Crush raises no hook at a turn's end, so nothing that waits for `done` fires there** (its only event is
  `PreToolUse`, 0.98.1; `docs/hooks/session-state.md`). A task handed to a Crush session that ends its turn
  without answering waits out its ticket (`internal/relay/observe.go`, `turnCandidates`): its database does mark
  the end (`finish.reason` other than `tool_use` in `.crush/crush.db`), but the conversation id reaches lich only
  on the first tool call, so a turn that calls no tool has nothing to read, and the asker hears nothing until the
  ticket's deadline. A pull request a Crush session opens reaches its card only when the window regains focus
  (`native/cmd/lich-native/gitstate.go` `recheckMissingPR`, `cardstate.go` `setFocused`): `gh pr create` moves
  neither branch nor HEAD, so a card showing no pull request asks gh again when its session's turn ends. A Crush
  card is never named after its conversation either: Crush files a title in that same database, and nothing tells
  lich when to read it (`docs/hooks/session-title.md`). The day Crush ships `Stop`, the existing
  `report-state.sh done` closes the first two, and a read like Kiro's and Cursor's (`internal/terminal`,
  `title_filed.go`) the third.
- **A CLI run as a tool inside a session is told apart by conversation id, and Kiro has none to tell by**
  (`internal/terminal/nested.go`, docs/hooks/session-state.md "Nested agent CLIs"): a `claude -p` the agent runs
  inherits the card's `LICH_*` env and reports as it. A state report naming another conversation is dropped, and a
  session-start naming one is dropped while the card's turn is open. A scripted Kiro CLI run (`--no-interactive`, how
  an agent runs one) carries no id in any hook payload (measured on 2.21.0), so a nested `kiro-cli` still reports
  as its host. The
  start guard reads lich's own turn, so a host lich never sees open cannot use it: a Crush session reports no turn
  at all, so a nested CLI's start can still move what its card resumes. A plugin older than 0.20 sends no id on state reports, which are then taken as
  before.
- **Keep-awake follows the session-state report, and only that** (`internal/awake`, `turnLog.onOpen`): the
  machine is held out of idle sleep while a hook says a turn is open, so what a provider reports decides whether
  the machine stays up at all (`docs/providers/`). Linux holds it through `systemd-inhibit --what=idle`: a distro
  without systemd logs one warning per burst of work and sleeps, and a desktop that ignores logind idle
  inhibitors sleeps silently. The hold was measured on Linux only; on Windows and macOS CI proves the request
  is registered (`powercfg /requests`, `pmset -g assertions`), not that the machine stays up.
- **The two-cards-one-conversation warning reaches only the providers that report an in-app switch**
  (`internal/terminal/terminal.go` `reportDuplicate`, `native/cmd/lich-native/duplicate.go`): lich learns
  which conversation a card runs from its session-start report, so a resume typed inside the agent's own app
  is warned about only once that report names the new id. Some report it only with their next prompt or tool
  call, so a turn may already have run in both cards, and Cursor CLI and opencode report nothing at all: their
  row keeps the old id and a restart resumes the conversation the card opened with (each provider's version:
  `docs/providers/`, Hooks). Reopening from lich itself is not this gap: every door through `store.reopen`
  focuses the card that holds the conversation.
- **Only a Claude Code session whose mod polls can be driven, asked, or report for itself** (`spawn.Control`,
  `spawn.Ask`, `internal/terminal/modcontrol.go`, `modusage.go`, `modstatus.go`; each contract in `docs/hooks/`,
  mod-control, mod-usage, mod-status and session-state, has its own Known ceilings): mods are a Claude Code
  feature, from 2.1.280 and only with the lich-plugin mod loaded, 0.15.0 or newer. The other seven providers, and
  a Claude Code session without the mod, have nothing that runs inside them and polls. So `lich control` and
  `lich ask` refuse them by provider before anything is queued (no other CLI exposes a fork of its own
  conversation to anything running inside it), their readout stays the one lich derives, their errands show only
  in the card's tooltip and inbox mark, and a compaction keeps the finished turn's check or is a spinner with no
  tool line, which reads as a stuck turn. The traps a caller walks into:
  - `/model` and `/effort` are refused as slash commands: run through a mod they write `~/.claude/settings.json`,
    the default for every new session (2.1.288). The `model` and `effort` actions are the per-session route.
  - A slash command that opens a dialog (`/cost`) holds that session's later slash commands until someone
    presses Esc in its terminal; lich cannot tell such a command apart before running it.
  - A session cannot ask itself, nor control itself past `model` and `effort`: an abort would end the asking turn,
    a prompt or a slash command would only run after it, and a question would ride the request it is waiting on.
  - An ask knows the conversation only as of the last finished model response, cannot be cancelled (past lich's
    90 seconds the fork runs on, billed, and its answer is dropped), never reaches the session's cost, and is cut
    at 16,000 characters (2.1.289).
  - A reported readout wins over the derived one, so two sessions on the same model can disagree by the requests
    no transcript records (2.7% and 3.1% of a session's cost on 2.1.289). Its cost replaces the conversation's
    ledger row and drops its sub-agent rows, so a lich restarted before the session's first report re-scans that
    conversation from the transcript until the report lands again. Only a long-lived token login's plan gauge
    uses the reported rate limits: a credentials login keeps its usage route, which names the plan, the account
    and the model-scoped caps a report does not carry.
  - An errand sent privately (a subagent or workflow step inside the session) never shows on the status line,
    since the session's own collect never reaches it.

## Transcripts, cost and plan usage

- **The cost readout bills per `(session, transcript)`** (`internal/pricing`, `internal/terminal/usage_cost.go`): a
  conversation forked inside the PTY bills its copied history twice — lich's own resume continues the same
  transcript and is unaffected — and each sub-agent's own transcript is counted in, so one unreadable or
  unpriceable sub-agent withholds the whole session's number. A Claude Code session whose mod reports a cost
  is not scanned at all: the report replaces the transcript, sub-agents included. A withheld number is marked `$—` on the footer
  only when the reason is standing (`costMiss.spoken` in `internal/terminal/usage_cost.go`): a transcript that
  merely could not be read this turn says nothing and keeps the last figure, so a reader cannot tell that
  absence from a session still on its first turn.
- **The session readout is each provider's own record, and only Claude Code and Codex fill it**
  (`internal/terminal/usage.go`, `usageSourceFor`; who shows what is README's provider table, and what each records
  is `docs/providers/`, Transcript & cost): which reading a provider gets is decided by what it writes down, not by
  what lich chose to read. The cost-only figures (oh-my-pi, opencode, Crush) are the providers' own arithmetic,
  never re-priced here, so each inherits what its own accounting leaves out, and how a sub-agent's spend reaches its
  parent's figure is measured behaviour of another tool's schema, not a promise it made: the day one changes its
  mind, the footer is silently wrong in one direction or the other, with nothing on screen saying so. Their usage
  event carries a zero window, which is what tells the footer to drop the ring (`contextReadout`,
  `native/cmd/lich-native/footer_readout.go`). A zero from any of the three is an answer and not a gap: it is what
  they record for a free model and for a subscription. The last turn's cache line (`api.SessionCache`) sits in the context
  tooltip on Claude Code and Codex, and in the cost tooltip on oh-my-pi and opencode, which record the split but
  no window. Crush folds its cache counts into the prompt and completion totals, Kiro files every token count as
  zero, and Cursor and Antigravity have no usage reader, so the four show none. Codex is the one provider lich prices itself, from a slice of
  rates baked into the release (`internal/pricing/prices.json`), so an offline Codex session shows a cost at the
  rate that shipped — **stale by however long it has been since the release**, until the refresh
  (`internal/pricing/pricing.go`) lands and overrides it. What is left with no price at all is a model newer than
  the build, or one LiteLLM never priced — and with no network the refresh that would settle it never runs — and a
  Codex conversation that ran `/model` has no cost from that turn on. **Those two absences are spoken; the staleness
  is not.** A session lich cannot price draws `$—` with a tooltip naming which of the two it is
  (`usageEvent.CostMiss`, `costMissReasonKeys` in `native/cmd/lich-native/footer_cost.go`), while a cost billed at a
  rate the release froze reads as an ordinary figure with nothing beside it. The marker also names the reason and
  never the model or the turn, so a session whose sub-agent alone is unpriced looks like one whose every turn is.
- **Footer choices cannot add readings a provider never reports** (`native/cmd/lich-native/footer_layout.go`):
  selecting a reading the session's provider does not record renders nothing, never a zero, and hiding a reading
  hides its warning too. Cost still uses the backend setting because it controls whether
  transcripts are priced: when another profile disables it, an old layout cannot enable pricing just by
  moving an unrelated item. The editor takes no change until that setting has loaded.
  An item without data can occupy an editor slot while drawing
  nothing in the live footer — a PR on a branch without one, or an unsupported provider reading.
- **The card's tooltip names the running model only where the readout above has one**
  (`modelRuns`, `native/cmd/lich-native/sessiontooltip.go`): the line comes from the model the session-usage
  event carries, which only Claude Code, Codex and Kiro CLI record (`docs/providers/`, Transcript & cost), so
  oh-my-pi, opencode, Crush, Antigravity and Cursor CLI get no line at all, never the model the session was born
  on standing in for one nobody read. When that model differs from the birth model (`model` column,
  `SetSessionModel`) the line reads "Opened on …, running …". The comparison is a heuristic, not a lookup:
  equal once both are shortened (`formatModel`), or the birth model is the family alias of the running id
  (`opus` against `claude-opus-5-5`, a `[1m]` suffix dropped). Any other alias the CLI accepts (`default`,
  `opusplan`) reads as a change, and so does a `/model` the user typed, which is intended.
- **Hands-on time on a Crush card misses every turn that calls no tool**
  (`internal/terminal/handson.go`, `noteOutput`): the figure beside the cost counts the gap between consecutive
  signs of life in a session — any hook report naming it, a keystroke at its PTY, or its own output while a turn
  is open — and drops any gap longer than `handsOnIdleGap`. The output signal is gated on the provider having
  reported `busy`, because a `tail -f`, a dev server or a TUI repainting would otherwise bill hours nobody
  worked. Crush opens no turn (`docs/hooks/session-state.md`) and is counted only through the reports its tool
  calls fire (`docs/providers/crush.md`), so a long answer written straight out, with no keystroke and no report
  between the prompt and the reply, is worth only the gap the prompt itself closed. The trap is reading the
  number as exactly comparable across cards — a toolless turn is time on a Claude Code card and nothing on a
  Crush one. The tooltip under the figure says which the session listened to, a turn or a tool call
  (`handsOnDetail`, `native/cmd/lich-native/footer_handson.go`), and not how much that cost the figure. A plain
  shell session is keystrokes-only by design and not a gap: there is no agent in it whose work could be missed.
- **The plan gauge answers to undocumented routes, and three providers have none** (`internal/quota`; each route
  is in `docs/providers/`, Account): every route is what a provider's own CLI or dashboard polls, and none is
  published API. oh-my-pi, Crush and Kiro CLI report no plan lich can read, and opencode reports one only for an
  OpenCode Go subscription. A field renamed upstream drops the window it fed rather than
  raising anything, so a new kind of limit is invisible instead of wrong. lich reads those logins and never
  writes them: it does not refresh a token, so an expired one reads as signed out until the provider's own CLI
  rotates it. A reading is cached for five minutes because the endpoints rate-limit hard; the number on screen is
  up to that old, and nothing on it says so.
- **A turn a usage limit stopped is picked back up on two providers only** (`internal/terminal/limit.go`,
  `internal/relay/resume.go`): Claude Code and Codex are the two that write the limit down with its reset. On
  Antigravity, Crush, Cursor CLI and Kiro CLI nothing marks a limit at all, so the card keeps spinning until the
  next prompt; opencode and oh-my-pi retry by themselves and lich adds nothing on top. Claude Code's end arrives
  only as `StopFailure`, which an older lich-plugin does not report. Codex raises no hook for it, so an open
  Codex turn is read off its rollout every 30 seconds, and the limit is *inferred*: a turn completing with
  nothing said right after a window shows 100% spent (measured on `codex exec` 0.144.5, never on the TUI); a
  change to that shape stops the read without a word. The continuation is a scheduled prompt told apart from a
  person's by its exact text, so someone parking those same words has them dropped on the session's next turn.
  Nothing is parked for a reset already past or more than 24 hours out (Claude Code's own auto-continue gives up
  at the same horizon), nor over a prompt already parked there. Any turn starting before it is due drops it,
  which is also what keeps it from landing behind Claude Code's own `autoContinueAtUsageLimit`, a setting the
  server enables per account, which is why the prompt is due two minutes after the reset rather than at it. The
  window draws neither the limit nor the parked prompt on the card.
- **The pace marker is borrowed calibration, and it is silent far more often than it is wrong**
  (`internal/quota/pace.go`): the two numbers that decide when a weekly window is marked as spending ahead —
  fifteen percentage points past the elapsed share, and no marking at all in the first twenty-four hours after
  a reset — are the claude-swap project's measurements (`src/claude_swap/pace.py`, its issue #125), not lich's
  own. Nothing here has been tuned against a lich user's accounts. The consequence to know before debugging a
  marker that "never appears": a window ninety percent spent on the first day of its cycle is deliberately
  unmarked, because just after a reset the elapsed share is near zero and almost any use at all would read as
  far ahead. Only the weekly window is paced, so the five-hour one and Codex's monthly free-tier window carry
  no marker however they are spent. The verdict is derived from the window's own reset time, which is the next
  reset and never the start — the start is that time rolled back whole windows — so a provider that stops
  reporting a reset time silently drops the marker rather than the gauge.
- **Measuring a token-only Claude Code login costs a request against the very plan it measures**
  (`internal/quota/claude.go`): the usage route refuses a `claude setup-token` token, so the gauge reads that
  account the way Claude Code reads it for itself, with one message (`docs/providers/claude.md`, Account).
  Reading the gauge therefore spends quota (negligibly) and appears in the account's own usage, once per cache
  window. A session on that login whose mod reports (docs/hooks/mod-usage.md) stands in for the request until one
  of its windows resets, so the request is only spent for an account no such session has spoken for since.

## Relay, errands and subagents

- **A new Claude Code session's roster name is its own, and lich does not know it** (`internal/terminal`,
  `nameArgs`; `internal/relay`, `RosterNameOf`): lich spawns it without `--name`, because a name passed there is
  the user's to Claude Code and a session holding one never writes the `ai-title` its card is titled from
  (measured on 2.1.306). It names itself after its directory and a random suffix, which keeps the roster
  unambiguous, but writes that name only into `~/.claude/sessions/<pid>.json`, keyed by a pid a confined
  session's namespace hides. So `list_sessions`, `lich sessions` and `lich whoami` show no second name for it,
  and a name an agent read off Claude Code's own `/list-agents` is not one lich resolves: that agent reaches the
  session through its own messaging or by the card's label. A `/rename` or a fork puts a name on record, and from
  then on lich answers to it; a fork pays for it with its `ai-title`.
- **A prompt in use is recognised from the bytes going in, never from the line itself**
  (`internal/terminal/draft.go`; typed deliveries only, a Claude Code session with its mod is handed the message, see below): a relayed message pastes at the prompt and sends an Enter behind it, so lich
  holds the delivery back while the user has unsent input there. What it counts is printable input since the last
  Enter, escape sequences skipped — it cannot see the line, so an edit that leaves it empty by another route
  (Ctrl+W, a click into the middle of it) reads as a draft that is still there, and a delivery waits out
  `draftIdle` for nothing. The stale-draft release is what keeps that a delay instead of a wedged relay. The hold
  is not drawn on the card: a delivery held there answers to its sender, through the ticket. What stays open is the provider that takes keystrokes through anything other than this PTY, which is
  invisible to every part of this.
- **`lich insert` pastes and leaves it, so what the person sees is the provider's own paste handling**
  (`internal/relay/insert.go`, `internal/terminal` `AtPrompt`): it types a bracketed paste at the PTY of all eight
  providers alike, a Claude Code session with the mod included, and presses nothing. The same gaps as a typed
  delivery apply and are not closed here. On Windows (ConPTY) the paste markers are dropped, so a multi-line
  selection reaches the TUI as typed characters and each provider's burst heuristic decides whether a newline
  inside it submits the line — measured on none of them, since there is no Windows hardware here. A provider that
  collapses a long paste (Claude Code's `[Pasted text #1 +40 lines]`) shows the placeholder, not the text, and the
  person cannot edit what is inside it. And a prompt lich cannot see — a trust dialog, a question of the provider's
  own — takes the text as an answer: `AtPrompt` only knows the agent has gone quiet, which `waiting` (reported by
  some providers) is what refuses; the rest are on the same footing as `send`. It ignores the user's unsent draft
  on purpose, unlike `send`, because it adds to the line instead of sending it; the draft heuristic's blind spots
  (above) do not apply to it for that reason.
- **`lich focus` raises the window the way a second launch does, with the same gaps**
  (`internal/relay/focus.go`, `SetRaiseWindow`): the card is opened by an event to the window, and the window
  is brought to the front, or opened when none is, best-effort as for a second launch. It reaches only live sessions other than the caller's own, as every
  command resolving a target does.
- **An interrupted wait lets go only when the caller says so, and oh-my-pi never says so**
  (`internal/cli/mcp.go`, `runMCP`; `internal/relay/inbox.go`, `collect`): a wait whose caller is gone must not
  take a result, because nothing reads it and the nudge was skipped for its sake. `lich wait` killed with Ctrl-C
  closes its connection and the backend sees the request's context end. An MCP tool call interrupted with Esc
  does not close anything: the `lich mcp` process lives on, so the backend only lets go when the client sends
  `notifications/cancelled`. The MCP clients of Claude Code, Codex, Antigravity, opencode, Crush, Cursor CLI and
  Kiro CLI carry that message (read in their shipped binaries, not measured Esc by Esc); oh-my-pi's does not, so
  an interrupted `wait_for_answer` or `send_to_session` there keeps holding the line until its own timeout, at most
  90 seconds (`mcpMaxWait`), and a result landing in that window is taken by a wait nobody reads. And even where
  the cancellation arrives, delivery is not acknowledged: a result drained in the instant before the hang-up
  reaches the backend, or already written back when the client withdraws the call, is lost with the reply. The
  window is the round-trip of one message, not the length of the wait.
- **A private errand is opt-in, because lich cannot tell a subagent from its session** (`internal/relay`,
  `SendPrivate`): a subagent or workflow step reaches lich through the session's own `lich mcp` and
  `LICH_SESSION_ID`, and an MCP call carries nothing that names its caller (Claude Code 2.1.288 sends only
  `claudecode/toolUseId` and `progressToken`). One that sends without `private` is the session to lich: a sibling's
  no-ticket collect can take its result and the note lands at the session's prompt, both measured on 2026-10-03.
  A private result is announced nowhere, so only its ticket recovers it, and when the ticket's hour runs out only a
  wait on that ticket hears it expired: one that comes back after another hour, or never, is told nothing. It covers tickets, not ownership: the sessions and worktrees a step opens still come from the session, their
  marks and stalled toasts land on its card, and closing them is the workflow's own job.
- **A relayed Enter is timed against silence, not against the target** (`internal/relay`, `awaitSettled`; typed deliveries only, a Claude Code session with its mod is handed the message, see below): lich
  presses Enter once the target's PTY has been quiet for `defaultSubmitDelay`, because nothing here can read a TUI's
  screen to know it has taken the paste in. The window that opens on the target's own keyboard is closed rather
  than lived with: from the paste to the Enter its keystrokes are held and written at the prompt the Enter leaves
  behind (`terminal.HoldInput`), so what a person types there is late by the length of the window instead of
  submitted inside somebody else's message. Held, not dropped — but held is still not typed: an interrupt reached
  for in that window lands after the delivery, on the turn the delivery started. On Windows that quiet is the whole instrument — ConPTY hands a child key
  events rather than bytes, the bracketed paste markers do not survive, and every provider TUI then guesses at where
  a paste ends from timing alone. A target that repaints on a timer of its own never goes quiet and gets its Enter
  at `defaultSettleLimit` regardless, which is the case this cannot tell from a paste still arriving.
- **A relayed message reaches a Claude Code session through its mod, and every other session by typing**
  (`internal/relay`, `deliver` and `handOff`; `terminal.SubmitPrompt`): the other seven providers have no mod
  system, so a task, a result note and a scheduled prompt are typed at them with every trap the typed-delivery bullets above
  describe. A Claude Code session is typed at too while no mod polls from it: lich-plugin older than 0.14.0, mods
  off, a folder not trusted yet, or a session so new its mod has not polled. A session whose mod polls is handed
  the message while its user is mid-sentence, since nothing is typed into that line. Delivery through the mod is at
  most once: a prompt no poll collected is typed instead, one a poll collected is never typed again, so a collected
  prompt lost on the way (the mod reloading as the response lands) is reported unread, not sent twice. A hook that
  drops the prompt is reported undelivered with its reason, except at a session that was mid-turn, whose ack lich
  stops waiting for after the receipt window.
- **A merge is announced only to a card running at that moment, and one made outside lich up to a minute late**
  (`internal/relay`, `AnnounceMerge`; called from the Pulls screen's merge and by the pull request watch,
  `internal/project` `RunBranchPRWatch`): a merge on github.com, with `gh` in a terminal or by an agent is found by
  the watch, the one reader of a checkout's pull request, which the cards' badges hear too. Each minute it reads,
  one `gh pr view` each, every checkout a live agent runs in and every one whose last read found an open pull
  request; any other is read only when a window asks (its branch or head moved, a turn ended there, the window came
  back into focus). A merge is announced only when one read saw the pull request open and a later one sees it
  merged, so one that merged while lich was closed, or before any read saw it open, is never announced; a card
  parked while the merge is seen, or never opened, stays believing it is open. What was read and what was announced
  live in memory, so a restart forgets both: a merge already announced is not repeated, because the next read
  finds it merged without having seen it open. The session that opened one of the checkout's sessions hears it
  too, wherever it runs, but only that one: the session that opened the opener does not. The notice also lands as
  a delivery like any other, so a cleanup clicked in the second before it starts a turn finds no session mid-turn,
  asks nothing, and removes the checkout with the notice still on its way.
- **Only a Claude Code subagent becomes a lich card** (lich-plugin's `hooks/agent-cards.js`, its
  `docs/agent-cards.md`): the plugin's mod takes the model's general-purpose `Agent` call and runs it as a lich
  session with `lich open --subagent --prompt`, in the background. The card opens in the asking session's checkout
  (`spawn.OpenSubagent`), unless the call asked for worktree isolation, which opens a worktree of its own; it is
  filed with the asker (in the asker's folder, or one named after it that the asker is filed under too), its
  errand is kept past `ticketTTL` for as long as the worker runs,
  the asker hears once per block when the worker waits on a permission, and the report comes back whole in a
  `[lich]` note through the asker's mod. The worker's own mod answers for it (docs/hooks/mod-answer.md), so it is
  handed the task with no ticket and its last message is its report; a worker whose mod had not polled when the
  task went in (4 of 84 workers measured on 2026-10-09), or runs a lich-plugin older than 0.17.0, is handed
  the ticket instead. A final turn that ends blank
  ends the errand `unanswered` (`relay.WorkerUnanswered`), once the plugin posts it (docs/hooks/mod-answer.md,
  Unanswered turns). A turn ended by an API error or a refusal fires `StopFailure`, which carries no background
  list, and no `Stop` (measured on Claude Code 2.1.296), so the plugin cannot tell it from a turn whose background
  work will resume the worker and posts nothing: that errand, like one whose turn was aborted with Esc, stays open
  with nothing reported while the worker runs, kept past `ticketTTL` rather than ending at a deadline, until a later
  turn answers or the worker is closed. A worker's report,
  and a turn end that would call a ticket-carrying errand unanswered (`relay.turnCandidates`), wait for the
  errands it sent itself (`relay.awaitsOutcomeLocked`), since Claude Code does not list them as
  background work; one of those that ages out on `ticketTTL` is filed in the worker's inbox as `expired`
  (`relay.sweep`, run on the scheduled-prompt tick, so it lands up to 30 seconds late), and that notice starts
  the turn whose report is the answer. Closing a worker
  ends its errand (`relay.SessionClosed`, wired before the PTY is killed so the SessionEnd its CLI reports finds
  nothing to call unanswered) without a word to an asker that closed it itself, as TaskStop does; closed from
  its card or by another session, the asker is told it `stopped`. Who closed it is the session `lich close` runs
  in (`terminal.CloseBy`), so a TaskStop run from anywhere but the asker's own process reads as someone else's. Both reach the asker the way Claude Code's own background agent's
  completion does, one `● lich session "…" finished` line on screen and the note whole in the model's context
  (docs/hooks/mod-control.md, A prompt as a notification), through a render path Claude Code does not document:
  a Claude Code that drops it shows the raw XML as a prompt, and a lich-plugin older than 0.16.0 shows the note as
  a prompt from the plugin. Without a mod polling there the note is typed short and the report waits
  for `wait_for_answer`, because typing a long report into a TUI is what the mod route exists to avoid. A worker in
  the asker's checkout is parked once it answered and the turn it answered in ended (`relay.SetWorkerFinished`,
  `spawn.CloseFinishedWorker`) with nothing it sent still out, so typing into its card after its report means resuming it from the history; one
  on its own worktree, pinned, stopped with Esc, holding another errand, or whose asker is gone stays open. Esc in
  the asker leaves its workers running, as it leaves Claude Code's own background agents (measured on 2.1.289);
  the mod's TaskStop stops one, Claude Code's `ctrl+x ctrl+k` does not, and the asker's status line counts them
  from `lich sessions --json` every 5 seconds. Cards
  nest two levels deep (`maxSubagentDepth`, internal/terminal): a worker turns its own subagents into cards,
  and a worker it opened spawns with `LICH_SUBAGENT_CARDS=off`, so that one's stay native, and is briefed to fan
  out with them rather than with new lich sessions; nothing but that briefing stops it opening plain sessions
  with `lich open`. A worker needs lich-plugin 0.19.0 to nest, since an older mod tells a worker only by
  `LICH_SUBAGENT_CARDS=off`; under one every worker keeps its subagents native. A nested card's report climbs
  back through each worker above it, and each answers only once its own outcomes are in, so a card stopped deep
  in the tree reaches its asker as a stop, not as silence. Closing a worker leaves the cards it opened running,
  as it leaves its own. The other seven
  providers have no mod system, so nothing can take a subagent call from them, and their subagents stay inside their CLI. On Claude
  Code too, a typed agent (Explore, Plan, a plugin's), a workflow step, another plugin's spawn, a remote one and the
  subagents of a worker two levels down stay native, and the worker is reached with `send_to_session` or `lich send`, never with
  Claude Code's `SendMessage`. Settings › Providers › Claude Code turns it off with "Subagents as lich sessions"
  (`store.SubagentCards`), which reaches the mod as `LICH_SUBAGENT_CARDS=off` at spawn, so a session already
  running keeps the value it started with.
- **A managed plugin can hide the Stop a worker's answer waits on**: on a machine with managed settings, Claude
  Code's builtin `cc-plugin-sec-default` sends `classic.Stop` on past the plugins a person installs (measured on
  2.1.306), and no mod call says so. The worker's mod then answers from `turn.complete` alone, held back only while
  `$.agent.list()` shows a subagent at work (lich-plugin's `docs/mod-answer.md`). A turn that handed a background
  shell, monitor or workflow off answers with what it said when it did, and the errand takes that first answer, so
  the report of the turn that resumes later is lost. A lich-plugin older than the release carrying that fallback
  never answers such a worker, and its errand stays open until the worker is closed.

## Git, worktrees and review

- **A project's gh account governs gh, not git**: `vcs.account` (`internal/project/ghaccount.go`) puts one
  account's token in `GH_TOKEN` (`GH_ENTERPRISE_TOKEN` on an enterprise host) for every gh call lich makes for
  that project, and in the environment of every session it spawns there (`internal/terminal/sandbox.go`
  `withAccountToken`; a confined one only through the sandbox's own grant). A project's Services
  (`internal/terminal/service.go`) are spawned without it and run gh as gh's active account. Inside such a
  session `gh auth switch` changes nothing, and the token is the one read at the spawn: an account logged in
  again or revoked reaches the session only when it is spawned again. An https push through gh's credential helper follows the
  token; an ssh push still rides the remote's ssh key and signs with the global `user.email`, so a PR can be
  *read* by one account and its commits *land* under another. lich never writes `user.email`, and the Version Control settings still print both identities
  without comparing them — noreply forms, vanity domains and org aliases make a string comparison a
  false-positive farm. The Commits tab of the Pulls screen lists each commit's author and compares it with
  nothing either, so a branch built under the wrong identity says so nowhere in lich.
- **A worktree off a local branch waits on a fetch first** (`internal/spawn/spawn.go` `newerOf`,
  `internal/project/freshness.go`): `lich open --worktree`, a subagent's worktree and `open_session` fetch the
  base's upstream to start from the newer of the two, so against a remote that does not answer the session
  appears only once the fetch gives up (`baseFetchBudget`, 60s). A failed fetch starts from the local branch,
  which may be behind without anything saying so. The create reuses that fetch for `baseRefReuse` (15s,
  `internal/project/worktree.go`) instead of fetching again, so a commit pushed in between is not in the
  worktree.
- **A working-tree row carries what git can name, and nothing else**
  (`internal/project/carry.go`): the copy is `git diff HEAD --binary` applied to the new checkout plus the
  untracked files `--exclude-standard` lists. What `.gitignore` covers is not carried from the forked
  session at all — and the fork is not without it either, which is the trap: `seedWorktree` copies the
  ignored files the project names (`.env*`, or `.worktreeinclude`) out of the **main checkout** into every
  new worktree, so an ignored file the forked session had edited arrives as the main checkout's version of
  it, silently disagreeing with the tracked work that came from the fork. `node_modules` and the rest of
  the build output are not carried at all. Everything arrives
  unstaged: which changes the source had staged is a fact about the commit *it* is preparing. The row is
  offered only when the base is the forked session's own branch, because the patch is read against that
  checkout's HEAD; picking any other base branches off it at its last commit, the way every fork did before.
  And it rides on the fork entry itself, so the five providers whose CLI only resumes (README's provider
  table) cannot reach it either.
  The **+** dialog offers the same row for the project's own checkout, never preselected. The ignored-file
  trap does not apply there, because `seedWorktree` reads the same checkout the copy does, and no provider is
  out, because the worktree exists before any session opens on it. What it adds is a snapshot that is not
  atomic: a session still writing in that checkout while the diff is read and the untracked files are copied
  can leave the new worktree with a mix of two moments. The source is only ever read, never cleaned, so
  nothing is lost there.
- **git status is polled**: every shown path is re-read each `gitPoll` (`native/cmd/lich-native/model.go`,
  `pollGit`); the lich plugin's `session-touched` hook nudges an immediate refresh.
- **A read must not write the index** (`internal/project/git.go`, `readOnlyGit`): git status and git diff refresh a
  stat-stale index and take `.git/index.lock` to save it, so a poll that does so fails an agent's own `git commit`
  in the same checkout on "index.lock: File exists" (measured at 5% of commits against a busy poll). `gitQuiet`
  and `readGit` run every call as `readOnlyGit`. A new call that diffs or stats the work tree goes through one of
  them, never plain `runGit`; `--no-optional-locks` alone is not enough, because porcelain diff ignores it and
  only stops refreshing with `diff.autoRefreshIndex=false`.
- **The status badge has a single source** (`internal/project/status.go`): the branch, the HEAD commit and the
  dirty count all come out of one `git status --porcelain=v2 --branch` parse. A git release that changes those
  records breaks all three together rather than one at a time, and there is no second call left to disagree with
  the first — `Branch` still asks `symbolic-ref`, but nothing on the polled path calls it.
- **A project opened from outside the window is matched by the spelling of its path**
  (`internal/project/project.go`, `Identify`; `internal/spawn/projects.go`): `lich open --project <dir>` normalizes
  what it is handed — `~` expanded, cleaned, refused unless absolute — and then matches that string against the
  open projects and the workspace's history. It stops there. A symlink to a directory, or a bind mount of it, is a
  different string and opens a second project on the same real directory: two rows, two ids, two tabs, and every
  path-addressed lookup lich makes (which project a checkout belongs to, which gh account its calls run as)
  answering with whichever the query reached first. The window's picker has the same hole — it takes the path
  zenity hands back — so resolving symlinks here would make the two surfaces disagree about which project a
  directory *is*, which is worse than the duplicate. The trap is a workspace where `/home/you/work` and
  `/home/you/src/work` are the same checkout: nothing on screen says the two tabs are one directory.
  Any directory on the machine is one `open_session` away from a tab, with no confirmation on the way. It is not a
  new privilege — the agent already runs as you, in a shell that can read
  the same disk — but it is new visibility, and a card it opens there is a card with a PTY in it.
- **A diff's context expander reads GitHub, one round-trip per gap** (`internal/project/filelines.go`):
  `FileLines` tries the local object first and asks the contents API when the clone does not have it, which
  on the Pulls screen is the normal case — the branch under review is usually not one this clone ever
  fetched. The call site is a promise from an RPC, so nothing there says a click costs a network round-trip
  and a rate-limit unit against the project's gh token. Anything that multiplies the clicks multiplies
  that: prefetching a file's gaps on mount, incremental ±20 stepping, or an "expand everything" control
  would each turn one reader's file into dozens of calls. The per-request line cap is the only bound, and
  a gap wider than it is several calls already. Two things follow from the same shape. The expander needs
  the revision the diff's new side stands at, so a source that cannot name one has no expander at all
  rather than a wrong one — a pull request whose detail carries no commits.
  And there is no expanding *past the last hunk*: a unified diff carries
  no file length, so nothing here knows whether anything follows it, and an affordance drawn there would
  be a no-op on every file whose change reaches the end.
- **A pull request's image costs GitHub calls the clone could not answer** (`internal/project/blob.go`): `/blob`
  reads an object this clone lacks from the contents API, the way `FileLines` does, and the old side stands on
  the merge base, which gh does not report (`baseRefOid` is the base branch's tip, wrong once the base moved).
  `PullRequestMergeBase` asks GitHub for both tips, and computes the merge base locally only when the clone
  holds both, else asks the compare API too. The window asks it once per pull request head, shared by every
  image card, and again only after a failure; each image side the clone lacks is still one contents call, and
  a GitHub read is checked against the size cap only after the whole body arrived. SVG is refused on purpose:
  git diffs it as text, and the window has no SVG renderer. One side
  is capped at `maxPreviewBytes` (20 MiB) and held whole in memory, and a Git LFS file shows the pointer's
  text diff, never the image. The window decodes in Go (`native/cmd/lich-native/blob_preview.go`) and `/blob` serves only what it
  decodes: PNG, JPEG, GIF (its first frame only), WebP and BMP, so AVIF, ICO, HEIC, TIFF and PSD show the
  no-preview state. It refuses an
  image whose header claims more than 40 MP before decoding it, holds it scaled to 2048 px on its longest
  side (no zoom), and never draws a PDF: Open in default app (`system.OpenFile`) reaches only a file in the
  working tree, so the old side of a change and a pull request's file have no way out but GitHub's page.
- **A line revert rebuilds its patch, and some of it cannot be done** (`internal/project/revertlines.go`): the
  panel sends line numbers and the text it drew, never a patch, so a file that moved since the draw is refused
  instead of reverting whatever those numbers point at now. The index is reverted with the working tree when it
  holds the change; when it holds a third version of the same lines (staged, then edited again) the revert is
  refused whole and Discard is the way out. A renamed file offers no line revert, since its old side lives under
  a path the one-file diff cannot see. The Undo is the reversed patch the window keeps for as long as the toast
  is on screen (`revertUndoFor`, `native/cmd/lich-native/dock_review.go`); nothing on disk records it after that. And a revert does exactly the lines it
  names: reverting part of a block can leave code that no longer compiles, which is the reader's call to make.
- **A Markdown preview shows the file's new version, never the change** (`native/cmd/lich-native/dock_review_markdown.go`):
  the Review card's preview renders the whole file as the Pulls screen renders a PR body, with nothing marking
  what was added or removed, no line to hang a review comment on and no selection to inject. It reads the
  working tree (`project.ReadFile`, as the Code tab does); on the Pulls screen an added file renders its patch,
  and an edited one is read at the pull request's head (`project.ReadFileAt`), from GitHub when the clone does
  not hold it, under ReadFile's size cap. A relative image
  shows its alt text (the renderer fetches nothing) and raw HTML is left out. A new version of the file puts
  the card back on its diff, as it reopens or folds a card by its size.
- **Any push resolves a thread sent to a session** (`internal/project/threadfix.go`): Send to session on the
  Pulls screen replies on the thread at once and resolves it when the pull request's head first moves after the
  send, replying with that head's short sha. lich cannot tell the commit that fixed the thread from one that did
  not, so an unrelated push resolves it too, and a fix that never gets pushed leaves it waiting. The head is read
  once a minute per waiting pull request, so the resolve lands up to a minute after the push. A pull request
  that closes, or a checkout that is gone, drops its threads without a word on GitHub. A thread whose resolve
  GitHub keeps refusing (deleted since) is retried every minute until its pull request closes. The replies are
  written in English, whatever the interface and prompt languages: the reviewer reading them is someone else.

## Persistence and history

- **A workspace database only moves forward** (`internal/store/migrate.go`): `PRAGMA user_version` counts
  the migrations applied, and a lich that finds a number above the ones it knows refuses to open the database,
  in a dialog naming both, rather than write its defaults over columns it has never heard of. There is no
  migration down: going back to an older lich means installing the newer one again or starting a fresh
  workspace. The refusal only exists from the release that stamped version 1; a lich before it reads no version
  and opens anything, which costs nothing until the first migration after version 1 adds a column. The trap for
  a contributor: `schema` and `legacyAlters` are frozen at the version-1 shape and every change after it is an
  appended entry in `migrations`, never an edit to the `CREATE TABLE` and never a line in `legacyAlters`: the
  version-1 step is the one step every database that already exists has long since run, so a column added
  there reaches none of them and every read of that table fails.
- **The workspace database is three files while lich runs** (`internal/store/store.go`, `open`): WAL mode
  appends commits to `lich.db-wal` beside `lich.db`, with `lich.db-shm` as its index, and folds them back into
  `lich.db` at checkpoints and on a clean close. Copying `lich.db` alone off a running lich, as a backup or a
  move, leaves the commits since the last checkpoint behind; copy all three, or close lich first. With
  `synchronous(NORMAL)` a power loss can drop the last commits, never corrupt the file.
- **The history's branch is read live, so a row whose checkout is gone has none** (`internal/project.BranchesOf`):
  the branch a row shows is not the one it stores — a worktree keeps the name it was created with while an
  agent moves the branch inside it, so the stored snapshot dates the close and only git can say what the
  checkout is on now, which is what the row draws. The batch runs once per settled search, which
  also means a branch that moved while the palette is up is stale until the query changes or the palette is
  reopened. A checkout removed behind lich's back has no branch to read and no session to resume: that row
  says `checkout gone` and offers to forget
  itself, which is the only way such a row is ever collected — `PurgeWorktreeSessions` never ran for it,
  because the removal never went through the app.
- **The history's external conversations are told from lich's own by a stone, and stones only start at schema
  version 8** (`internal/store/external.go`, `internal/terminal/conversations.go`): a provider keeps its
  transcript after lich deletes the session row, so the listing leaves out ids a row holds and ids a deleted row
  held (`provider_session_tombstones`, kept by triggers). A row deleted before that table existed left no stone,
  so its conversation lists once, as if started outside lich, on every provider but Claude Code, which carries
  the name lich spawned it under (`docs/providers/claude.md`, Transcript & cost).
- **An external conversation matches a project only in a checkout's own directory** (`store.ExternalSessions`):
  a conversation started in a subdirectory of a project is not offered, because a session lich opens runs in
  the project's directory or a worktree and nowhere below it. Checkouts come from `git worktree list`, run for
  every project with a `.git` on each listing (about 300 ms over 30 projects); a project that is not a
  repository owns only its own directory. Search reaches an external conversation by its title, project and
  path; what was said in it is indexed only after it is adopted and closed, the same as any session.

## Plugin install and upgrades

- **A plugin install is pinned, so the harness never updates it on its own** (`internal/agentplugin/compat.go`,
  `claude.go` `claudePinMarketplace`, `codex.go` `codexPinMarketplace`): lich installs the newest plugin release inside the range it
  speaks and declares Claude Code's and Codex's marketplace at that release's tag, so a plugin release this lich
  would not parse never arrives behind its back. The price is that a newer compatible release only lands through
  lich's own update prompt, and a marketplace the user pointed somewhere else is re-pointed on the next install.
  Neither CLI can change a declared ref, so an install removes the marketplace and adds it back; on Claude Code the
  remove also uninstalls the plugin, and an add that fails there (offline mid-install) leaves it uninstalled until
  the next attempt, which the install prompt then offers. An install that was already outside the range, from
  before the pin or by hand, is caught at startup from its installed version, and again by the `X-Lich-Plugin`
  header its hooks send (docs/hooks/README.md) once a session reports. A report whose payload lich refuses never
  reaches that check, so a plugin old enough to send one only shows up in the startup prompt and the log.
- **A `lich` upgraded under a running lich is refused by a call the backend does not have**
  (`internal/rpc/rpc.go`): a package manager replaces the binary without restarting the backend it is serving,
  so the new CLI, and the `lich mcp` of every session opened after the upgrade, speaks to an older backend until
  lich is restarted. Every call they make takes one options object (`spawn.OpenOptions`, `relay.SendOptions`,
  `store.CostTotalsOptions` and their siblings), so an option the backend does not know yet is dropped and one
  an older client does not send is its zero value; the version probe `system.Diagnostics` takes nothing at all,
  which has nothing to misalign. What is left: a method newer than the backend is refused as unknown, and the
  release that moves a call to the object (a parameterless one included, the day it takes one) is refused by a
  lich from before it, once, with "want 1 arguments, got N" (or the other way round); a backend from the typed
  contract on reads an empty argument list as the zero options, so only the older one refuses. Every call `lich`
  and `lich mcp` make goes through a method `api` declares (`callMethod` in `internal/cli/transport.go` is the
  one way the CLI posts), whose request `api`'s `TestEveryRequestIsOneObject` holds to one struct, and
  `internal/cli/wire_test.go` posts each one as a client of another release. The window's calls go through the
  same declarations (`native/backend`), so a window from another build, pinned with `LICH_NATIVE` or replaced
  on disk by an upgrade under a running backend, is answered the way a `lich` of another release is, and reads
  the `/events` stream through the same declarations.
- **A release's highlight is read from the binary, so fixing it after the tag fixes nothing a user sees**
  (`internal/patchnotes`): the What's new dialog parses the `CHANGELOG.md` embedded at build time — the alert
  blocks under the version heading included. Editing that release on GitHub, or the changelog on `main`,
  changes the release page and no dialog anywhere; the users who already updated have recorded the version as
  seen, and the ones who have not will read the binary they install. A wrong headline is a patch release. The
  update toast reads nothing from the release but its tag, on purpose: the toast says a release exists, and
  the dialog is where it speaks.

## Profiles

- **A profile is one provider's** (`internal/profilestore`): which providers have profiles on which OS is
  README's provider table, and how each is laid out, logged in and moved is `docs/providers/`, Profiles.
  `internal/xdgprofile` lists the link-home providers each platform has (`supported_linux.go`,
  `supported_windows.go`, `supported_other.go`), and the native window shows profile rows for the same ones
  (`profiles_xdg_*.go`). The
  profile slug of a session row belongs to that row's provider: a Codex profile named "work" and a Claude Code one
  named "work" are two profiles.
- **A profile that links to your own directories shares their writes, and keeps what a tool creates inside it**
  (`internal/profilestore/linkhome.go`, `layout.go`, `internal/claudeprofile/profile.go`, `internal/ompprofile`):
  Claude Code, oh-my-pi, opencode, Cursor CLI and Kiro CLI profiles link Personal's own content, and a tool writes
  through a link into its target. So `claude plugin install` run in a profile writes `enabledPlugins` into the
  one shared `settings.json`, and a plugin a work account installed reads as enabled in Personal, where it is not
  installed; `/config` changes in any profile change them all. An entry the profile already holds, link or
  file, is never replaced: whatever a tool creates inside a profile is the profile's from then on, and an entry
  you make later with the same name is not linked there. A Claude Code `settings.json` written there first stays
  the profile's, an oh-my-pi profile with an `extensions/` of its own does not get lich's extension through it and
  its sessions report nothing, and a Kiro update that replaces `tui.js` by a rename inside a profile leaves that
  profile its own copy, which no later update in yours reaches. On Windows a file link needs Developer Mode:
  without it Claude Code's, oh-my-pi's and Codex's linked files are copies that drift on the first edit, and an
  opencode, Cursor CLI or Kiro CLI profile leaves the file out until the first spawn after Developer Mode is on.
  A sandboxed session gets the profile bound writable, and its links reach only what the sandbox already exposes
  at their targets.
- **An oh-my-pi, opencode, Cursor CLI or Kiro CLI profile still sees the provider API keys in lich's
  environment** (`internal/ompprofile`, `internal/xdgprofile`): each drops the variables that would point it at
  another account or another profile's state (`docs/providers/`, Profiles), but `ANTHROPIC_API_KEY`,
  `OPENAI_API_KEY` and the rest are passed through, so a profile that has no login for a provider spends the key
  the environment has, as Personal does.
- **Continue in refuses a turn in progress only where the session-state hooks report one**
  (`terminal.ContinueIn`; what each provider moves and leaves behind is `docs/providers/`, Profiles): a Claude
  Code without the lich plugin, or an oh-my-pi without the lich extension, reports no turn and is moved mid-turn.
  On Claude Code the prompt history and the project's `lastSessionId` stay in the profile the conversation left,
  so `claude --continue` there points at a conversation it no longer holds. A session born in a profile deleted
  since is refused at spawn with "claude profile no longer exists", including its resume: the conversation was
  deleted with the profile.
- **A Claude Code profile's plugin versions live in a store lich never empties**
  (`internal/claudeprofile/plugincache.go`; the layout is `docs/providers/claude.md`, Profiles): each version a
  profile names is moved into `plugin_cache/` beside the profiles and linked back, which is what keeps Claude
  Code's 14-day orphan sweep from deleting a plugin only one profile has. Versions no profile names any more stay
  in the store, since a session started before an update may still run one; deleting `plugin_cache/` with every
  Claude Code closed only costs the downloads. A profile made before `CLAUDE_CODE_PLUGIN_CACHE_DIR` was set keeps
  the marketplace link it was made with and clones each marketplace again into a directory Personal no longer
  reads, until that link is removed.
- **A profile's directory never moves, and lich's config dir decides where it is**: on macOS Claude Code keeps a
  login in the Keychain under a service named after a hash of the config directory string
  (`internal/quota/claude_storage.go`), so a renamed profile keeps its slug and directory and changes only its
  name, and a moved one would lose its login. Deleting a profile deletes that Keychain item with
  `security delete-generic-password` (`quota.ForgetClaudeLogin`), which has not run on a Mac yet. The
  directory is built from `os.UserConfigDir()` on every call, so an `XDG_CONFIG_HOME` (or `APPDATA`) that
  changes between launches points lich at another, empty set of profiles. A `task dev` rig has its own set
  (`profiles-dev`), so its profiles need their own login.
- **The version Settings shows is the one lich's own environment gets** (`internal/providers/version.go`):
  `--version` runs in lich's working directory with lich's environment, not in a session's checkout. A
  version-manager shim that picks its tool per directory (mise, asdf) can answer one version there and start
  another in a project that pins its own, and the "Also on PATH" list compares those same per-lich answers.
  All eight providers print a version (measured on Claude Code 2.1.299, Codex 0.162.1, Antigravity 1.3.5,
  opencode 1.18.35, oh-my-pi 18.8.9, Crush 0.98.1, Cursor CLI 2026.10.01, Kiro CLI 2.21.0); a CLI whose
  `--version` hangs past 10 s or prints no version shows its path alone.
- **The binary setting and lich's own plugin checks still answer for Personal**: a profile's session runs the
  configured Claude Code binary, so a wrapper that exports its own `CLAUDE_CONFIG_DIR` or a token still wins
  over the profile, and the same binary answers the profile's `auth status` and installs its plugins. The
  start gate reads the profile's own plugin state, but the nested-subagent-cards check and the MCP servers a
  card lists read Personal's, so a profile on an older lich plugin than Personal's can be offered cards its
  plugin cannot open: one whose follow-up install failed, or one still waiting for it.
- **A profile's lich plugin follows Personal's, after it and in the background**
  (`internal/agentplugin/profiles.go`): an install or update of the lich plugin for Claude Code or Codex,
  from Settings or anywhere else, runs in Personal and returns; then the same pinned install runs as each
  other profile of that provider whose version differs, one at a time, logged in or not. A backend start makes
  the same pass, which is what reaches a profile logged in before profiles got the plugin at login; it is the
  start and not the next spawn because a pass clones the marketplace, and a spawn would wait on it. A start asks
  for the release list only when a provider has a profile besides Personal. While Personal itself is behind the
  newest release this lich speaks, a profile that has the plugin keeps its version and one that has none gets the
  newest, so the two can differ until Personal is updated. A profile whose install failed is named in an error
  toast of the native window, in the CLI's own words, and is tried again at the next start or update. A Claude
  Code profile's pass removes the marketplace clone it links to Personal's and clones it back, so a Personal
  session starting during a pass can find the clone missing for the seconds that takes.
- **A setting changed inside a Codex profile does not last** (`internal/codexprofile/config.go`; the layout is
  `docs/providers/codex.md`, Profiles): the profile's `config.toml` is rebuilt from Personal's before every spawn,
  so a `/model`, `features enable` or MCP server added inside a profile is gone at its next session, a setting
  Personal removes leaves every profile then, and a trust answer only Personal withdrew stays in the profiles
  that gave it.

## Sandbox

- **The sandbox confines a working agent, not hostile code** (`internal/sandbox`): namespaces and mounts on
  Linux, a path policy on macOS, and nothing else — no seccomp filter, no Landlock ruleset. The network is
  never cut (the agent needs its API and the plugin's hooks report over loopback), so anything readable
  inside is exfiltrable, and `~/.config` *is* readable: a token stored there is in reach. `gh`'s is not
  one of them — it lives in the system keyring, which is why reaching it takes the flag below. Every session also carries `LICH_TOKEN`, so a confined agent can call lich's own RPC over
  loopback: a method that reads a host path it was *handed* would copy anything into reach of the sandbox,
  which is why the attach flow opens the picker inside the backend (`drop.Attach`) instead of taking a path. The private home is writable and vanishes with the session, so a dotfile an agent writes is gone
  next spawn with nothing saying so. On Ubuntu and Debian the kernel may refuse the user namespace outright
  (an AppArmor policy), which surfaces as bubblewrap's own error in the card and no session: `Available` is
  `exec.LookPath` and stays that way, because only a spawn can ask the kernel, so nothing before the card
  can refuse the rung. What answers ahead of time is `lich doctor`, whose `sandbox` check opens one confined
  child and reads a file in the checkout and a file in the home the backend replaces: it is a diagnosis, not
  a gate, and a machine where the probe fails still lets the user turn the sandbox on and watch it fail. `~/.ssh` is not
  mounted at all: a push over ssh from inside a confined session fails unless the project hands over the ssh
  agent (below), and lich's own PR flows run outside the sandbox and are unaffected either way. The distribution's `/etc/ssh/ssh_config.d` drop-ins are replaced by an empty
  directory, because inside the namespace they belong to nobody and ssh refuses to read a config file it
  does not own — so a host whose ssh depends on one of them (a corporate `ProxyCommand`, say) does not have
  it inside the sandbox. The display server's socket *is* mounted, because the agent's own copy and its
  clipboard image paste shell out to `wl-copy` and `xclip` and both fail without it: a confined session can
  therefore read whatever you copy, a password manager's paste included. A host without Wayland gets the
  X11 socket and its cookie instead, the wider of the two — X clients are not isolated from one another —
  and macOS gets neither, its pasteboard being a mach service rather than a socket, so a confined session
  there has no clipboard at all. macOS has no hardware here — its profile is unit-tested and has never run.
- **The two sandbox grants are all-or-nothing** (`internal/store/settings.go`, `internal/sandbox`,
  `internal/terminal/sandbox.go`): a confined session reaches the network as the user only where the project
  turned one of them on, and each is all-or-nothing. The ssh agent is handed over as a socket, so nothing
  private enters the sandbox — but the session signs with **every** identity loaded in that agent, against any
  host it can reach, for as long as it runs. OpenSSH can pin a key to one destination, and only when the key
  is added on the host (`ssh-add -h`); lich is given a socket that is already populated and can only pass it
  on whole. The socket does not travel alone: `~/.ssh/known_hosts` is mounted read-only beside it, because
  without it ssh cannot verify github.com, has no tty to ask on, and fails with "Host key verification failed"
  before the key is ever offered — the grant would hand over the credential and not the push. That file is
  public host keys, never a secret; what a confined session learns from it is the list of machines the user
  connects to. Read-only, so a host the user has never connected to *outside* the sandbox still fails inside
  it and cannot be learned there — blind trust-on-first-use is not a thing to grant an unattended agent. And a
  `known_hosts` symlinked out of a dotfiles repository is dropped like every other link in the home
  (`internal/sandbox`, `mounts.keep`; the card names it from `SkippedLinks`), which takes the whole grant down with it. That is why Settings lists what
  is in the agent, and re-reads the list every time the window regains focus: `ssh-add` is run in a terminal
  outside lich, so the tab back is the frame where a key loaded a moment ago has to already be named by the
  switch that hands it over. The GitHub token is one account's, the project's own (`vcs.account`), and it
  rides in the session's environment: the agent can read it back out of its own environment and spend it on
  anything that account's scopes allow, this repository or not. Both switches say that much in the sentence
  under them — a grant is decided in a settings pane beside a sandbox whose whole promise is that the user's
  credentials are not in there, and a switch read as "let it push with my GitHub key" is read as a smaller
  promise than it makes. Neither grant is keyed by provider — a grant describes what is inside the sandbox,
  not who runs in it — so turning one on turns it on for every provider confined in that project. It is
  resolved once per spawn, so a token gh rotates mid-session goes stale with nothing saying so, and a `gh auth
  token` that fails leaves the session with no token rather than failing the spawn. Both are off by default,
  and both are Linux in practice: the macOS profile denies reads *inside* the home while a launchd agent
  socket and gh's keyring live outside it, so a confined macOS session never lost either and the switches
  change nothing there — they are inert rather than hidden, and there is no macOS hardware here to prove it
  further. Windows has no sandbox backend, so neither switch exists.

## Terminal

- **"Unfold long pastes" copies two providers' fold rules, and nothing tells lich when they move**
  (`native/cmd/lich-native/paste_unfold.go`, `internal/terminal/paste_unfold.go`): the switch is off until turned
  on, and for Claude Code and Kiro CLI it predicts the fold from a rule copied out of one measured version
  (`docs/providers/`, Known limits). Codex, Antigravity, Crush and Cursor CLI fold with a constant and no key that
  unfolds, so they have no switch, and oh-my-pi was not measured. A paste lich thinks folds and Claude
  Code does not lands twice, one it thinks does not stays a chip, and a paste into one of Claude Code's own
  dialogs that does not fold gets the text twice; a paste lich misjudges for Kiro CLI gets a Tab, which in the
  `/` menu picks an entry. The window also unfolds what it writes as a paste itself (a batch of review comments,
  a pull request's or an issue's prompt, dropped and attached file paths), measured against the session's own
  terminal; one written into a session whose terminal the window never opened has no height to measure and lands
  folded. No caller does that today: each writes into the session on screen, or waits for a session it just
  opened to reach its prompt, which only happens after the window started its terminal.
- **A split stage puts every pane on the visible cadence** (`internal/terminal/coalescer.go`,
  `native/cmd/lich-native/terminals.go`, `setVisible`): the coalescer batches a *visible* session's output
  every 8ms and a hidden one's every 250ms, and until the stage could divide, exactly one session per window
  was ever visible — `Service.SetVisible` had one true at a time. A layout of eight makes eight,
  deliberately: a pane nobody demoted is the entire point of opening it. So the window's hot path carries
  as many streams as there are panes, and anything reasoning about that cadence from the constants alone
  will read a number that was true for one terminal.
- **A theme switch reaches only the apps that asked** (`native/gioterm/scheme.go`, `setScheme`): lich answers
  mode 2031 and sends the light or dark report on a switch, only to a terminal whose app turned the mode on. An
  app that reads the background once at boot keeps the old look until it restarts: Codex, Antigravity and Kiro
  CLI do, and Crush reads it not at all.
- **Only zsh, bash and fish get their prompt cleared on a resize** (`internal/shellintegration`,
  `native/gioterm/prompt.go`): a shell session's shell is started with scripts that mark its prompt with
  OSC 133 and ask the terminal to clear it on a resize (`redraw=1`, `redraw=last` for bash, which redraws only
  the last line). sh, PowerShell and a shell started by hand inside a session are unmarked, and a prompt line
  as wide as the terminal (a right prompt) leaves its wrapped head on screen after a narrowing, the way any
  terminal without shell integration does. Agent sessions are untouched: they run no shell rc and draw their
  own screen. The scripts are rewritten under lich's config dir at every start, so two lich builds running at once
  share the newer one's. bash marks where a command's output starts from `PS0`, which it has since 4.4: the bash 3.2 macOS
  ships marks its prompt but not that, which the resize clearing does not need.
- **A session close is a hang-up on Unix and a Ctrl+C on Windows** (`internal/terminal/pty_unix.go`,
  `pty_windows.go`): closing a card signals the agent and gives it `closeGrace` to leave, so its exit path runs —
  hooks, transcripts, whatever it writes on the way out. A ConPTY has no signal to deliver, so Windows sends the
  terminal's own Ctrl+C instead, and that is a weaker ask: an agent whose TUI reads one Ctrl+C as "interrupt the
  turn" and wants a second one to quit — Claude Code does — is still killed when the grace runs out, and nothing
  on screen says so. That the byte arrives at all depends on `heedCtrlC`: a lich started by a service passes an
  inherited "ignore Ctrl+C" to every agent it spawns, and clearing it before the first spawn is what the Windows
  close rests on.
- **The shell-env pty read ends on a printed end marker, not on the child's exit, and Windows never gets one**
  (`internal/terminal/shellenv_unix.go`, `runShellDump`): resolving PATH and friends runs the login shell on a
  pty rather than a pipe so an rc guarded on `[ -t 0 ]`/`tty -s` (nvm's and fnm's own init, among others) loads —
  but neither closing that pty from another goroutine nor `SetReadDeadline` interrupts a read already blocked in
  it (both measured directly against a blocked read: Close returns without error and the read stays parked in
  the kernel regardless, and the deadline is never enforced on this fd). So the read is driven from a goroutine
  free to outlive the call, and the dump is `echo <start>; env; echo <end>`: the read returns the moment the end
  marker arrives, which is what lets an rc that backgrounds a job (an `ssh-agent`/`gpg-agent` eval, a prompt
  tool) still hand back the dump instead of paying the full `shellEnvTimeout`. Two edges follow. A shell that
  never prints the end marker (a hung rc, a prompt waiting on input, an
  unanswered privacy prompt) is bounded only by the 5s ceiling, and a dump cut there is discarded whole rather
  than merged half-read. A shell still alive after printing it (a slow `.zlogout`, an rc that `exec`s into
  something holding the pty) also waits out the 5s, dump kept: the read ends on the marker *and* the shell's
  exit, because the marker alone called a shell milliseconds from exiting parked and refused the next
  resolution. And the reader goroutine outlives the call for whatever still holds the pty: the fd,
  the goroutine and the zombie child persist until that holder exits on its own or lich itself does, one leak
  per resolution that hits this edge, not a recurring one.
  And **Windows gets none of this**: `SHELL` is normally unset there, so `ResolveShellEnv` returns before
  `shellenv_windows.go`'s pipe-based `runShellDump` ever runs — but on a machine where the user sets it anyway
  (Git Bash, a POSIX-ish shell reached through PATH), that path still runs over a pipe, so an rc guarded the
  same way is skipped there, with no ConPTY wired in to close the gap.
- **A session's cwd readout names the host it cannot see into, and only Unix has one to name**
  (`internal/terminal/cwd.go`, `cwd_unix.go`, `cwd_windows.go`): the directory is polled off the terminal's
  foreground process group, and a foreground job that is itself hosting the shell somewhere else (tmux, ssh, a
  container) has a directory of its own that is readable and wrong. The unix readers match that job's comm
  against `shellHosts` and publish the host with no path, so the card, its tooltip and the footer draw
  `cwd unknown · inside tmux` rather than a real local directory nobody is standing in. Windows has no
  foreground process group to read a comm from and so has no host to report: a session hosted elsewhere there
  goes on naming a local path that is not where the user is typing, with nothing on screen saying so. There is
  no Windows hardware here to build the replacement against.
- **A terminal is rebuilt from a 2MB replay ring** (`internal/terminal/replay.go`,
  `native/cmd/lich-native/terminals.go`): the window builds a session's terminal from the backend's ring when it
  is first shown, and again after it reconnects. Scrollback past the ring is gone, not paged, and a mode an app
  set in bytes that have left the ring is not set on the rebuilt terminal.
- **One socket carries every session's output** (`internal/terminal/writequeue.go`): the per-session outbox
  decouples the *producers*, never the wire. A window that stops reading stalls the connection's single writer,
  so after `wsWriteTimeout` (5s) every session's output switches to the `/events` bridge at once. That is a
  second socket, and the window reads the two independently: a frame that fell back can land ahead of one still
  sitting in the stalled connection's buffer, so output is never dropped but its order across that switch is not
  guaranteed. Until then the queue holds `writeQueueDepth` frames for the whole app, and two things still wait on
  it across sessions: a push that finds it full, and the flush a session runs before its exit banner so the banner
  cannot overtake its own last bytes.

## Services

- **A service is up exactly as long as its command runs** (`internal/services`, `internal/terminal/service.go`):
  there is no shell behind it and no supervisor. A command that detaches (`docker compose up -d`, a trailing
  `&`, a tool that daemonizes) exits 0 at once and the service shows stopped while its work carries on, and a
  service that fails stays failed until someone presses Start. Stop types Ctrl+C at the PTY, so a command that
  ignores it stays stopping until Force stop. Force stop and quitting lich end the process lich started
  (SIGTERM, then SIGKILL past the PTY's grace), not what it handed off to: a container `docker compose up`
  started survives the kill of that command.
- **A service's command loads no rc file, except under fish** (`serviceArgs`): `$SHELL -c` on Linux and macOS,
  PowerShell `-NoProfile` on Windows, so an alias, a function or a `PATH` entry from the user's rc is not there.
  fish reads `config.fish` even for `-c`, so the same line can work there and nowhere else.
- **A service lives as long as its project's tab** (`services.ProjectChanged`, `main.go`): closing the tab asks
  its services to stop and kills what is still up after 10 seconds, without asking first, and reopening the
  project starts only the ones saved to start with lich; one started by hand stays down. Closing the window
  stops nothing. Quitting lich stops everything the same way, and an in-place restart after an update is a quit:
  the new lich starts the services saved to start with lich in the projects it opens with, so one started by
  hand is down after an update. Home's services, edited from Settings › Services in Global, run for as long as
  lich does, since Home has no tab to close.
- **What a stopped service printed is kept in memory only** (`services.Output`): the tail of its last run, as
  long as a session's replay, until it starts again or lich exits. After a restart of lich a service that has
  not run yet has no output to show.
- **The tray names at most eight running services** (`internal/tray`, `serviceSlots`): the menu is built once
  and its entries shown and hidden, so past eight the count above them says how many run and the window lists
  them all. The macOS menu-bar entries were never run on a Mac: they compile, and the logic they share with
  Linux and Windows is tested.
- **On Windows, Stop is Ctrl+C through ConPTY** (`InterruptService`): it reaches the console's processes as
  `CTRL_C_EVENT`, which a process that left the console or turned the event off never sees. Not measured: the
  service tests spawn real PTYs and run on Linux and macOS only.

## Window, sidebar and settings

- **A closed card outlives its session by 180ms** (`native/cmd/lich-native/sidebar_closing.go`, `exitDuration`):
  the card animates out, so the closed session is held and drawn for the length of that exit. For those frames
  the sidebar draws a card for a session the state no longer holds, and anything counting or measuring cards
  sees one card too many.
- **A folder is a name on a session, so it has no life of its own** (`internal/store/mutations.go`,
  `native/cmd/lich-native/sidebar_groups.go`): the folder a card is filed under is a column on the session
  row, and the name is the whole identity — there is no folder record to point at. A folder exists only while a
  session carries its name, so the last card leaving ends it; a rename is a write across every session in it,
  parked rows included (`RenameFolder`), and the block, keyed by the name, remounts with anything keyed off it
  but the fold (`moveCollapsed`, `native/cmd/lich-native/sidebar_prefs.go`); renaming onto a name the project
  already holds merges the two folders with no undo; two names differing only by case or a space are two
  folders; and a folder belongs to one project, since the name is scoped by `project_id`.
- **An agent closes its own session on its word that the user asked** (`internal/spawn/close.go`): `self` on
  `close_session` (`lich close --self`) lifts the refusal, and nothing checks that a person asked for it; the
  tool description is the whole guard. The close kills the caller mid-call, so its answer never reaches it.
- **An agent closes a project without the window's confirmation** (`internal/spawn/close_project.go`):
  `close_project` (`lich close-project`) stops every session in the project with no dialog, where the tab's
  close asks first when one is running. Nothing is lost, since the rows stay for the reopen, but a session
  mid-turn is cut off. Only the caller's own project and Home are refused.
- **An agent files by the name as written, like the window** (`internal/spawn/folders.go`): `lich file`,
  `lich rename-folder`, `lich color-folder` and their MCP tools match a folder exactly, so an agent that types `auth` beside `Auth`
  starts a second folder; the tool descriptions send it to `list_folders` first instead of folding case behind
  the user's back. A rename from there refuses a name no session carries, because the store would report
  success over nothing, and answers with every session that moved, which is how a merge shows.
- **A layout is a free split tree** (`native/cmd/lich-native/layouts.go`, `model_layouts.go`,
  `stage_layout.go`): a layout is stored under `lich.native.layouts`. A session sits in one pane of one layout, because its PTY has one
  size; clicking its card anywhere shows it in that layout, while its project's tab shows it alone. A split
  is refused, with a toast, when a pane would drop under 480dp (51 columns at the default 13sp, measured with
  Claude Code and Codex): a larger terminal font fits fewer columns in the same floor. Only the focused pane
  takes a dropped file; a file dropped on a pane in the background goes nowhere. A pinned card cannot be dragged onto a pane: the
  pinned block shares no carry with the panes, so its card never leaves the sidebar. The panes' drop zones are
  not SortCarry targets (`layouts_drag.go`): a new drop target over the stage belongs there, not in the carry.
  Show beside lays out only the delegates that fit the 180dp pane
  floor, counted from where the panes were drawn the frame before (`columnRoom`); the rest stay out and
  the pane header offers them. Its footer item is not an item of the Footer layout, so Settings can neither
  move nor hide it: it shows only while the session, in no layout, has delegates.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch asks
  the running lich to show its window (`singleton.Show`, `system.ShowWindow`) and exits. Raising is
  best-effort: a Wayland compositor may only mark the window urgent.
- **Settings remembers its open pane under one global key** (`native/cmd/lich-native/settings.go`,
  `settingsSectionKey`): the nav is the same list of panes in every project, so opening Settings in project B
  lands on the pane project A was reading. Whoever adds a pane about one repository's content keys its
  remembered state with the project id, not with another global key beside this one.
- **A Settings control exists only once it has an entry in the hand-written index** (`settingsSections`,
  `native/cmd/lich-native/settings_index.go`): the panes draw their rows off that list, with the section and
  group each lives in, and the search matches a row's title, its hand-picked search words and its group, never
  a description, a stored value or a theme's name.
- **Closing the window does not quit lich** (`main.go` `serve`, `internal/restart.Window`): with a session
  running, the window asks whether lich keeps running in the background or quits, and the answer can be kept
  (Settings › General › When the window closes, `native/cmd/lich-native/window_close.go`); with none running, closing it quits. Kept running, the
  backend holds every session's process with nothing on screen, and with them the pinned port (47821), the
  keep-awake assertion while a session works, the providers' spend and the desktop notifications. The tray
  icon (below) is all that says it runs. Launching lich again opens a window on it; `lich quit` (or SIGINT /
  SIGTERM) ends it. `task dev` behaves the same: kept running, the dev backend stays on 47822 until Ctrl+C or
  `LICH_DEV=1 go run . quit`.
- **The tray icon is late, and on macOS it rides AppKit through purego** (`internal/tray`, `internal/appkit`):
  lich is built without cgo, so on a Mac the backend runs AppKit's loop on its main thread through purego, as
  an accessory app (no Dock tile, no Cmd-Tab), and the menu-bar item and LaunchServices' reopen and terminate
  live on that loop. A Mac where AppKit does not load runs lich without either, and opening lich again from
  Finder then depends on LaunchServices launching a second copy. On Linux it is a
  StatusNotifierItem: GNOME draws none without the AppIndicator extension, and a desktop with no tray host
  shows nothing either. Its words are the window's interface language, so it comes up once a window has
  handed them over (`native/cmd/lich-native/tray.go`); a lich whose window never opened has no tray. `task dev` puts up a second
  icon beside the installed lich's, with the same name.
- **Restart and update end every running session, even with nobody watching** (`internal/restart.Coordinator`):
  /restart and an update stop this process and every PTY with it, so a windowless lich restarted by
  `install.sh` from outside loses its sessions with no window to show it. The window that was up closes with
  the old lich and the successor opens one of its own, so a lich that had none stays without one: the old
  `lich-native` never outlives its backend, since the calls it makes may be ones the new backend no longer takes
  (`Coordinator.Do`).
- **A window that cannot start is a dialog, never a fallback**: a `lich-native` missing beside `lich` (and not
  pinned by `LICH_NATIVE`), or one that exits at startup, on a missing system library say, ends in the error
  dialog with the reason and the log path. With no session running lich then quits; with one, it keeps serving
  it, and the tray, launching again or `lich quit` is the way back.
- **A backend holds one window at a time** (`lich native`, `docs/native-window.md`): `/events` and `/ws` hold one
  client, and a new one replaces the old with no close frame, so two windows cannot share a backend: the window
  already up, its `/events` dropped, says lich lost it (`native/cmd/lich-native/model.go`, `follow`). A focus that has to start the window again loses the
  card it named: the window comes up on the session it last showed. On macOS the window is built and run only on
  GitHub's runners (`native.yml` draws one frame against a real backend, `.github/scripts/mac-e2e.sh` opens it from
  the bundle): its keys, Retina scaling, IME and menu bar have never been looked at on a Mac. Option types what it
  composes there (Option+c is ç), as Ghostty's and Terminal.app's default does, so an agent's Alt/Meta shortcut on a
  letter (Option+P) never reaches it; Option with Backspace or an arrow still does
  (`native/gioterm/keys_option_darwin.go`). A quit or a restart (`lich quit`, the tray, an update, SIGINT or
  SIGTERM) closes it with the `window-close` event; a backend killed outright or crashed sends nothing, so the
  window shows lich as lost. `/events` replays nothing,
  so the window reads each session's status, usage and working directory from `terminal.TurnStates` on connect.
- **Only a native window the backend starts tells lich how it ended** (`restart.StartNative`,
  `restart.WatchNative`): the backend waits on the native window it starts again (the tray's Show lich after the
  window closed) and logs its exit code or the signal that killed it, with its last `VmRSS`/`VmHWM` (Linux only,
  sampled every 10s, so up to 10s old). The window `lich native` starts from a terminal is not the backend's
  child: the command exits right after, nobody waits on that window, and its death leaves no line in `lich.log`.
  Every window started either way prints into `lich-native.log` beside `lich.log`, where an event loop stuck on
  one event for 3s or more is reported with every goroutine's stack (`native/cmd/lich-native/stall.go`); a
  stall inside `w.Event()` itself, in Gio's own dispatch, looks like an idle window and is not reported.
- **The native window draws its own title bar, and a few desktops keep theirs**
  (`native/cmd/lich-native/caption*.go`, the Gio fork's `app/caption_unix.go` and `app/os_x11_caption.go`):
  the topbar is the title bar, with the window's buttons drawn by lich in each desktop's look (Windows 11,
  GNOME's round buttons on the side and in the order of `button-layout`, Breeze under Plasma) and macOS's own
  traffic lights in a 78dp reserve. A desktop that decorates its windows itself keeps its bar and lich draws no
  buttons: on Wayland, a compositor that answers the request for client-side decorations with server-side
  (KWin was measured granting client-side; what Hyprland, sway and niri answer was not measured); on X11, a window manager whose `_NET_WM_NAME` is on the fork's
  tiling list (`tilingWMs`), so a tiling window manager missing from it gets buttons it cannot use. Under
  Plasma the buttons are always Breeze's, whatever decoration theme is set. `button-layout` is read once, at
  start, and only GNOME's portal key. On X11 the window has square corners and no shadow (Gio's surface has
  no alpha), and a WM with no `_NET_WM_MOVERESIZE` cannot move it. Windows 11's Snap Layouts flyout does not
  open over Maximize (it needs `HTMAXBUTTON` from the fork's hit test). On macOS the traffic lights sit where
  AppKit puts them in its 28pt title bar, not centered on the 44dp topbar, and a double click on the empty
  strip does not zoom; neither was looked at on a Mac.
- **The native window finds fonts through fontconfig on Linux and by scanning the font folders on Windows and
  macOS** (`native/fonts/locate_fontconfig.go`, `native/fonts/locate_fontscan.go`): neither ships fontconfig,
  and a window that shelled out to `fc-match` there closed at launch. The scan (go-text's fontscan) marks a
  family monospace only when every glyph shares one advance, so the terminal and editor font pickers on Windows
  and macOS leave out the dual-width Nerd Font families (the ones not named "Mono", whose icons take two cells)
  that fontconfig's `spacing=dual` lists on Linux; the "Mono" variant of each is offered. Its family names come
  from parsing one file per family, a few hundred on a stock Windows, the first time a font picker opens.
  On Linux each face fontconfig names is kept for the rest of the run (`fonts.Locate`), since every lookup is an
  `fc-match` subprocess inside a frame: a family installed while lich runs, after a terminal or the editor
  already asked for it, keeps drawing in the substitute fontconfig gave until lich restarts. Choosing it in a
  font picker for the first time is not affected.
- **The Chinese font the native window downloads reaches its interface, not its terminal on Linux**
  (`native/fonts/download.go`, `native/glyphs/raster.go`): the terminal finds a glyph its family lacks through
  fontconfig (`fonts.Covering`), which never sees `<user cache dir>/lich/fonts`, so with no Chinese font
  installed a branch name or agent output in Chinese still draws as tofu in the terminal while the cards and
  Settings around it read fine. Installing a Chinese font system-wide covers both. Windows and macOS find the
  terminal's fallback by scanning the system font folders, which leave lich's cache out the same way.
- **Following the OS's light or dark is polled on macOS**: a theme's `pair` and `lich theme switch` reach the
  window through the backend's `theme-selection` event. Following the OS
  reads the freedesktop appearance portal on Linux and `AppsUseLightTheme` in the registry on Windows, both
  watched. macOS polls `defaults read -g AppleInterfaceStyle` every 3 seconds, so a switch reaches the window
  up to that late: observing `AppleInterfaceThemeChangedNotification` needs an Objective-C shim no one
  could compile or run without a Mac. The BSDs read no scheme and draw dark.

- **A session's browser is a second Chromium, one per card, drawn from frames** (`internal/browser`,
  `docs/browser-tab.md`): a headless Chrome, Edge, Chromium, Brave, Vivaldi or Helium already installed,
  whose page reaches the session's browser tab as a JPEG screencast over `/events`, at the pace the window
  takes frames. The native window cannot hold a webview: on Linux it speaks Wayland without GTK. So the pane
  has no DevTools, shows the arrow whatever cursor the page asks for, loads what the page opens in a new tab or window in the page itself, and leaves the page's own dialogs out; Open in
  a window moves the page to a real Chromium window for those, reopening it on the same profile with every
  cookie, since chromedp cannot show a headless process in place. A half-filled form or sessionStorage is
  lost on that move. Closing the card, or the session's process ending, closes the browser; Continue in,
  which ends the process and spawns it again in the same card, keeps it. Closing that
  window does not close the session. A headless browser no pane shows and no one used for five minutes is
  closed; one a pane shows, or a window, never is. It loads only http, https and about:blank. Page info never
  includes a form's value. Screenshots are files. There is no tool that runs JavaScript in the page. It is
  not a sandbox: the page runs as the user, in an ephemeral profile. Chromium needs its own sandbox, and on
  Ubuntu 23.10 and later AppArmor refuses it the user namespace unless the browser ships an AppArmor profile,
  as the distribution's Chrome, Edge and Brave do: one unpacked from a tarball fails to start with "No usable
  sandbox!", which the agent reads as the browser's error.
- **A note left on the page names only what the page shows lich** (`internal/browser/notes.go`,
  `native/cmd/lich-native/browser_notes.go`): the pick asks the page for the element under the pointer, so an
  element inside an iframe is picked as the iframe and one inside a closed shadow root as its host, and a
  selector picked inside an open shadow root finds it again only from that root, where `browser_click` does
  not look. The React component comes from the page's fiber, there only in a development build, and its file
  and line from `_debugSource`, which React 19 dropped. Pins follow their elements by looking them up again
  every 400 ms while notes wait, and every 30 ms for a moment after the wheel turns, so an element the page
  moves on its own is outlined late. A pin is cut to every ancestor whose overflow is not visible, up to a
  fixed one, so an absolutely positioned element that escapes such an ancestor is cut anyway, and one a
  sibling covers keeps its pin. Notes not sent yet
  live in the window: closing it, or the browser, loses them.

- **On macOS, a JavaScript dialog a page opens before its first paint leaves the pane black behind the dialog's
  card** (`internal/browser/chrome_view.go`): headless Chrome there paints a page some time after its load, and
  a page held by a dialog cannot paint, so a screencast started then has no frame to send until the dialog is
  answered (Chrome 155 on the macos-latest runner, measured). A dialog opened once the page has been drawn, as
  one an agent's click opens, shows the page behind it. Linux and Windows paint before the load returns.

- **A session browser's downloads go straight to the user's Downloads folder** (`internal/browser/downloads.go`,
  `session_downloads.go`, `docs/browser-tab.md`): `XDG_DOWNLOAD_DIR` on Linux, `~/Downloads` on macOS and Windows,
  even for a Windows user who moved that known folder. Chrome writes each one under its download id into a staging
  folder in the session's profile, and only a download a page announced moves to Downloads, once complete. The
  download behaviour is the whole browser's, so what Chrome downloads on its own, with no page (4 KB to 33 MB files
  on the macOS and Windows runners), lands in staging, is never moved and goes with the profile when the browser
  closes; until then it takes the profile's disk. The profile sits in the temp dir, so with Downloads on another
  filesystem or volume the move is a copy, written under a hidden `.part` name and renamed once whole; the agent
  hears the download completed once it is in place. A move that fails is reported to the agent as a failed download
  and the file goes with the profile; the window shows no toast for it. Chrome reloads the page 1.46 to 1.70 seconds after an `<a download>` begins, and the reload aborts a
  navigation started before it (`net::ERR_ABORTED`; Chromium 155, with downloads on or off), so an agent's click,
  key press, open or navigate that downloaded holds its answer 2.5 seconds; a navigation the person starts in the
  pane is not held. A download still running after ten seconds is reported in progress and named again when it
  ends. No tool lists past downloads or cancels one, nothing caps their size, and closing the browser cancels one
  still running.
- **A dialog a session's page opens holds the page until someone answers it** (`internal/browser/dialogs.go`,
  `chrome_dialog.go`, `docs/browser-tab.md`): nothing answers it on its own, so an agent that never calls
  `browser_dialog` and a person who never looks at the pane leave the page frozen, every other agent call failing
  with the dialog's name, until the browser is closed. The agent's click that opened it answers with the page as it
  stood, not as the dialog's answer left it. A dialog the page opens on a timer interrupts whichever call is
  running. Accepting a `beforeunload` less than a second after it opened is held to that second: accepted sooner,
  Chrome aborts the navigation it held and reloads the page, which asks again (3 of 3 at 300 ms, 3 of 6 at 400 ms,
  none of 30 from 500 ms; Chrome for Testing 155 and Helium 0.19.2.1). The agent hears of a dialog the person
  answered, at most twenty between two page answers, but not of one the agent itself answered. A held page cannot lay
  out, so a pane opened or resized meanwhile shows it at the size it had, and lays it out at the pane's once the
  dialog closes; a screenshot fails until then. On Chrome 153 and older, which answers a screencast's start and
  stop only once the dialog closes, a pane resized while one is open keeps its last frame until it is answered
  (Chrome 151 on the macos-15-intel runner, measured; 154 on answers at once).
  A window opened with Open in a window was not tried with a dialog.

## Languages

- **The prompt language reaches a running session only through lich itself** (`internal/prompt`): text lich
  composes in its own process (relay messages, nudges) follows the setting at once, but anything composed
  inside the session (the `lich` CLI, its MCP results, the plugin's hooks) reads `LICH_PROMPT_LANG`, which is
  fixed when the session spawns. Change the language and an open session can get relay messages in the new
  language and CLI output in the old one until it is restarted. The spawn briefing is likewise fixed at spawn,
  and only Claude Code and oh-my-pi receive one at all: they are the two whose command line can append to the
  system prompt (`briefingFlags`), so the other six read lich's words only in relayed messages. MCP tool
  descriptions, `--help` and error returns stay English in every language. A usage-limit continuation parked
  before the setting changed keeps the language it was parked in: the row stores the text, so
  `isResumePrompt` matches it in every locale.
- **Some interface text stays English in every language**: the exit marker written into a terminal's scrollback
  (`exitMarker`, `native/cmd/lich-native/terminal_exit.go`), because it is data in the replay and not a label;
  the `Session N` label a session gets when nothing names it (`internal/spawn/spawn.go`), because
  it is persisted and would otherwise change language on the next launch only for the rows made afterwards;
  every error and the output of `backend.ErrorText`, which show backend and library text as it came; the
  CHANGELOG group labels in the What's new dialog (`native/cmd/lich-native/settings_updates_notes.go`), which
  render a file that is English;
  and MCP tool descriptions and the `lich` CLI's `--help`, which agents read and which do not follow the prompt
  language either.

## Developing lich

- **The gate run inside lich kills lich's window** (measured 2026-10-10): `go vet`, `go test ./...` and the
  cross-compile loop started together from a session ran the machine out of memory, and the lich window died
  with SIGBUS; the window took the session that ran the gate down with it. `/tmp` is tmpfs on many distributions, so the Go
  build's temporaries count as RAM too. Nothing in lich bounds what a session spends. The way around it is
  in CLAUDE.md › Local Gate: one step at a time, in a `systemd-run --user --scope` with `MemoryMax`, with
  `GOTMPDIR`/`TMPDIR` on a short path on disk (`/var/tmp/lich-gate`; under `~/.cache` the sandbox suite's
  Unix sockets outgrew the 108-byte limit); there a step over the limit is the one killed. macOS and Windows have no
  equivalent here.
