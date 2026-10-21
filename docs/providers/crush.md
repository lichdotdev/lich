# Crush

`crush` (`internal/providers.Registry`). It is xdg-basedir: config under `$XDG_CONFIG_HOME/crush` and data under
`$XDG_DATA_HOME/crush`. Its conversations are not there but in the checkout it was started in, as a database at
`.crush/crush.db`, so the same id proves nothing about another checkout. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Crush has no
per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

Crush asks once per checkout whether to initialize the project, and text typed into that question runs an init
turn (measured 2026-10-08). It keeps the answer as an empty `.crush/init` at the checkout's top (measured on
v0.98.1), so lich writes it (`internal/terminal/crush_init.go`), with Crush's own `*` `.gitignore` beside it,
before every Crush spawn, and the offer to write `AGENTS.md` never comes up in a lich session. Crush reports
nothing before its first turn, so the start gate has only a quiet screen to read.

`--effort` is refused: Crush has no such option (0.88.0).

## Hooks

Crush reports no session state at all (the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)), so no turn ever opens or closes there. Two things
follow. A waiting card sends no reason and keeps the generic "Waiting on you"
(`statusRung` in `native/cmd/lich-native/statusrung.go`), which means the harness never spoke and not that the block
is trivial. And keep-awake has nothing to listen to (`internal/awake`, `turnLog.onOpen`): a Crush session left
working behind a locked screen sleeps as it always did, and no card says so.

Its only hook event, `PreToolUse`, fires once per tool call (measured 2026-09-03 against Crush 0.88.0: three tool
calls in one turn, three POSTs), each a `/session-start`, which is how hands-on time hears a Crush turn at all. A
resume typed inside Crush (v0.98.1) is reported with its next tool call, so a turn may already have run in both
cards before the two-cards-one-conversation warning arrives.

## Transcript & cost

Cost only: `sessions.cost` per conversation, with no window recorded beside it, so the footer shows the figure and
no ring. Crush rolls a sub-agent's spend into its parent (`updateParentSessionCost`), so summing its children
would bill them twice. That is measured behaviour of Crush's schema, not a promise it made.

The History tab reads Crush's project index at `$XDG_DATA_HOME/crush/projects.json` (else `~/.local/share`),
measured on Linux only; where Crush keeps it on Windows and macOS is not measured, and a miss there lists no Crush
conversation.

## Plugin install

Crush has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). Crush records nothing
about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich reads
it as not installed. Crush below 0.88.0 ignores those lines in silence, which is why the install asks its version
first.

## MCP

Crush's config block registers lich's MCP server by the absolute path of the binary that installed it.

## Account

Crush runs on the user's own API keys, so there is no plan account to name (`internal/quota`, `Plan.Account`),
which is the same reason it has no gauge.

## Sandbox

Nothing provider-specific.

## Known limits

A long paste folds by a constant with no setting and no key that unfolds it, so Unfold long pastes has no switch
here.

Crush reads no terminal background at all, so a theme switch never reaches it.

Profiles are unmeasured.
