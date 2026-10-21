# opencode

`opencode` (`internal/providers.Registry`). It reads its directories through the xdg-basedir convention on every
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
walks the `parent_id` chain. That is measured behaviour of opencode's schema, not a promise it made.

## Plugin install

opencode has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). opencode records
nothing about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich
reads it as not installed.

## MCP

A new MCP tool reaches opencode a release later than everyone else (`internal/cli/mcp.go`, `mcpTools`; the
registration table in [`../cli.md`](../cli.md)). Every other harness is handed lich's own server, so a tool added
there is in that session's list on the next spawn. opencode cannot register an MCP server from a plugin, so its
plugin defines each tool itself in the companion repo (`lichdotdev/lich-plugin`, `opencode/lich.js`), which means a
tool arrives there only once that repo cuts a release and the user reinstalls the plugin. Until they do it is
missing from that session's list while it is in every other. `lich rename` works there like anywhere else; it is
discovery that lags, which is the whole reason the tools exist.

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
