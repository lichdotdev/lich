# opencode

`opencode` (`internal/providers.Registry`). opencode 2 is the same provider: it ships under the same binary name,
reads the same config and database, and its upgrade replaces 1.x in place. What it changes for lich is in
[opencode 2](#opencode-2) below; everything else on this page holds for both. It reads its directories through the xdg-basedir convention on every
platform rather than the OS-native ones: config under `$XDG_CONFIG_HOME/opencode`, and every conversation in one
database at `$XDG_DATA_HOME/opencode/opencode.db`. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): opencode has
no per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

opencode files a fork under the parent's directory, not the new checkout's (measured on 1.18.23): the copied
session's `directory` column is the one the original ran in, and its `parent_id` is left empty, so opencode's own
session list places a fork in the checkout it came from and records no lineage. lich's own card is right, since
it carries the worktree it was opened in and `origin_session_id` names the parent, but the two disagree and only
lich's side is visible in lich.

opencode asks nothing in a fresh worktree, but under load it goes quiet mid-boot and text typed then lands before
its TUI does; it reports nothing before its first turn, so the start gate has only a quiet screen to read.

`--effort` is refused: opencode's `--variant` lives on `run` only (1.18.31), so the TUI lich spawns would drop
the level and hand back a session on its default. On resume the model comes back from the conversation
(1.18.31).

With Unfold long pastes on, the spawn sets `experimental.disable_paste_summary` through
`OPENCODE_CONFIG_CONTENT`, extending one the user exports rather than replacing it. opencode's own "Disable paste
summary" command is stored in its state and wins over any config, so once used there the switch changes nothing.

## Hooks

opencode's `.asked` events carry only the thing being asked about (`permission`, `action`), so a waiting card
reads a bare `edit` (`statusRung` in `native/cmd/lich-native/statusrung.go`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). That says which card to open and not what it will ask.

opencode reports its own abort, and reports it as the turn *finishing* (`session.status idle`), so an interrupted
opencode card wears the same solid ring a completed turn does — nothing in the event says which happened, and the
provider's own word outranks the keystroke. A conversation switched with `/sessions` raises nothing (1.18.35), so
the card's row keeps the old id and a restart resumes the conversation the card opened with.

## Transcript & cost

Cost only: `session.cost` per conversation, with no window recorded beside it, so the footer shows the figure and
no ring. opencode files each sub-agent as a session of its own and does *not* roll it into the parent, so the read
walks the `parent_id` chain. That is measured behaviour of opencode's schema, not a promise it made. The cost
tooltip also carries the newest finished assistant message's cache line, from `tokens.input`, `tokens.cache.read`
and `tokens.cache.write` on that `message` row, whose input already leaves the cache out. A sub-agent's turns are
its own session's and are not read.

## Plugin install

opencode has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). opencode records
nothing about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich
reads it as not installed.

## MCP

The session is handed lich's own MCP server at spawn, as a `lich` entry under `mcp` in `OPENCODE_CONFIG_CONTENT`
(`internal/terminal`, `mcpEnv`; measured on 1.19.4): opencode merges that inline config over every file it reads,
so the user's own servers stay and nothing is written to disk. A tool added to `internal/cli/mcp.go` is in that
session's list on the next spawn, as `lich_<tool>`.

## Account

opencode runs on the user's own API keys unless an OpenCode Go key is stored; that subscription gets a gauge
(`internal/quota`, `opencodePlan`) but names no account (`Plan.Account`), since its usage route returns none. The
route is the OpenCode Go console's. Its key is read from the `opencode-go` entry of opencode's `auth.json`, so a
key given only through `OPENCODE_API_KEY` or `OPENCODE_AUTH_CONTENT` is not read (the first is shared with Zen
keys, which have no plan). The 200 shape of that route was ported from the opencode console as the Orca project
parses it, never measured here: only its 401 and 403 were. A 403 there means the key has no Go subscription, and
reads as signed out.

## Profiles

