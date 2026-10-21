# Cursor CLI

`cursor-agent` (`internal/providers.Registry`). Cursor keeps its state in two directories. Its config dir is
`$CURSOR_CONFIG_DIR` ‖ `$XDG_CONFIG_HOME/cursor` ‖ `~/.cursor`, not xdg-basedir, the fallback being the home
directly, and it holds the chats and `cli-config.json`. The credentials are not in it on Linux: `auth.json` is
`$XDG_CONFIG_HOME/cursor/auth.json` else `~/.config/cursor/auth.json` (2026.10.01, read in the bundle and
measured against a throwaway home), which is the config dir only when `XDG_CONFIG_HOME` is set; macOS keeps it in
`~/.cursor/auth.json` (after the Keychain) and Windows in `%APPDATA%\Cursor\auth.json`, read in the same bundle and
resolved by `providers.CursorAuthDir`. But `~/.cursor` is resolved off the home with no variable
in the way at all, and that is where `mcp.json`, the per-project transcripts and the CLI state live. The chat
itself is SQLite at `chats/<md5 of the resolved cwd>/<chatId>/store.db`. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): the CLI has
no append flag, so what a Cursor session knows about lich is its tool list.

In a fresh worktree Cursor CLI asks whether the folder is trusted, and text typed into that question with an
Enter behind it exits the CLI (measured 2026-10-08), which is why the start gate waits for its session-start
report. Trust (`internal/terminal/cursor_trust.go`): Cursor finds a marker
(`~/.cursor/projects/<slug>/.workspace-trusted`) for the folder or a parent below the home, and a worktree sits
beside its repository rather than under it, so before each spawn in a worktree lich writes the marker when the
main checkout is trusted; its profiles need nothing, the marker following the home (measured on 2026.10.16,
Windows not). A confined session is spawned with `--trust`, which also writes its marker, so that folder stays
trusted outside the sandbox too.

`--effort` is refused: the effort is part of the model id (`claude-opus-4-8-high`), and the `model[effort=high]`
override its own `--help` advertises spawns into "Cannot use this model" (2026.08.11), so a caller who wants it
passes that id as `--model`. On resume the model comes back from the conversation (2026.09.15).

## Hooks

A Cursor CLI session reports through Claude Code's plugin, or not at all (`internal/agentplugin`,
`internal/terminal/start.go`, `providerKind`). lich installs no plugin into Cursor, and it does not have to: the
CLI executes every Claude Code hook on the machine, the user's own and each installed plugin's (measured on
2026.08.11: `hookSource: claude-user` and `claude-plugin`, with `${CLAUDE_PLUGIN_ROOT}` expanded). So on a
machine where the lich plugin is installed in Claude Code, a Cursor session reports the chat id it is running and
the files it touches, with nothing installed there, and on a machine without it that session reports nothing at
all. Nothing on the card says which of the two it is.

**The turn arrives; the wait does not.** Of the nine events the plugin registers, Cursor's TUI delivers every
one but `Notification`, `UserPromptSubmit` and `Stop` included (measured 2026.08.11 through 2026.10.01;
`cursor-agent -p` fires only `SessionStart` and `SessionEnd`). So a Cursor card spins, rings, keeps the
machine awake and records a last turn like a Claude Code one, and the relay closes an errand when the turn
ends without an answer. A permission prompt reads as `busy`, never `waiting`. Esc ends a turn with two `stop`
hooks, `aborted` then `error`, and lich keeps only the first (`terminal.repeatedEnding`,
[`../hooks/session-state.md`](../hooks/session-state.md)).

Cursor CLI skips `sessionStart` on any resume (2026.10.16), so a conversation switched inside the app is never
reported: the card's row keeps the old id and a restart resumes the conversation the card opened with.

## Transcript & cost

Cursor keeps its chats per checkout, so a resume asked without the session's own working directory answers
"conversation gone" (`internal/terminal/transcript.go`), the same shape as Crush.

lich reads no usage from it: the chat is filed as SQLite lich has no reader for, search and recap included, and a
reader means one for Cursor's own schema, which is not a contract anybody promised to keep.

## Plugin install

lich installs nothing into Cursor. Installing for Cursor refuses while Claude Code has no plugin, its version is
Claude Code's, and its row offers no update of its own: the update is the Claude Code row's, one line up the same
screen.

## MCP

Cursor takes no MCP server on its command line and reads none from a Claude Code plugin, so lich's own tools come
from an `mcpServers` document its install writes under `~/.cursor`.

## Account

The plan gauge names the account a session spends (`internal/quota`, `Plan.Account`): `~/.config/cursor/auth.json`'s
token is a JWT whose `sub` is an opaque `github|user_…` and whose claims contain no email, so lich reads
`authInfo.email` from `cli-config.json` in the config dir.

