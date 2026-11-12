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
  prompt, an issue or pull request handed over, a handoff note) goes into the question, with the Enter behind it answering it
  (what each provider then does: `docs/providers/`, Spawn). Claude Code, Cursor CLI, oh-my-pi and Kiro CLI stay
  unready until their session-start report, which their hooks send only from the agent's prompt. That hold needs
  lich-plugin's hooks in the provider: without them nothing would ever release it, so nothing is held and the
  typed text lands in the question. Claude Code and Cursor CLI are not held in a folder lich reads as trusted
  already (`trustAnswered`): no question is coming, and the quiet screen is the answer. The hold trusts the
  plugin on disk at spawn to be the one that process runs, so a session whose provider came up without the hooks
  (the plugin reinstalled while it started) stays unready until the delivery runs out: on Claude Code and Cursor
  CLI only in a folder not trusted yet, on oh-my-pi and Kiro CLI always. Codex, Antigravity, Crush and opencode
  report nothing before their first turn, so a quiet screen is still all lich has for them.
- **A handoff leaves two sessions in one checkout** (`internal/spawn/handoff.go`, `internal/cli/handoff.go`):
  `handoff_session` and `lich handoff` never close the caller, so until the user does, both agents can edit the
  same files; the caller is only told to stop. The note is the caller's own account of its work, checked against
  nothing, and waits 45 seconds for the successor's prompt (inside the 120 seconds an MCP client gives the call),
  where a relayed task waits five minutes; past that the session is open without it. The successor inherits the
  model, effort, ultracode and profile the caller was opened with, not one switched inside it (`/model`). It
  reaches every provider through spawn and `relay.Submit`. The successor always starts on the caller's profile,
  because lich moves work to another account only on the user's click; a `profile` on the call is only a suggestion
  put to the user as a toast once the successor's first turn has ended, so its button works when it appears: the
  first turn runs on the caller's account (`internal/terminal/limit_switch.go` `SuggestProfile`, the same offer as
  a usage limit's and the same `TakeProfileOffer`). It reaches the providers whose conversations can change profile where they have
  profiles: Claude Code, Codex, oh-my-pi, Cursor CLI (Linux and Windows) and Kiro CLI (Linux only). Antigravity and
  Crush have no profiles and opencode's conversations cannot move, so there the suggestion is dropped and the result
  says why, and the handoff goes on. Whether the profile is logged in is not checked. The toast waits for a `done`
  with a turn open, whichever turn that is: all five report one through lich's hooks, so a session of any of them
  without its lich plugin or extension installed, or a Codex turn a usage limit ended (Codex reports no end for
  those), never shows it, and the card's Continue in stays available. Unlike a limit's, the offer stands through
  later turns: a click on one that started since is Continue in's own refusal naming the turn, and the offer goes
  back to waiting and shows again when that turn ends, because the click took the toast down. A toast standing when
  the backend restarts is forgotten by the process and refused on click.
- **opencode 2 is told apart by its `--version`, and every 2.x session runs a server of its own**
  (`internal/providers/version.go` `IsOpencode2`, `internal/terminal/opencode2.go`): 2.x keeps 1.x's binary name,
  config and database but not its argv, so the spawn asks the binary for its major, once per path, size and
  modification time. A wrapper whose `--version` answers for another opencode than the one it runs (a
  version-manager shim that picks one per directory) spawns with the other generation's argv, which dies on an
  unknown flag. A 2.x session runs `--standalone`, because the background service 2.x attaches to by default
  holds the environment of whichever command started it, which would leave a session without its profile, its
  model or lich's MCP server. The price is that a lich session is not on that service, so the user's other
  opencode clients (`opencode pair`, the desktop app) do not see it, and each one boots a server of its own. A
  fork goes through 2.x's own route (`POST /api/session/{id}/fork`, run before the spawn) because its terminal
  interface has no `--fork`; a fork that route refuses is refused, not reopened as a resume.
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
  the cards the sidebar shows (`hotkeyCards`): a card in a folded block, under a folded mother, or left out by the
  search and state filter is skipped rather than unfolded, so a waiting session folded away is reached only through
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
- **A program's own status (OSC 7501) is a second source, and the hook outranks it** (`internal/terminal/progwire.go`):
  from the first hook state the window hears until a hook reports `idle` or the shell starts a prompt (OSC 133 A),
  the program's records are kept and the card is the hook's. Only Claude Code and Cursor CLI report `idle`, so
  in a session lich spawned as any other provider, and in a shell lich does not integrate where one of them ran, a
  program never drives the card again (Provider support table, README). Crush reports no state, so a program in a
  Crush session drives the card throughout. A command an agent runs through its shell tool usually writes to a pipe
  the agent reads, not to the PTY, so its report never reaches lich at all.
  - A `working`, `blocked` or `idle` record ends at a shell prompt (OSC 133 A, which lich's shell integration marks
    for bash, zsh and fish), at the next report, at a full reset (RIS) or when the session's process exits. There
    is no foreground process watch: in a shell lich does not integrate, a program that exits without reporting
    `done` or `error` leaves its state on the card until one of those. `done` and `error` end when you type or paste
    from the clipboard into the terminal (the window says so in a text frame on `/ws`); an emulator's automatic reply,
    a mouse report, a dropped or attached file and looking at the card do not end them.
  - The card shows one record: `blocked`, then `working`, then `error`, `done` and `idle`. `working` outranks the
    finished states on purpose, because they outlive a prompt and the card reads them as a finished turn; tuios
    ranks them the other way. The card has no error state, so `error` shows as a finished turn. `kind` and
    `progress` are read and dropped: the card has no place for either. The relay never sees these states, so a
    delivery does not wait on a program that says it is working, and they do not keep the machine awake.
  - A program's state reaches the card and the tray at once, at most 10 times a second per session, a burst folding
    into its newest state. Only the alerts it raises (the toast and the desktop notification, sound included) are
    limited further, in the window (`programAlerts`, `native/cmd/lich-native/program_alerts.go`), to one per session
    in 30 seconds: inside them the newest alert is held and goes out when they end, if the session is still in that
    state, so a second question arrives up to 30 seconds late rather than not at all. A hook's alerts are not limited.
    The tray's dot and menu read the current state, so they are not alerts and follow a program that flips.
  - Nothing else a report causes outside the terminal runs per report. The unread mark is written when it changes, at most
    four times in 30 seconds, and past that not at all: under a flood the mark may be out of date when a window
    reopens, which beats a late write undoing a card you read. A program's `done` does not look the pull request up
    again, re-read the plan's usage or quote the agent's last reply in its notification, and its `idle` does not
    clear the provider's mark, an in-flight relay mark or the footer's numbers, which a hook's `idle` does.
  - The backend answers `OSC 7501 ; ?`, because the window's libghostty-vt answers it only when the window sets a
    program status callback (`GHOSTTY_TERMINAL_OPT_PROGRAM_STATUS`). `native/gioterm` sets none; one that did
    would answer the query a second time. One goroutine per session writes the answers, so a child that stops
    reading its input costs one blocked write however often it asks.
  - **Unverified on Windows.** The PTY opens ConPTY without passthrough (`internal/terminal/pty_windows.go`), which
    may drop an OSC it does not know before lich reads it. Nobody has measured it there, so a report may simply never
    arrive.

## Transcripts, cost and plan usage

- **The cost readout bills per `(session, transcript)`** (`internal/pricing`, `internal/terminal/usage_cost.go`): a
  conversation forked inside the PTY bills its copied history twice — lich's own resume continues the same
  transcript and is unaffected — and each sub-agent's own transcript is counted in, so one unreadable or
  unpriceable sub-agent withholds the whole session's number. A Claude Code session whose mod reports a cost
  is not scanned at all: the report replaces the transcript, sub-agents included. A withheld number is marked `$—` on the footer
  only when the reason is standing (`costMiss.spoken` in `internal/terminal/usage_cost.go`): a transcript that
  merely could not be read this turn says nothing and keeps the last figure, so a reader cannot tell that
  absence from a session still on its first turn.