opencode (1.18.35, measured 2026-10-12 in throwaway homes) reads its data from `$XDG_DATA_HOME/opencode` and nothing
else moves it: `OPENCODE_CONFIG_DIR` and `OPENCODE_TEST_HOME` leave it where it was. `opencode.db` holds the logins
(its `credential` and `account` tables, beside `auth.json`) together with every conversation, so the folder cannot
be split. A profile is a data home of its own (`XDG_DATA_HOME` pointed at `<lich config
dir>/profiles/opencode/<slug>/`, `internal/xdgprofile`, `internal/profilestore/linkhome.go`) made of links to every
entry of yours but `opencode/`, which is the profile's. The config under `~/.config/opencode` (plugins, agents, MCP
servers, lich's plugin) is not under the data home and is every profile's, and `XDG_STATE_HOME` is not moved, so
opencode's locks are every profile's too. `OPENCODE_DB`, `OPENCODE_AUTH_CONTENT`, `OPENCODE_CONSOLE_TOKEN` and
`OPENCODE_API_KEY` are dropped from a profile's spawn; the provider API-key variables are not.

The same holds on macOS and Windows: the 1.18.35 bundle, read on 2026-10-17 and never run there, resolves its data as
`$XDG_DATA_HOME`, else `<home>/.local/share`, with `opencode` under it, in JavaScript with no branch per platform,
and keeps no secret in the Keychain or the Credential Manager.

`opencode auth login` is an interactive picker, so a login runs in a shell card (`xdgprofile.Service`) and nothing
in lich sees it finish; the list is read again when Settings opens and when the window regains focus after a minute.
"Logged in" is a non-zero count at the end of `opencode auth list`, which names no account. Every reader of
conversations (resume, usage, search, the History tab) scans each profile's database and resumes under the one
holding it, and a conversation adopted from History runs in the profile whose directory holds it. A conversation
cannot move to another profile without moving rows between two databases, which lich does not do, so Continue in is
refused before the session is stopped and the card menu does not offer it.

## Sandbox

Nothing provider-specific.

## Known limits

A theme switch reaches it: opencode turns on mode 2031 (driven in a PTY).

## opencode 2

Measured on 2.2.5, in throwaway homes, beside 1.19.4. lich tells the two apart by `--version`, once per binary
(`providers.IsOpencode2`).

- **Spawn.** The root command has no `--model` and no `--fork` (both are `run` flags), and by default attaches to
  a background service every opencode shares, which reads its environment once. lich spawns `--standalone`, names
  the model as `model` in `OPENCODE_CONFIG_CONTENT`, and forks through `opencode api --standalone POST
  /api/session/{id}/fork` before resuming the copy with `--session` (`internal/terminal/opencode2.go`). The
  copy is filed under the checkout the fork ran in. `--session`, `--auto` and the inline config (1.x's `mcp` shape,
  normalized) are unchanged. 2.x still turns on mode 2031.
- **Paste summary.** 2.x drops `experimental.disable_paste_summary`; the switch lives in its terminal config, so
  Unfold long pastes sets `prompt.paste` to `full` through `OPENCODE_CLI_CONFIG_CONTENT`.
- **Database.** Conversations move to `session_v2`, one `session_message` row per message (a prompt's words
  under `data.text`, an answer's under the `text` entries of `data.content`). `session_part` stayed empty.
  2.x copies a 1.x conversation over when it first touches it, so lich reads each conversation from the newer
  table it is in (`internal/terminal/sessiondb.go`). Tokens and cost keep their shape; a sub-agent is still a
  session of its own under `parent_id`, not rolled into its parent (measured on tokens, since the free model it
  ran on billed $0).
- **Hooks.** 2.x loads the module's default export and calls `setup(ctx)`; 1.x calls the same export's `server()`
  and also its `setup()`, with a context missing `event` and `session`, which is how the module tells them
  apart. The reports ride `session.execution.started` (session start, then busy), `session.execution.succeeded`,
  `failed` and `interrupted` (done), `session.renamed` (title), `permission.asked` (waiting, with its `action`)
  and the `write`, `edit` and `patch` tools finishing (touched). No session event carries a parent, so each
  session is looked up once with `ctx.session.get`. The question tool's event is not measured: a
  non-interactive run never asked one. The module goes in `plugins/`, the only global directory 2.x reads.
- **Profiles and plan.** `auth list` and `auth login` run `--standalone`, since the shared service would hold
  another profile's data home. `auth list` prints one row per integration and no count; a row ending in
  `stored` is a login. The OpenCode Go key moves to the `credential` table of `opencode.db` (an active
  `opencode-go` row, `{"type":"key","key":…}`), which the gauge reads before `auth.json`.
