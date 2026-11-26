# Crush

`crush` (`internal/providers.Registry`). It is xdg-basedir: config under `$XDG_CONFIG_HOME/crush` and data under
`$XDG_DATA_HOME/crush`. Its conversations are not there but in the checkout it was started in, as a database at
`.crush/crush.db`, so the same id proves nothing about another checkout. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Crush has no
per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions. Crush does hand
those to the model, in its system prompt inside `<mcp-instructions>` (measured 2026-11-07 on v0.100.0, `crush
run` against a local OpenAI-compatible stub that recorded the request, since the account's provider had no
quota left: the system message carried the probe server's initialize `instructions` and lich's own).

Crush asks once per checkout whether to initialize the project, and text typed into that question runs an init
turn (measured 2026-10-08). It keeps the answer as an empty `.crush/init` at the checkout's top (measured on
v0.98.1), so lich writes it (`internal/terminal/crush_init.go`), with Crush's own `*` `.gitignore` beside it,
before every Crush spawn, and the offer to write `AGENTS.md` never comes up in a lich session. Crush reports
nothing before its first turn, so the start gate has only a quiet screen to read.

`--effort` is refused: Crush has no such option (0.88.0).

## Hooks

Crush has 13 hook events from 0.99.0 (read off its source; measured on 0.100.0 in the TUI and in `crush run`,
which fire the same ones). lich registers the state reports, the session id and the tool line on them, and the
mapping is in [`../hooks/session-state.md`](../hooks/session-state.md). Every payload carries `session_id` and
`cwd`, and the environment carries `CRUSH_EVENT`, `CRUSH_SESSION_ID`, `CRUSH_CWD`, `CRUSH_PROJECT_DIR`, and on a tool
event `CRUSH_TOOL_NAME` plus `CRUSH_TOOL_INPUT_COMMAND` or `CRUSH_TOOL_INPUT_FILE_PATH`. Only the tool events and
`PreCompact` read a hook's output; the rest observe.

| Event | Fires | Payload beyond the id | lich registers |
| --- | --- | --- | --- |
| `SessionStart` | the first prompt of a new conversation, not at launch | `source` (`startup`, `resume` from `crush run --continue`) | session id |
| `TurnStart` | every prompt, before any tool | — | session id, `busy` |
| `PreToolUse` | before each tool | `tool_name`, `tool_input`, `tool_use_id` | tool line, touched, subagent veto |
| `PostToolUse` | after a tool that succeeded | the same, `tool_response` | `busy` |
| `PostToolUseFailure` | after a tool that failed or was denied | the same, `error` | `busy` |
| `PermissionRequest` | a permission prompt shown | `tool_name`, `tool_input` | `waiting` |
| `PermissionResult` | that prompt answered | the same, `decision` | nothing: it runs beside the tool and can land after `TurnEnd` |
| `TurnEnd` | the turn finished | `last_assistant_message` | `done` |
| `StopFailure` | the turn ended on a provider error, instead of `TurnEnd` | `error` | `done` |
| `Interrupt` | the user stopped the turn, instead of `TurnEnd` | — | nothing: lich reads the key itself |
| `PreCompact` | before a compaction | `trigger` (`manual`, `auto`) | nothing (session-state.md, compaction) |
| `PostCompact` | after it | `trigger` | nothing |
| `SessionEnd` | Crush exits | `reason` (`exit`) | `idle` |

Subagents fire none of the tool or turn events, only the outer `agent` call does. A resume typed inside Crush is
reported at its next prompt, so a turn may already have run in both cards before the two-cards-one-conversation
warning arrives.

Below 0.99.0 `PreToolUse` is the only event, and the block lich writes there is the one it always wrote: the
session id rides every tool call, and the card carries no state. A waiting card would send no reason, keep-awake
has nothing to listen to, and a turn that calls no tool reaches lich not at all. The block lich wrote for an older
Crush stays until the plugin is installed again, so a Crush upgraded to 0.99.0 reports state from the next install
or update, not before. Once `crush --version` reads 0.99.0 or newer, lich's next pass writes the block again,
with or without a newer plugin release (`crushBlockBehind`).

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
first, and the same answer decides whether the lifecycle events are registered. The relay takes a Crush session's
silence as meaningful only when lich's block registers `TurnEnd`.

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