- **An opencode database can hold two generations, and each conversation is read from the newer one it is in**
  (`internal/terminal/sessiondb.go`, `opencode2Session`): opencode 2 copies a 1.x conversation into `session_v2`
  the first time it touches it and stops writing the 1.x tables, which it leaves in place (measured on 2.2.5
  over a 1.19.4 database). lich reads a conversation from `session_v2` when it is there and from the 1.x tables
  otherwise, so one continued under 1.x after 2.x copied it (a downgrade) shows its 2.x state.
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
- **A session a usage limit stopped is offered another profile only on Claude Code and Codex, and only one whose
  plan reads** (`internal/terminal/limit_switch.go`, `internal/quota` `HasQuotaLeft`): the wait for the reset runs
  as ever, and when another logged-in profile of the same provider reads with every window under 100%, a toast
  offers it. Nothing moves without the click, because a profile can be another account and which account holds a
  conversation is the person's to say. A profile whose plan cannot be read (signed out, an error, an account lich
  may not ask about) or reads with no windows counts as having none. The readings are cached for five minutes, so
  a profile a limit stopped a turn in is not offered again before its own reset, whatever its reading says; that
  memory, like a standing offer, lives in the backend's process and a restart forgets it, and with no window
  attached the offer goes nowhere and the session just waits. Only the first profile with quota is offered. The
  toast stays until answered and is not taken down when the session moves on: a turn starting, the card closing or
  moving, or the process ending withdraws the offer, and a click after that is refused and says so. The click's
  continuation takes the row the reset's prompt sits on, so it is typed once, up to 30 seconds later, once the
  respawned session is at its prompt, and one that waits past that for a slow start carries the late notice. A
  prompt a person parked there stays and goes out at its own time, so that click moves the session and sends
  nothing. The other six providers record no limit lich can read (above), so none of them is offered anything;
  opencode's conversations could not move between profiles anyway. Nothing crosses providers.
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

- **Only a Claude Code session with the mod is reminded of its errands after a compaction**
  (`internal/relay/observe.go`, `compactReminderLocked`; docs/hooks/session-state.md): a relayed task is a turn of
  the worker's conversation, and a compaction may summarise its ticket away. With two errands open an answer that
  names none is refused, so the worker cannot answer either. The mod's `session.compact` is the only compaction
  lich hears (`compacting`), so on the other seven providers, and on Claude Code without the mod, the ticket can
  still be lost. The reminder is typed when the compaction's closing report arrives: after a mid-turn
  auto-compaction that report is `busy`, and the reminder queues behind the running turn, so an errand that turn
  ends unanswered is reminded after it is already closed, where only a late answer by its ticket still lands.
- **Codex and Kiro CLI sessions get the spawn briefing, not lich's MCP instructions** (`internal/cli/mcp.go`,
  `mcpInstructions`; `internal/terminal/command.go`, `codexBriefingArgs`, `briefingEnv`): both initialize the
  server, list its tools and never hand its initialize `instructions` to the model (measured 2026-11-07 on
  0.162.1 and 2.21.0, `docs/providers/codex.md`, `docs/providers/kiro.md`). Each now gets the short briefing by a
  route of its own (a `developer_instructions` override on Codex, an `agentSpawn` hook on Kiro), so the longer text
  about waiting without polling and answering tickets still stops at each tool's own description there. On Codex,
  a user whose `config.toml` sets `developer_instructions` keeps their text and gets no briefing, since a `-c`
  would replace it; a project's `.codex/config.toml` setting it is replaced for the session, unread; and a
  conversation resumed or forked keeps whatever it was born with, none if it began before this or outside lich.
  On Kiro, the hook is written by the plugin install, so an agent installed earlier briefs nothing until the next
  one. Claude Code, Antigravity, opencode and Crush were measured passing the instructions on; Cursor CLI was not
  measured (no login on the measuring machine), and oh-my-pi is briefed on its command line whatever its MCP
  client does.
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
- **The ping-pong breaker knows a turn did no work only from the tools its hooks named** (`internal/relay/pingpong.go`):
  a message between the same two sessions counts toward the streak only when the sending turn reported tools through
  `PreToolUse` and every one was a relay tool (`send_to_session`, `reply_to_session`, `wait_for_answer`,
  `list_sessions`). The second such message in a row carries a warning and the fourth is refused. Crush reports no
  tools, and neither does a provider without the plugin installed, so a message from or to such a session never
  counts: it may be working unseen. A turn that reaches the relay through the command line
  (`lich send`, `"$LICH_BIN" reply`) ran a shell tool, which reads as work, so an exchange held that way is not caught
  either. A report that reaches lich after the send it precedes reads the same way: the breaker errs toward letting a
  message through. Only `send_to_session` and `lich send` count as hops; a reply closes an errand and is never
  refused, and `ask_session` never enters the other conversation. The streak ends when either session messages a
  third one, when a person sends a line at either prompt from the window, or when either session runs anything but a
  relay tool, so a worker's work between a coordinator's tasks ends it; a prompt a Claude Code mod submits is not a
  line typed there.
- **The chain limit follows a message only through the errand it was sent on** (`internal/relay/chain.go`): a
  message is one deeper than the deepest relayed task its sender still owes an answer to, and the fifth in a chain is
  refused. That is read off the relay's own tickets, so it is the same on all eight providers. A session that replies
  to its task before passing it on, or whose turn ended and closed the errand unanswered, owes nothing and starts
  the chain over, and so does a turn that worked, on the ping-pong breaker's rule and with its gaps (Crush is never
  counted). A turn the user started while an errand is still owed keeps that errand's depth.
- **A session's watch reads only what the pull request of its branch says** (`internal/relay/watch.go`,
  `internal/project/branchpr.go`). GitHub only, through `gh`, and the window cannot tell a remote gh does not
  speak to: such a session's dialog says "no pull request yet" for good. Checks count once none is pending, so
  a long matrix reports a failure only when it ends. `review` means a submitted review; a plain comment on the
  conversation does not fire it, and reviews already there when the watch is armed are the baseline. A read is
  once a minute per checkout, one more `gh` call for a checkout that holds a watch. The run limit
  (`api.WatchRunLimit`, 3) counts per pull request and an agent re-arming keeps its count; a person arming starts
  it over. It is checked when the condition holds again, so the third run succeeding says nothing. A session
  holds one trigger, a time or a condition, but lich's own continuation after a usage limit may sit beside a
  watch, and Remove takes off the watch first. `lich schedule` acts only on the session it runs in, and a time
  or condition the person set refuses it.