The usage route is the cursor.com dashboard's own (`/api/usage-summary`), sent the CLI's token as the dashboard's
session cookie with the dashboard's Origin and Referer, which its CSRF check demands. It reads only the CLI's own
login (`auth.json` where the CLI writes it, `providers.CursorAuthDir`, and on macOS the Keychain item cursor-agent
2026.06+ writes), never the Cursor editor's session, which lives in another app's SQLite database. Accounts still
on request-quota billing (the dashboard's older `/api/usage` route) show their plan with no windows.

## Profiles

Linux and Windows. Cursor CLI (2026.10.01, measured 2026-10-12 in throwaway homes) keeps its state in three places
that three different variables move. The config dir (`cli-config.json`, which names the account,
`statsig-cache.json`, `chats/<md5 of the cwd>/<id>/store.db`) is `$CURSOR_CONFIG_DIR`, else
`$XDG_CONFIG_HOME/cursor`, else `~/.cursor`. The login, `auth.json`, is not in it on Linux, so `CURSOR_CONFIG_DIR`
splits the chats from the account without splitting the account (issue #765). `projects/` (transcripts) answers to
`CURSOR_DATA_DIR` else `~/.cursor`, and `mcp.json`, `hooks.json`, `skills/`, `skills-cursor/`,
`agent-cli-state.json`, `policy.json` and `sandbox-policies/` hang off `$HOME/.cursor` with no variable, so they are
every profile's. A resume finds a chat only under the active config dir. The install directory
(`~/.local/share/cursor-agent/versions/<v>/`) and the logs under `$TMPDIR` never move, and `XDG_CACHE_HOME` is not
moved, so the compile cache is every profile's.

On Linux a profile is a config home of its own (`XDG_CONFIG_HOME` pointed at `<lich config
dir>/profiles/cursor/<slug>/`, `internal/xdgprofile`, `internal/profilestore/linkhome.go`) made of links to every
entry of yours but `cursor/`, which is the profile's. The mirror is of `~/.config`, so `~/.config/lich` (lich's
config dir, profiles included) is absent from its profiles, and a tool in a Cursor profile session that reads
`$XDG_CONFIG_HOME/lich` finds nothing; on Windows the same holds for `%APPDATA%\lich`. On Windows (read in the
2026.10.01 bundle, `cli-credentials`, on 2026-10-17, never run) the login is `%APPDATA%\Cursor\auth.json`
(`APPDATA`, else `<home>\AppData\Roaming`) and the config dir is `~/.cursor`, so a profile is a link home of
`%APPDATA%` whose `Cursor` folder is its own, with `CURSOR_CONFIG_DIR` pinned to that same folder so the chats and
`cli-config.json` go with the login (`internal/xdgprofile/cursor_windows.go`). That folder is also where the Cursor
editor keeps its user data, so an editor started from inside a Cursor profile session sees none of yours. On macOS
the login is in the Keychain by default: services `cursor-access-token`, `cursor-refresh-token` and `cursor-api-key`
under the account `cursor-user`, names no variable changes, in the one login keychain every profile's session would
read, so it has no profiles there. `AGENT_CLI_CREDENTIAL_STORE=file` moves the login to `$HOME/.cursor/auth.json`,
which only `HOME` moves; lich does not set either. `CURSOR_CONFIG_DIR`, `CURSOR_API_KEY` and `CURSOR_AUTH_TOKEN` are
dropped from a profile's spawn; the provider API-key variables are not.

`cursor-agent login` runs in a shell card (`xdgprofile.Service`), and nothing in lich sees it finish; the list is
read again when Settings opens and when the window regains focus after a minute. "Logged in" is `isAuthenticated`
from `cursor-agent status --format json`: logged out it is `{"status":"unauthenticated","isAuthenticated":false,…}`
with exit 0, and the logged-in shape (`userInfo.email`) was read in the bundle, not run. There is no plan per
profile in Settings and no usage on the Continue in rows; the footer of a session reads the plan of the login its
own process sees. Continue in (`xdgprofile.MoveConversation`) moves a chat's `chats/<md5 of the cwd>/<id>/`
directory between config dirs, both ways (Personal's is `~/.cursor` when `XDG_CONFIG_HOME` is unset, a profile's is
its own `cursor/`), and a conversation adopted from History runs in the profile whose directory holds it. Resume and
the History tab look in every profile's chats.

## Sandbox

The two directories above are two mounts (`internal/sandbox/sandbox.go`). On a machine with `XDG_CONFIG_HOME` set
they are different directories, and a sandbox binding only one is a session that cannot see its own MCP servers.

## Known limits

A long paste folds by a constant with no setting and no key that unfolds it, so Unfold long pastes has no switch
here.

Cursor CLI asks for mode 2031 in its shipped code, so a theme switch should reach it (not driven).

The plugin's script reports `claude`, the argument Claude Code's own registration passes it, and lich drops that
name for a card whose provider it chose itself. A **shell** session running `cursor-agent` by hand has only the
report to go on and wears Claude's mark.
