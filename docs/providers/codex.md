# Codex

`codex` (`internal/providers.Registry`). Its config directory is `$CODEX_HOME`, else `~/.codex`, and it files a
rollout by the date it started: `sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<id>.jsonl`. The limits these sit
under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Codex has
no per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

## Hooks

Codex's `PermissionRequest` carries only the thing being asked about (`tool_name`, `tool_input`) and no message
of its own, so a waiting card reads a bare `Bash` (`frontend/src/components/sidebar/SessionStatusRung.tsx`; the
mapping table is in [`../hooks/session-state.md`](../hooks/session-state.md)). That says which card to open and
not what it will ask.

## Transcript & cost

Nothing provider-specific.

## Plugin install

Driven through Codex's own plugin CLI (`internal/agentplugin`), so lich writes none of the files itself and does
not track the installed version in a marker line of its own.

## MCP

The session is handed lich's own MCP server on its command line (`-c` overrides for its `mcp_servers` table,
`providers.AcceptsMCPServer`), so a tool added here is in that session's list on the next spawn.

## Account

The plan gauge names the account a session spends (`internal/quota`, `Plan.Account`): Codex carries an `email`
claim in the OIDC id token beside its access token. It is read unverified, and discarded once its `exp` has
passed, because the CLI can rotate the access token without rewriting the id one and a gauge under a login the
user has left is worse than one under no name.

## Profiles

A profile is a `CODEX_HOME` of its own with its own `codex login` (`internal/codexprofile`). It links Personal's
`hooks.json`, `AGENTS.md`, `skills/` and `rules/` and keeps its own `auth.json`, rollouts, history, thread
database, plugin downloads and `config.toml`. That file is rebuilt from Personal's before every spawn
(`config.go`): Personal's tables, minus its `[plugins.*]` and `[marketplaces.*]`, then the profile's own plugin
set and the project and hook trust it was given that Personal was not. Plugins are ported with `codex plugin add`
run as the profile, after copying Personal's `[marketplaces.<name>]` declaration (ref included) into the profile
and `codex plugin marketplace upgrade` fetching it, and the lich plugin is installed in each profile after its
login, pinned as in Personal. Every reader of rollouts (resume, usage, search, the History tab) scans all of
them and resumes under the profile that holds the rollout. What it shares, what it moves and where it
disagrees with Claude Code's profiles is in [`../ceilings.md`](../ceilings.md).

## Sandbox

Nothing provider-specific.

## Known limits

Nothing provider-specific.