- **Only a GitHub Actions job carries a log in `lich check-log`** (`internal/project/checklog.go`): the job is
  read off the check's `detailsUrl` (`.../actions/runs/<run>/job/<job>`), so a commit status or another app's
  check is listed by name and link with nothing more. Logs are read only once no check is pending, since gh
  refuses a job's log while its run goes on. When GitHub's archive does not split a job by step (gh names every
  line `UNKNOWN STEP`, which is what this repository's runs get), the failing step is guessed from the runner's
  own markers: the last `##[group]Run` before the job's last `##[error]`. A job with two failing steps under
  `continue-on-error` then shows the last one only, and a step that prints its own `##[group]Run` line is cut
  there. Nothing parses a test framework's output: the tail is whatever the step printed last.

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
  session, except the ignored files the project names (`.env*`, or `.worktreeinclude`, read from the main
  checkout): `seedWorktree` copies those from the forked session's checkout, and one that session does not
  have stays absent. `node_modules` and the rest of the build output are not carried at all. Everything
  arrives unstaged: which changes the source had staged is a fact about the commit *it* is preparing. The row is
  offered only when the base is the forked session's own branch, because the patch is read against that
  checkout's HEAD; picking any other base branches off it at its last commit, the way every fork did before.
  And it rides on the fork entry itself, so the five providers whose CLI only resumes (README's provider
  table) cannot reach it either.
  The **+** dialog offers the same row for the project's own checkout, never preselected. No provider is
  out there, because the worktree exists before any session opens on it. What it adds is a snapshot that is not
  atomic: a session still writing in that checkout while the diff is read and the untracked files are copied
  can leave the new worktree with a mix of two moments. The source is only ever read, never cleaned, so
  nothing is lost there.
- **git status is polled**: every shown path is re-read each `gitPoll` (`native/cmd/lich-native/model.go`,
  `pollGit`); the lich plugin's `session-touched` hook nudges an immediate refresh.
- **The Git panel's history is paged by re-walking it** (`internal/project/gitlog.go`): every page is a `git log
  --date-order --skip=N`, and ordering walks the whole history first, so on a very large repository without a
  commit-graph each page costs seconds and one over `gitLogBudget` (30s) fails with a message rather than a partial
  list. `--skip` also means a commit landing while you scroll can repeat or hide a row until the next read, which
  the ref poll (`gitLogPoll`, only while the panel is open) triggers. The Author menu lists only authors of the
  commits already loaded, and the graph clips past `gitMaxLanes`.
- **A branch delete's Undo is a new branch at the old commit** (`internal/project/gitbranch.go`): the toast's Undo
  makes the branch again where it pointed, without its upstream setting or its reflog, and only for as long as the
  toast is up. The dialog asks first only when commits would be on no branch, remote branch or tag
  (`GitBranchOnly`); a branch whose commits are on a remote is deleted at once, even when that remote branch is the
  only copy left. A checkout in a checkout with a session confirms, then switches under the agent with whatever it
  has not committed: git refuses only edits the switch would overwrite.
- **A narrowed history has no graph** (`native/cmd/lich-native/git_store.go`, `narrowed`): text, author, date and path
  leave parents out of the page, so a line would join commits that are not adjacent. Narrowing by a branch keeps the
  graph, since that page holds the branch's whole ancestry. A merge's files and diff are against its first parent;
  the changes it brought from the other side as a conflict resolution do not show separately.
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
- **One opencode module serves both generations, from the directory both read** (`internal/agentplugin/opencode.go`):
  lich writes it to `~/.config/opencode/plugins/`, the only global plugin directory opencode 2 reads, and
  removes the copy an older lich left in `plugin/` when it carries lich's marker, since 1.x reads both and would
  report every event twice. A copy there without the marker is someone else's and stays, with the same double
  report. 1.x runs the module through its default export's `server()` (measured on 1.18.23 and 1.19.4) and 2.x
  through its `setup()`; an opencode older than 1.18.23 is not measured with that export.
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
  update check reads one thing from the published notes, whether a release is required (next bullet), and
  nothing else: the toast says a release exists, and the dialog is where it speaks.
- **A required update is a `> [!CAUTION]` block in a release's notes, and only a lich that knows the signal
  obeys it** (`internal/appupdate`, `patchnotes.RequiresUpdate`): the check reads the releases list, the same
  one request an hour that names the latest release, and reports `Required` when any release newer than the
  running build carries the block, so skipping three releases does not skip the one that was a security fix.
  A lich older than the release that introduced this reads nothing but the tag and shows the ordinary toast
  with Later, so the first required release cannot require itself of the liches before it. The list is one
  page of 100 releases: a build further behind sees only the newest hundred. Unlike the What's new dialog,
  this reads the notes GitHub publishes, not the binary: adding the block to a release after its tag makes it
  required for everyone still behind it, and removing it lifts the gate at the next check.
- **No GitHub, no gate**: a lookup that fails (offline, rate limited, a proxy that blocks api.github.com)
  reports no update and nothing required, and a restart forgets the last answer. The gate is pressure on a
  user who can update, not containment, and blocking the lookup lifts it.
- **A required update refuses new agent sessions and never touches one lich already holds**
  (`store.SessionRefused`, asked by every session insert and by `internal/spawn` before it creates a
  worktree): the window, `lich open`, the MCP tools, a handoff, a fork and a subagent card are refused on
  every provider, with an error naming the release and the way this install updates. A running session
  keeps running, respawns after the restart, resumes, and comes back from the closed list. A shell session
  still opens, on purpose: the prompt's Install action is a shell with the command typed into it, and that
  command restarts lich through the `LICH_PORT` and `LICH_TOKEN` only a lich session exports. An agent
  started by hand in that shell is therefore not stopped. The backend answers from the last lookup for two
  hours and then looks again itself, so a lich with no window open still learns of the release, and the one
  open that finds the answer stale waits on GitHub for up to five seconds. A worktree asked for in the
  window's New worktree dialog, or for a pull request, is created before its session is refused and stays;
  the next try reuses it.
- **A package-manager install still needs the user to run the command**: only the Windows installer updates
  from the prompt's button. Everywhere else the required prompt types the command into a shell, or links the
  release page for a tarball, and lich never runs it. The prompt has no Later, comes back when swiped away,
  and every hourly check raises it again once its action closed it. `lich.appUpdateDismissed` is not read for
  a required release.

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
- **opencode 2's `auth` runs on a private server, and only a stored login counts** (`internal/xdgprofile`,
  `opencode2Status`): 2.x sends `auth list` and `auth login` to the background service every opencode shares,
  which holds the data home of whichever profile started it, so lich passes `--standalone` to both. Its list
  names no account and no count: a row ending in `stored` is the profile's login, and a key found in the
  environment (`environment`) is not. The OpenCode Go gauge reads 2.x's key from the `credential` table of
  `opencode.db`, before `auth.json`, which 2.x no longer writes (`internal/quota`, `opencodeGoKey`).
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
- **The binary setting still answers for Personal**: a profile's session runs the configured Claude Code
  binary, so a wrapper that exports its own `CLAUDE_CONFIG_DIR` or a token still wins over the profile, and the
  same binary answers the profile's `auth status` and installs its plugins.
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
  one of them — it lives in the system keyring, which is why reaching it takes the flag below. lich's own
  directory under it (`~/.config/lich`: the launch token in `runtime.json`, the database, every account's
  profile) and the worktrees of every other session (`~/.local/share/lich/worktrees`) are taken back out:
  an empty directory sits over each, and only what the session owns is mounted back on top (its profile and
  plugin store, the hook scripts under `plugin/`, its dropped files, its own checkout). The two paths are
  spelled in `internal/sandbox` as well as where lich creates them, and a lich directory moved elsewhere by
  `XDG_CONFIG_HOME` or `XDG_DATA_HOME` is followed, a symlinked one to its target. A confined session is also
  handed a token of its own instead of the launch-wide `LICH_TOKEN` (`internal/terminal`, `scoped.go`): it
  reaches the hook endpoints for its own session only, and the RPC through `internal/scope`'s allowlist, deny
  by default. What is out is every call that acts on the machine outside the sandbox or belongs to the window:
  `services.*` except the reads (a service's command runs unconfined), `store.SetSetting` (the rung),
  `terminal.*` except `Ready`, `project.*`, `drop.Attach`, `system.*` except `Diagnostics`, the window's
  browser methods, `/events`, `/ws`, `/blob`, `/drop`, `/restart`, `/debug/goroutines`. On the methods that
  name their caller (`from`, a browser's `id`) the name must be the token's session, so a confined agent
  cannot speak to the relay as another session. A session it opens (`open_session`, `lich open`, a handoff, a
  subagent card) is confined whatever the rung says, since `SandboxDefault` answers no for the main checkout
  under "Worktrees only". The token is minted per spawn and lives in the backend's memory: a respawn, a parked
  session resumed and a backend restart each issue a new one, and a caller that outlives its process (a
  background agent) is refused from then on and cannot fall back on `runtime.json`, which it cannot read; the
  `lich` CLI says so. The relay stays, held to one more rule where each method resolves its target
  (`scope.Target`, called from `internal/relay` and `internal/spawn`, never from the transport, which cannot
  turn a label into a row): a confined session speaks to confined sessions and answers anyone. `relay.Send`,
  `SendPrivate`, `SendSubagent`, `Insert`, `Submit`, `spawn.Control` and `spawn.Ask` put text at the target's
  prompt, and the receiving agent carries it out as its task, so a target whose row is not confined is refused
  by label with the reason; `relay.Reply` reaches any ticket, since a reply is read as a result, and it is how a
  confined worker reports to the unconfined session that opened it. `spawn.Close` refuses `force` from the
  sandbox: the checkout it would remove is outside it and the uncommitted work there exists nowhere else. The
  cost is the deliberate one: a confined session cannot delegate to an unconfined one, only to sessions it
  opens (which are confined) or that were already confined. The eight providers
  are untouched by the scoping: every hook and `lich mcp` reads `LICH_TOKEN` from the environment
  (`docs/hooks/`), none reads `runtime.json`. The window's own flows run on the launch token and lose
  nothing. A method that reads a host path it was *handed* would copy anything into reach of the sandbox,
  which is why the attach flow opens the picker inside the backend (`drop.Attach`) instead of taking a path, and why it is not on the allowlist. The private home is writable and vanishes with the session, so a dotfile an agent writes is gone
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
- **The display socket hands over whatever the compositor advertises, not only the clipboard**
  (`internal/sandbox`, `displaySockets`). A Wayland socket is one connection to the compositor, and the
  compositor decides what a client on it may do; the sandbox cannot narrow it. On the wlroots family (sway,
  Hyprland, river, labwc and the rest) `wayland-info` run inside a confined session lists
  `zwlr_screencopy_manager_v1` and `ext_image_copy_capture_manager_v1` (capture the screen, any window),
  `zwp_virtual_keyboard_manager_v1` and `zwlr_virtual_pointer_manager_v1` (type and click into windows
  outside the sandbox, a terminal with the user's shell included) and `zwlr_data_control_manager_v1` (read
  and set the clipboard without focus), all measured on one such compositor. KDE's KWin exposes its own
  capture and input protocols to every client too; GNOME's Mutter exposes neither and routes both through
  the portal, which the sandbox's empty runtime directory does not hand over. Which family a host runs is
  therefore the whole answer to "can a confined agent watch my screen and type at my shell", and only a
  compositor that filters clients by security context (`wp_security_context_v1`, which bubblewrap does not
  set up) closes it. The mount stays: without it the agent's copy and image paste fail, and the cost is
  written here rather than hidden behind a sandbox that looks tighter than it is.
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
  free to outlive the call, and the dump is `stty -opost; echo <start>; env -0; echo <end>`: the read returns the moment the end
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
  The `-0` is what keeps a value spanning newlines whole, and it holds the dump to an `env` that has it (GNU
  coreutils does): an `env` without it prints its usage, no entry parses, and the session gets
  the launch env with a logged warning, as on any other failed dump.
  The `stty -opost` in front is what keeps the bytes as `env` printed them: under ONLCR a macOS pty sends a `"\n"`
  that meets a full output queue as `"\r\r\n"`, which no undo tells from a value's own `"\r\n"`. A shell that
  skips the stty (a fake one in a test, a broken PATH) is caught by its sentinel line still ending in `"\r\n"`, and
  its dump falls back to undoing every `"\r\n"`, the one path where a value's own `"\r"` is lost.
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
  `native/cmd/lich-native/terminals.go`): the window builds a session's terminal from the backend's ring when it is
  first shown, and again after it reconnects. Scrollback past the ring is gone, not paged. The modes set in bytes
  that have left it survive: `internal/terminal/modestate.go` reads each evicted byte and the replay opens with the
  state at the ring head (DEC private modes with their XTSAVE values and DECSTR's reset, kitty keyboard stacks,
  modifyOtherKeys, each screen's cursor style with its blink, OSC 4/10/11/12 colors and the same colors through OSC
  21), then the sequence the head cut in two. What it does not track is lost the old way: the screen's contents
  under an alternate screen still active (the primary screen starts blank), the cursor position, the title, insert
  and newline mode, OSC 5 and 13 to 19 colors, scroll margins (libghostty resets them on a resize, which the bytes
  do not show) and charsets (it restores them with the saved cursor), and a sequence the head cuts that cannot be
  replayed from its middle, which is skipped: an OSC longer than 8 KB (an OSC 52 copy) or a DCS or APC string (a
  kitty image chunk). A service's output (`lich service`, `read_service_output`, the console of a stopped one) is
  the tail alone, with no preamble: it is read as text.
- **Images are Kitty graphics sent in the stream, and only placed ones** (`native/gioterm/kitty.go`): a terminal
  takes images sent inline (`t=d`), PNG or raw pixels, up to `imageStorageLimit` (64 MB decoded per terminal;
  Kitty's own default is 320 MB). The file, temporary file and shared memory media stay off: honoring them would
  let any program on the PTY have the window read a path of its choosing. Sixel and the iTerm2 protocol are not
  implemented, and neither are Unicode placeholders (`U=1`, the form tmux passthrough needs), so a program that
  only speaks those draws its text fallback. The row renderer composes a row's text and backgrounds in one image,
  so a placement between the backgrounds and the text (`z` from `INT32_MIN/2` to `-1`) draws under a colored
  cell's background as well, the same as one below the backgrounds. Images live in the window's terminal, not in
  the replay ring: a terminal rebuilt from the ring gets back the images whose bytes are still in it, which a
  program that repaints (a TUI) recovers from and an image printed once at a shell prompt does not.
- **A PNG is decoded with the terminal locked** (`native/gioterm/kitty.go`): libghostty calls the decoder from
  inside `ghostty_terminal_vt_write`, so `goDecodePng` runs under `Terminal.mu`, and the frame waits for that lock
  in `Update`. One 1080p PNG holds it about 20 ms, a dropped frame for the whole window. Releasing the lock inside
  the callback would let the frame into libghostty mid-write. The two ways out, if a slow window is ever reported:
  decode ahead in `Write` (parse the Kitty APC chunks, `o=z` included, and hand the callback a cached result), or
  let the frame skip a locked terminal and redraw its last snapshot, which every frame-side call that takes
  `Terminal.mu` would need too.
- **One socket carries every session's output** (`internal/terminal/writequeue.go`): the per-session outbox
  decouples the *producers*, never the wire. A window that stops reading stalls the connection's single writer,
  so after `wsWriteTimeout` (5s) every session's output switches to the `/events` bridge at once. That is a
  second socket, and the window reads the two independently: a frame that fell back can land ahead of one still
  sitting in the stalled connection's buffer, so output is never dropped but its order across that switch is not
  guaranteed. Until then the queue holds `writeQueueDepth` frames for the whole app, and two things still wait on
  it across sessions: a push that finds it full, and the flush a session runs before its exit banner so the banner
  cannot overtake its own last bytes.
- **An emoji sequence is only as whole as the emoji font's ligatures** (`native/glyphs/cluster.go`, `cluster`): the
  terminal gives a ZWJ sequence or a flag one two-column cell whatever the font holds, and draws the one glyph
  shaping turns it into. A sequence the font has no ligature for draws its first glyph only, the rest of it still
  in the cell and in what a copy returns. Windows always hits this for flags: Segoe UI Emoji has no flag glyphs, so
  a regional-indicator pair draws its first letter alone. The renderers differ where it shows: a stack of combining
  marks taller than a row is cut at its row on `rows`, the default, and paints over the rows beside it on `atlas`
  and `paths`; `paths` leaves the font to Gio's shaper, so an emoji asked for with VS16 draws monochrome and a keycap
  draws as its plain digit.

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
  stops nothing. Quitting lich stops everything the same way, and the next launch starts only the services saved
  to start with lich in the projects it opens with. After an in-place restart nothing comes up until the outgoing
  lich has finished stopping its services, up to its 10 second grace, or a command holding a port would lose it to
  its own second copy. Home's services, edited from Settings › Services in Global, run for as long as lich does,
  since Home has no tab to close.
- **What a stopped service printed is kept in memory only** (`services.Output`): the tail of its last run, as
  long as a session's replay, until it starts again or lich exits. After a restart of lich a service that has
  not run yet has no output to show.
- **The tray names at most eight running services, and eight sessions per group** (`internal/tray`,
  `serviceSlots`, `sessionSlots`): the menu is built once and its entries shown and hidden, so past eight the
  services' count says how many run, a "more" line counts the sessions past the slots, and the window lists
  them all. The menu and the icon's badge are read every five seconds, not on each report, so a badge can be
  that late. A session sorts into Needs you or Working by the state its provider's hooks report, the one its
  card draws: a provider that reports none for a turn (Crush reports no `busy`) counts as idle there too. A
  finished turn leaves Needs you only once its card is read in the window. The badge's dot is drawn in the
  window's dark-theme tones whatever the bar's colour. The tray's host draws the menu at its own width and cuts a long
  line there, and a menu entry has no tooltip on Linux (dbusmenu carries none) or Windows: the full lines go
  in the icon's tooltip instead, which a host may show cut, on one line, or not at all. Only macOS gives each
  entry a tooltip of its own. The macOS menu-bar entries were never run on a Mac: they compile, and the logic they share with
  Linux and Windows is tested.
- **On Windows, Stop is Ctrl+C through ConPTY** (`InterruptService`): it reaches the console's processes as
  `CTRL_C_EVENT`, which a process that left the console or turned the event off never sees. Not measured: the
  service tests spawn real PTYs and run on Linux and macOS only.

## Extensions

- **An extension runs unsandboxed, as the user, and placing its directory is the only consent** (`internal/extensions`):
  every directory under `<config dir>/lich/extensions` (`extensions-dev` under `LICH_DEV`) with a valid manifest is
  started when the backend boots. There is no prompt and no signature, the manifest's permissions bound only what
  it asks lich for (below), and the sandbox of `internal/sandbox` does not apply, so an extension can read and do
  whatever the user can. The
  directory is read once per launch: an extension added or edited by hand takes a restart of lich, while one
  installed or removed through `extensions.CommitInstall` and `extensions.Remove` is started or stopped at once.
- **Installing runs code from the internet as the user, vouched for only by its own release**
  (`internal/extensions/install.go`): the archive is checked against the `checksums.txt` the same release carries,
  which proves the download arrived whole, not who built it or what it does. Whoever controls the repository
  controls what runs. Only `https://github.com/<owner>/<repo>` is accepted, only its latest release that is neither
  a draft nor a pre-release, only the archive named for this OS and architecture (`docs/extensions.md`), and only
  public repositories: the GitHub API is read without a token, so its unauthenticated rate limit applies. There is
  no update: remove the extension and install it again. A prepared install waits in an `.installing-*` directory
  that discovery skips until its confirmation is answered, for at most 15 minutes (`internal/extensions/staged.go`,
  `stagedTTL`): a person who takes longer gets an error naming the expired install and starts again. A download
  lich is killed in the middle of stays on disk until lich next starts, which deletes every `.installing-*`
  directory. Remove deletes the directory, or only the
  link when it is a symlink, and what the extension wrote elsewhere stays.
- **An extension's data directory outlives it, and nothing bounds its size** (`internal/extensions`):
  `<config dir>/lich/extension-data/<id>` is created before each start and handed over in `LICH_EXTENSION_DATA`.
  Remove keeps it, so a reinstall of the same id finds it, and so does any other extension later installed under
  that id. It goes only through `extensions.DeleteData`, which the remove confirmation calls when its box is
  ticked, and which refuses an installed id; data left behind by an extension removed without the box stays on
  disk until it is deleted by hand. lich does not limit how much an extension writes there.
- **Installing from a folder copies it once, and refuses links** (`internal/extensions/installdir.go`): the picked
  folder is copied under the same bounds as an archive (1 GiB, 1000 entries) and validated as discovery does. A
  symlink anywhere in it, the folder itself included, fails the install rather than being followed. The copy is
  what runs: editing the folder afterwards changes nothing until the extension is removed and installed again,
  so to develop in place symlink the folder into the extensions directory instead. Its source is the folder's
  path, with no release, and nothing checks who wrote what is in it.
- **What an extension writes is input, and what fails the checks is dropped without telling it**
  (`internal/extensions/inbound.go`): a line over 3 MiB, JSON nested over 512 levels, a tree over 5000 nodes or 64
  deep, a kind that only travels from lich to the extension, and `openExternal` with anything but an http or https
  URL are refused and written to lich's log, and the process goes on. Its log is budgeted at 100 lines a minute
  each for stderr and for refusals, so past that a refusal leaves no trace beyond the count of lines dropped. The
  surface a `render` names is not checked against the manifest.
- **lich translates only an extension's name and surface titles** (`internal/extensions/locales.go`,
  `extension/i18n`): it reads them from the extension's own catalogs (`locales/<tag>/extension.json`), falling
  back to English and then to the manifest. Toasts and every string in a tree are the extension's to translate:
  lich hands it the language (`LICH_LANGUAGE` at start, `language` on each change and start, and in
  `surface.opened`) and the `extension/i18n` package, and never looks a tree's text up itself. `LICH_LANGUAGE`
  is the backend's best guess: with no language chosen it reads the environment's locale, which macOS and
  Windows leave out, so there an extension starts in English until a window attaches and sends `language`. A
  plural `manifest.name` or title is ignored.
- **An extension with `sessions:write` drives the person's sessions, and the install is the only consent**
  (`internal/extensions/calls.go`): it can open a session in any open project or directory path, on any provider,
  in a new or existing worktree, and type any prompt into any live session, as `open_session` and
  `send_to_session` do for an agent. The install confirmation lists every permission the staged manifest declares,
  this one in the wait tone, and lich asks nothing past it: there is no per-call prompt and no way to grant part of
  the list. The lines are lich's words for a permission name, so they say what the call can reach, not what the
  extension says it does with it. The permissions bound only the calls: the process itself is
  unsandboxed. Every provider is reached exactly as those MCP tools reach it, with their gaps (the Provider support
  table: a session whose provider reports no state lists an empty state and sends no `session.status`).
- **What an extension opens or sends is recorded as coming from no session** (`internal/extensions/methods.go`):
  lich records who asked only as the origin session of a new session, and an extension is none, so a session it
  opens has no origin on its card, and the prompt it sends arrives introduced as relayed by the lich command line,
  as `lich send` outside a session does. Nothing records which extension did it.
- **An extension collects a pending send only by its ticket, and only its own** (`internal/extensions/methods.go`):
  a send from no session goes to no inbox and no note announces its answer, so `sessions.answer` with the ticket
  is the one way to it, each wait at most 90 seconds. The relay redeems a ticket for any caller, so the extension
  layer keeps the tickets each extension's `sessions.send` and `sessions.open` returned `pending` and refuses any
  other; an extension cannot read another's answers, nor a session's, by guessing a ticket. A ticket is spent
  once its outcome is not `pending`. The tickets are kept in memory: a lich restart forgets them, and the relay
  with them.
- **An extension with `sessions:ask` reads what a Claude Code session knows** (`internal/extensions/methods.go`,
  `spawn.Ask`): a side question is answered from that session's whole conversation, so the extension can learn
  anything said in it, and the answer arrives with nobody at the session told. It carries `ask_session`'s gaps
  unchanged: only a Claude Code session with the lich-plugin mod loaded answers (every other provider is
  refused before anything is queued, the Provider support table's `lich ask` row), the wait is 90 seconds with
  no ticket and a later answer dropped, the fork runs on and is billed past it, and the answer is cut at 16,000
  characters (the mod ceiling above).
- **An extension with `services:write` runs the commands a project saved, and `services:read` reads what they
  print** (`internal/extensions/methods_services.go`): it starts, stops and restarts any saved service of any
  open project, as the service tools do for an agent, but cannot create, edit or delete one. A console read may
  carry tokens a dev server prints. `service.status` fires per service whose state changed between two
  services events; a service lich has not listed before fires only when it is not stopped, and one deleted
  fires nothing.
- **`git.status` reads git on each call, and counts against the upstream without fetching**
  (`internal/extensions/methods_window.go`): the window keeps the git state it shows in its own process, so the
  backend has no store to answer from and runs the reads a card runs (`project.Diff`, `project.BaseStatus`),
  plus `project.UpstreamCounts`. Ahead and behind come from the refs on disk, as `git status` counts them, so
  they are as fresh as the last fetch. `git.pullRequest` reads the branch watch's store and makes one `gh` call
  only for a checkout the watch never read, which it does not then start following; a repository with no
  remote answers no pull request without one.
- **An extension's notifications obey no notification setting, only a bound** (`internal/extensions/methods_window.go`):
  `notify` goes through `system.Notify`, the path lich's backend raises its own on (a schedule forfeited). lich's
  two notification settings are about sessions (one needing input, one finishing a turn), so neither applies,
  and the window's focus does not either: the notification goes up while the person is in lich. Each extension
  raises at most 3 a minute (`maxNotifications`), the bound kept across its restarts; the rest are refused with
  an error. There is no click action, and a call that returns fine says only that the notification was handed
  off: on a Linux desktop with no notification server it still returns without error and nothing shows (measured
  on a rig with no session bus).
- **An extension with `window` acts on screen in a session's name** (`internal/extensions/methods_window.go`):
  `window.showCode` opens any file of a live session's checkout in the dock and `window.openURL` loads any http
  or https page in a session's browser, both as the session's own tools do, and the person sees them as that
  session's. Both need a window attached to `/events`, and refuse otherwise. `window.openURL` takes neither a
  local file nor the artifact folder, which `browser_open` serves, and drives no page past the load.
- **Calls are bounded per extension, not across them** (`internal/extensions/calls.go`): 8 running at once, the
  ninth refused, each cut at 2 minutes, which is also where a `sessions.open` slower than that ends while the
  session it was opening may still appear. A result or event the extension does not read in time, past the 64
  lines its stdin queue holds, is dropped and logged.
- **A message sent while no window is attached is dropped** (`internal/events`): an extension's lines ride
  `/events` like every event, with no queue and no replay, and the extension is not told. A window that attaches
  later sees nothing until the extension renders again. One slow window stalls that extension's stdout rather than
  queueing, and a write stalled past 5 seconds drops the `/events` socket for every session.
- **`render` is coalesced, at 30 a second per surface** (`internal/extensions/throttle.go`): the first goes out at
  once, and while the interval has not passed only the newest is kept and sent when it ends, so a loop of renders
  costs the window a frame rate and the newest tree always arrives. The older ones in between never reach the
  window. A surface the manifest does not list shares one gate with every other such surface, so naming a new
  surface each time is no way around the limit, and of those only the newest render arrives. Other kinds (toasts,
  `openExternal`, clipboard) are not limited: a loop of those still reaches the window line by line, and a process
  that exits drops the render it had pending.
- **Restart is the way back from failed** (`Service.Restart`, `extensions.Restart`): it stops a running extension
  as lich stops it on exit, starts it again and starts its crash count over. It does not re-read the directory, so
  an edited manifest or a newly invalid one takes a restart of lich, and an invalid extension is refused.
- **Any exit lich did not ask for counts as a crash** (`internal/extensions/supervise.go`), a clean exit 0 included.
  It restarts after 1 second, doubling, capped at 30, and lich gives up after 5 crashes in a row, each within 30
  seconds of its start: the extension shows failed until lich launches again. A file that cannot be executed at
  all fails at once, with no retries.
- **Stopping reaches the extension's own process only** (`internal/extensions/supervise.go`): stdin closes, 3
  seconds later the process is killed, and whatever it started is left running. If lich itself is killed, stdin
  ending is the only signal an extension gets. During an in-place restart the outgoing lich stops its extensions
  while its successor starts them, so two copies can overlap for up to that grace.

## Window, sidebar and settings

- **Pinned project tabs are a window preference, not a close lock** (`native/cmd/lich-native/projects.go`,
  `pinnedProjectsKey`): the global setting keeps project ids even while their tabs are closed, so reopening one
  brings it back to the pinned group. The tab's × is hidden, but Close project in its right-click menu and
  `close_project` still close it.
- **A frame is drawn in part only with `GIO_DAMAGE=on`, and only on OpenGL 3 or OpenGL ES 3 and up**
  (`lichdotdev/gio`, `gpu/damage.go`): with it, the window diffs a frame's draw operations against the last
  frame's, redraws only the rectangles that changed into an sRGB framebuffer the renderer keeps, and blits that
  whole to the back buffer before a plain full swap, which is what keeps a spinning ring from repainting the
  window 24 times a second. It no longer reads the buffer age, so a driver misreporting it cannot show stale
  pixels. It costs width x height x 4 bytes of texture and is opt-in because frames with stencilled paths can
  differ from the same frame drawn whole by one level per channel, as whole frames already differ from each other
  with their paths at other atlas positions. Off, on other backends, on js, and on a frame with a translucent
  clear, every frame is drawn whole even when opted in. The diff sees operations, not pixels: a path is told apart by its
  content hash and transform, an image by its handle, so an image mutated in place behind the same `ImageOp` is
  not seen as a change (`gioterm` makes a new one per changed row).
- **A closed card outlives its session by 180ms** (`native/cmd/lich-native/sidebar_closing.go`, `exitDuration`):
  the card animates out, so the closed session is held and drawn for the length of that exit. For those frames
  the sidebar draws a card for a session the state no longer holds, and anything counting or measuring cards
  sees one card too many.
- **A child sits with the session that opened it, not with its checkout** (`native/cmd/lich-native/sidebar_tree.go`):
  the sidebar's tree is read off `OriginSessionID`, so a session opened from another hangs from it in the
  mother's block even when it runs in another worktree, under a row for that worktree inside the mother's
  group; that worktree's own block does not list it. Each generation hangs a level deeper, so a deep tree
  cuts its titles short at the sidebar's default width. A child whose mother closed, or lives in another
  project, is a root again. A child pinned or filed in a folder on its own leaves the tree for that block.
  How rich the tree is depends on the provider: only Claude Code, Codex and opencode fork, a subagent card
  exists only on Claude Code with lich-plugin, and `open_session` reaches the providers with lich's MCP. A mother's fold is stored per session (`lich.sidebar.collapsed.__session__:<id>`)
  and stays stored after the session closes.
- **A checkout row reads the facts of its path, not of the sessions under it** (`checkoutFactsOf`,
  `native/cmd/lich-native/sidebar_blocks.go`): the row shows the branch, pull request and changes of the
  checkout's own path, which the git reader polls only while some session shows it. A checkout every
  session under it left with `cd` shows nothing until one comes back. A checkout with no open session has no
  row: the window learns of checkouts only through sessions. Inside a folder or the pinned block, the checkouts
  of the sessions gathered there are rows too, but they only label: they do not fold, and they have no menu. The branch name is cut short at the sidebar's
  default width when the row also shows a pull request and changes.
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
  size; clicking its card anywhere shows it in that layout, and its project's tab never shows it alone
  (`aloneLocked`, `sidebar_split.go`): the tab shows the project's first session in no layout, or a notice
  when every session is in one, which is also what a session closed there or a window reopened lands on, so
  the tab does not come back on the session it last showed. A layout's tab has a sidebar of its own, its
  sessions under their projects' names in the project sidebar's tree. That list is not the project's: it has
  no New Session, filter or services, a card there offers no folder and is not dragged, a mother's children
  cannot be folded, a checkout row has no menu, a folder is not drawn, and no card is lit, the panes showing
  which session has the focus. The collapsed sidebar still lists the focused pane's project. A split
  is refused, with a toast, when a pane would drop under 480dp (51 columns at the default 13sp, measured with
  Claude Code and Codex): a larger terminal font fits fewer columns in the same floor. Only the focused pane
  takes a dropped file; a file dropped on a pane in the background goes nowhere. A pinned card cannot be dragged onto a pane: the
  pinned block shares no carry with the panes, so its card never leaves the sidebar. The panes' drop zones are
  not SortCarry targets (`layouts_drag.go`): a new drop target over the stage belongs there, not in the carry.
  A pane entering or leaving moves only its frame and its neighbours' (`stage_motion.go`): each terminal is
  drawn at its final size from the first frame, so for the 200ms a neighbour that gains room shows its wider
  terminal cut by a frame still narrow. A pane that left is drawn as an empty frame shrinking, not with its
  terminal, and not at all when more than one pane takes its room. Swapping two panes, rotating a split, a
  zoom and a divider do not animate.
  Show beside puts the delegates in a column on the right half of their opener's pane, or stacked under it
  when that pane is too narrow to halve (`delegateRoom`), and lays out only the delegates that fit the 180dp pane
  floor, counted from where the panes were drawn the frame before (`columnRoom`); the rest stay out and
  the pane header offers them. Its footer item is not an item of the Footer layout, so Settings can neither
  move nor hide it: it shows only while the session, in no layout, has delegates.
- **Single instance via the pinned port**: the bind is the lock (`internal/singleton`); a duplicate launch asks
  the running lich to show its window (`singleton.Show`, `system.ShowWindow`) and exits. Raising is
  best-effort: a Wayland compositor may only mark the window urgent.
- **On macOS a quitting lich can hold the port for a few milliseconds after it lets go**
  (`internal/terminal/listener_linux.go`): a child process lich is starting copies the listening socket and
  keeps it until it execs. Linux takes the socket out of listening at once; macOS refuses that on a listener,
  so there the port frees when the child execs, and a relaunch rides it out with its bind retry
  (`bindTimeout`). Windows children do not inherit the socket.
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
  A Chinese, Japanese or Korean font downloaded while a terminal is open leaves the characters that terminal already
  drew as empty boxes empty until lich restarts or its font or font size changes.
- **Following the OS's light or dark is polled on macOS**: a theme's `pair` and `lich theme switch` reach the
  window through the backend's `theme-selection` event. Following the OS
  reads the freedesktop appearance portal on Linux and `AppsUseLightTheme` in the registry on Windows, both
  watched. macOS polls `defaults read -g AppleInterfaceStyle` every 3 seconds, so a switch reaches the window
  up to that late: observing `AppleInterfaceThemeChangedNotification` needs an Objective-C shim no one
  could compile or run without a Mac. The BSDs read no scheme and draw dark.

- **A session's browser is a second Chromium, one per card, drawn from frames** (`internal/browser`,
  `docs/browser-tab.md`): a headless Chrome, Edge, Chromium, Brave, Vivaldi or Helium already installed,
  whose page reaches the session's page pane as a JPEG screencast over `/events`, at the page's own frame
  rate; a window slower to decode skips to the newest frame. A HiDPI pane gets frames drawn at 1.5x at most
  (`frameScale`): 2x halved a busy page's frame rate. The browser draws at a scale forced at its launch, the one
  the window drew pages at then (its own scale times the page zoom, 1 at least; 1.5 when no window had said).
  A page zoom changed afterwards keeps the frames of the launch, enlarged by the window past that scale and under
  it reduced by Chrome, which blurs text (between 100% and 150% on a 1x screen), until the browser is closed and
  opened again or relaunched: a relaunch on a zoom would cost the page its scroll and a half-filled form. A move
  of the window to a monitor of another density does relaunch it once the window's own scale holds still for a
  second (`rescaleWait`), on its profile at that scale times the zoom then, with its tabs, the front one, their
  DevTools and its cookies, never while a dialog is open, a page is paused in the debugger, an agent's call runs
  or it is in its own window, and one no pane shows when a pane next does. A half-filled form, sessionStorage,
  where each tab was scrolled and a download under way are lost to it. A browser drawing at 1.5x
  under a 1x pane also cost a 1140x1300 pane 11 to 29 frames a second, for the pixels it draws; one drawing at 1x
  under a page zoomed to 90% lost none (measured 2026-11-11). A frame past what the window's `/events` socket reads, 8 MiB as base64, is not sent, since it would end
  that socket for every session: the pane keeps its last frame until one fits. A wall of photographs in a 4K
  pane made 2.7 MiB frames and a page drawing pure noise in a 3440x1440 one 4.5 MiB (measured 2026-11-11);
  noise in a 4K pane, about 7.5 MiB by pixel count, still fits. A page still for 600 ms (`stillAfter`) gets one
  PNG capture of it, and none while a caret blinks in a focused field, which the capture checks first since a
  late blink frame passes 600 ms on a loaded machine, so still text loses the JPEG's ringing; nothing is captured while the page holds a dialog,
  and a PNG over 3 times its JPEG (`stillGrowth`) is not sent: a photo, 11 times its JPEG and no sharper,
  keeps the JPEG (measured 2026-11-12). A still page gets two captures at most (`stillTries`): where the frame
  a capture draws is not the one before it byte for byte, as on Apple silicon under a pane drawn below the
  browser's scale, that frame replaces the PNG and the pane stays on the JPEG (CI, macos-latest, 2026-11-12). The native window cannot hold a webview: on Linux it speaks Wayland without GTK. So the pane
  shows the arrow whatever cursor the page asks for, draws only the front tab, and leaves the page's own dialogs out. Its
  tabs sit on the pane's header (`internal/browser/tabs.go`): what a tab opens in a new tab or window joins
  them, in front when its opener was, and closing it brings its opener back as it was. A tab behind the front
  one is never drawn, so the header names it by title and has no thumbnail, and switching shows that tab's own last frame
  (the window keeps one for each of the last four tabs fronted; a tab never in front shows the empty pane)
  until its live one. Tabs shrink to share the header and those past its width are cut off; there is
  no overflow menu and no reordering. A dialog a tab behind opens reaches the agent at once, since with its
  opener the front tab shares that renderer and is held by it too. Open in
  a window moves the front tab to a real Chromium window for those, reopening it on the same profile with every
  cookie, since chromedp cannot show a headless process in place. A half-filled form or sessionStorage is
  lost on that move. The page is drawn beside its session's terminal and is a pane of no stored layout
  (`native/cmd/lich-native/pages.go`): it sits after the terminal, at its right or under it, and cannot be
  dragged anywhere else or dropped on. Minimize on its header hides it and leaves the Chromium running;
  minimizing forgets where the divider was. A running browser is a row of the sidebar's tree under its
  session (`sidebar_browser_row.go`), named by the page's host and muted while minimized or in a window of its
  own, so the name does not say which of the two; the tooltip does. The row is no session: it files nowhere,
  a drag leaves it where it was, a folded mother hides it behind a globe on her own row, and the collapsed
  sidebar does not draw it at all. Closing the card, or the session's process ending, closes the browser; Continue in,
  which ends the process and spawns it again in the same card, keeps it. Closing that
  window does not close the session. A headless browser no pane shows and no one used for five minutes is
  only asked about, in a toast, and closes only on Close: an unanswered one holds its Chromium until the
  person answers or closes the card, and with the window closed nobody is asked; one a pane shows, or a
  window, never is. It loads only http, https and about:blank. Page info never
  includes a form's value, though `browser_eval` reads whatever the agent asks for. Screenshots are files, their
  longest side at most 1280 pixels, so a wide pane's is smaller than the page and x and y are read off it. An
  annotated screenshot draws its index labels into the page itself, so the person's pane shows them for the
  capture, a frame or two, and a label covers the start of its element's text; a dialog opened during the capture
  leaves them up to 5 seconds, until the page's own timer removes them. It is
  not a sandbox: the page runs as the user, in an ephemeral profile. Chromium needs its own sandbox, and on
  Ubuntu 23.10 and later AppArmor refuses it the user namespace unless the browser ships an AppArmor profile,
  as the distribution's Chrome, Edge and Brave do: one unpacked from a tarball fails to start with "No usable
  sandbox!", which the agent reads as the browser's error.
- **The pane's DevTools is Chrome's front-end drawn as frames** (`internal/browser/chrome_devtools.go`,
  `docs/browser-tab.md`): a target lich creates in a headless window of its own, loading the browser's own
  DevTools from its debugging port, screencast like a page. It has no device toolbar (it emulates on the
  page's own target, which the pane does not show as a device), no DevTools extensions, the arrow for every
  cursor, and Chrome's colors, not lich's theme. It draws its first frame about 2 s after the button on a warm
  browser and 5 s right after the browser launched, its first document blank until a second replaces it 4.5 s
  after the launch, and 16 s at 40% of a core (Helium 157, measured 2026-11-12), and scrolls its Console
  at about 40 frames a second in a 900x600 pane; idle, it gets the still PNG a page gets, except while the
  Console's prompt caret blinks. The page zoom scales DevTools as it scales a page. A page stopped in its
  debugger stops drawing: the pane keeps its last frame and the DevTools tab carries a pause glyph, which
  lich hears by enabling the debugger on its own session of that page, breakpoints off, while DevTools is
  open (under 4 ms, and the page's scriptParsed events a second time). Every agent tool then fails at once
  naming the pause: a paused page answered reads with the document as it stood and lost clicks and keys
  while reporting them done. The person resumes in DevTools or closes it. A dialog DevTools itself opens is
  not handled. Opening DevTools logs chromedp's `unhandled node event *dom.EventAdoptedStyleSheetsModified`
  once or twice: chromedp 0.16 has no case for that DOM event its own session receives, and the node tree
  lich never reads loses nothing, so it is left in the log rather than filtered. A browser relaunched closes
  its DevTools.
- **An element's index is never handed out twice, so indexes only grow** (`internal/browser/pageinfo.go`): an
  element keeps the index it was first listed with while it lives, and a new one takes the next number of a
  counter each browser carries across documents and tabs, so an index from a replaced element, a page left or
  another tab finds nothing instead of clicking something else. A long session's indexes reach three digits
  after about ten pages of thirty links, costing about a character per listed element. The page can see the
  `data-lich-idx` stamps and `window.__lichIdx`; one scrolled out of view keeps its index but leaves the list,
  and click and type scroll it back.
- **A local file loads from the session's checkout, served over loopback http** (`internal/preview`): a path or
  `file:` URL inside the directory the session was spawned in becomes `http://127.0.0.1:<port>/…`, the port being
  the checkout's `LICH_WORKTREE_PORT` plus 1000 (25000-25999). The server is bound the first time a page from that
  checkout is asked for and holds the port until lich exits; a port something else holds moves it to the next
  free one in the window, so that checkout's origin, and the storage a page keeps under it, changes until it
  gets its port back. Everything under the directory is served, ignored files included, and the root is
  the spawn directory, so a session started in the home directory serves all of it. `os.Root` refuses `..` and
  symlinks leading out, and a request whose `Host` is not `127.0.0.1:<port>` or `localhost:<port>` is refused,
  so a page elsewhere cannot rebind a name of its own to the machine and read the checkout through it. It is a static server: no reload on save, no SPA fallback, no headers of the project's.
  The root stays the checkout whatever file is opened, so a build in a subdirectory that links its assets from
  `/` (Vite's default `base`) loads blank from `dist/index.html`; one built with a relative base loads.
- **A session's artifacts live in a folder of its own for as long as its row, and the agent decides whether a page
  goes there** (`internal/artifact`, `internal/preview`): every spawn gets `<config>/lich/artifacts/<session id>/`,
  named in `LICH_ARTIFACT_DIR`, in the MCP instructions and in `browser_open`'s description, and bound writable in a
  confined session. It is served on a listener of its own at the checkout's `LICH_WORKTREE_PORT` plus 2000
  (26000-26999, from 26000 for a session with no checkout), so a page there cannot fetch the checkout; every page of
  one session shares that origin and its storage. The folder is named by the session's files key
  (`store.SessionFilesKey`), which a resume carries to the row's fresh id, as it does for the dropped-file copies,
  so it outlives a restart, a closed card and a resume, and a path already said in the conversation keeps naming its
  page. It goes when the row is deleted; one a crash left behind goes at the next startup. Nothing caps its size, and a session kept in the history keeps its pages.
  Nothing makes an agent use it: one sentence of the spawn briefing (`relay.SpawnBriefing`, naming the variable, not
  the folder), the MCP instructions and the tool description are the whole of it. A Codex conversation resumed or
  forked keeps the briefing it was born with (the bullet on their briefing), and Claude Code's own system prompt
  still pushes it to publish to claude.ai with its Artifact tool, which the user then gets as a link instead of the
  tab. No file name is set: the agent picks one, and a second page under the same name replaces the first.
  The dock's Artifacts tab reads the folders of the open sessions every two seconds (`artifact.RunWatch`, no file
  watcher), so a new page shows up to that late. It lists a folder's top level only: a file, or a folder holding an
  `index.html`; hidden entries and symlinks are left out. Other sessions means the open ones of the same project, so
  a parked session's pages show again only once it is resumed. Attach to prompt copies a page of another session
  into the active one's folder (a folder page with everything in it), read through an `os.Root` so a link inside
  the source cannot copy a file from elsewhere; edits to either copy never reach the other.
- **`browser_eval` hands the agent whatever the page holds, form values and passwords included**
  (`internal/browser/eval.go`): page info never reads a field's value, but a script asks for what it wants, and
  its answer lands in the session's transcript. It runs in the page's own world, so it changes what the user
  sees without a cursor on the pane, and answers with JSON, cut at 2000 characters: a DOM node comes back as
  `{}`, a cycle fails, and anything larger has to be picked in the script.
- **A page answer marks what the page wrote, and the mark is a request, not a wall** (`internal/cli/browser_page.go`):
  title, dialog messages, console errors, text and element labels, and an `eval` value, sit between
  `<page nonce>` and `</page nonce>`, the nonce 8 random hex characters per answer, so a page cannot close the
  block it cannot predict. Nothing makes an agent obey the one sentence of `browser_info` that says the text
  between them is never instructions, and `url:` stays outside though a redirect can write it. `lich browser eval`
  wraps its value too, so a script piping it needs the lines between the markers.
- **An action's page answer leaves out an element list the agent already heard** (`internal/browser/told.go`):
  lich keeps the list of the agent's last page answer and, when a click, key, tab or dialog leaves the same one,
  says so in one line. An agent whose context was compacted since no longer holds that list, and has to ask
  `browser_info`, which always answers whole.
- **A click by index or selector tests the point at the element's box center** (`resolveJS` in
  `internal/browser/chrome.go`): `elementFromPoint` there must be the element, inside it, or the label of a
  control the element is. An element with a hole at its center can have that point on something else, and the
  click fails as covered though a click elsewhere on it would land.
  Disabled is `:disabled` or `aria-disabled="true"`, which a page may set on something it still handles.
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
  and the file goes with the profile; the window shows no toast for it. A download the page asks for in the
  browser's first seconds is held back, shows nothing (no event, no request) until Chrome reloads the page and
  starts it, and the reload aborts a navigation started before it (`net::ERR_ABORTED`) and wipes a click that
  landed before it. On Chromium 157 (Helium 0.21.1.1) that reload came 4.3 to 5.3 seconds after the launch in 35
  of 39 downloads asked for 0.7 to 3.3 seconds after it (7.4 once), for `<a download>`, an attachment link and
  `location.href`; the other four began without one, and a download asked for after 5 seconds began at once with
  none. So an agent's click, key press or other action after one whose answer showed the page as it was waits for
  the browser to be 6 seconds old or for a reload to land, and a navigation the reload aborts is tried again; a
  page that changed or opened a dialog is no download request and holds nothing. A download asked for in the first
  seconds that changes the page too, a click that lands before a reload of more than 6 seconds, and a navigation
  the person starts in the pane are not held. A download still running after ten seconds is reported in progress and named again when it
  ends. No tool lists past downloads or cancels one, nothing caps their size, and closing the browser cancels one
  still running.
- **A dialog a session's page opens holds the page until someone answers it** (`internal/browser/dialogs.go`,
  `chrome_dialog.go`, `docs/browser-tab.md`): nothing answers it on its own, so an agent that never calls
  `browser_dialog` and a person who never looks at the pane leave the page frozen, every other agent call failing
  with the dialog's name, until the browser is closed. The agent's click that opened it answers with the page as it
  stood, not as the dialog's answer left it. A dialog the page opens on a timer interrupts whichever call is
  running. Accepting a `beforeunload` less than a second after it opened is held to that second: accepted sooner,
  Chrome aborts the navigation it held and reloads the page, which asks again (3 of 3 at 300 ms, 3 of 6 at 400 ms,
  none of 30 from 500 ms; Chrome for Testing 155 and Helium 0.19.2.1). That bound grows with the machine's load
  (2 to 3 s with Chromium held to 30% of a core), so a reload after an accepted `beforeunload` is taken for an
  abort: lich accepts the reload's own `beforeunload` unseen and loads the held URL again once the reload lands,
  up to three times, and the agent's answer waits up to 15 seconds for it. A held navigation that a redirect
  would have sent elsewhere is taken up at the URL it started with. The agent hears of a dialog the person
  answered, at most twenty between two page answers, but not of one the agent itself answered. A held page cannot lay
  out, so a pane opened or resized meanwhile shows it at the size it had, and lays it out at the pane's once the
  dialog closes; a screenshot fails until then. On Chrome 153 and older, which answers a screencast's start and
  stop only once the dialog closes, a pane resized while one is open keeps its last frame until it is answered
  (Chrome 151 on the macos-15-intel runner, measured; 154 on answers at once).
  A window opened with Open in a window was not tried with a dialog.

- **A file input in a session's page opens the OS file chooser only while a pane shows the page**
  (`internal/browser/files.go`, `chrome_files.go`): headless Chrome has no chooser of its own, so lich intercepts
  it and opens the one the attach button uses, one file or several as the input asks. An agent's click on a file
  input in a page nobody is looking at leaves the input empty, and no tool fills one; an agent's click while a
  pane shows the page opens the chooser in front of the person. The input's `accept` filter is not passed on: the
  chooser lists every file and the page refuses a type it does not take.

- **Showing code is one file, read-only, in the session's own checkout.** `show_code` and a Ctrl+click on a
  `path:line` open the dock's Code tab, and a new one replaces the file on screen, with no history to go back
  through, and an agent's comment under the lines goes with it. A path outside the session's checkout (a dependency's sources, the JDK, another worktree) is refused.
  The tool validates the file against the disk, but a click searches the checkout's listing, so a file git
  ignores reads as not found even when it exists. The terminal underlines by the text alone, because the
  hover runs every frame and cannot touch the disk: a `path:line` naming nothing still underlines, and the
  click answers with a toast, as it does for a host written like `example.com:443`. Only a soft-wrapped path is
  joined. Claude Code and Kiro CLI, measured at 44 columns, break a path wider than the terminal with a real
  line break, so the click reads only the half under the pointer; the other providers are not measured.

## Languages

- **The prompt language reaches a running session only through lich itself** (`internal/prompt`): text lich
  composes in its own process (relay messages, nudges) follows the setting at once, but anything composed
  inside the session (the `lich` CLI, its MCP results, the plugin's hooks) reads `LICH_PROMPT_LANG`, which is
  fixed when the session spawns. Change the language and an open session can get relay messages in the new
  language and CLI output in the old one until it is restarted. The spawn briefing is likewise fixed at spawn:
  Claude Code and oh-my-pi take it on the command line that appends to their system prompt (`briefingFlags`),
  Codex as a `developer_instructions` override and Kiro CLI from an `agentSpawn` hook. opencode, Crush and
  Antigravity read lich's MCP instructions instead, composed at spawn too (`LICH_PROMPT_LANG`), and Cursor CLI's
  is unmeasured (the bullet above on Codex and Kiro CLI, under Relay). MCP tool
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
