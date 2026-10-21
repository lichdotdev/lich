# Antigravity

`agy` (`internal/providers.Registry`). Its state lives under `~/.gemini`, read off the home alone: 1.1.19 falls
back to a hardcoded `.gemini` when it cannot resolve the home and honours no environment variable of its own.
The OAuth credentials sit at that root, the customizations under `config/`, and a conversation is SQLite at
`antigravity-cli/conversations/<id>.db` with its JSONL beside it under
`antigravity-cli/brain/<id>/.system_generated/logs/`. The limits these sit under are in
[`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Antigravity
has no per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

In a fresh worktree Antigravity asks whether the folder is trusted, and text typed then is lost to a second
screen (measured 2026-10-08); it reports nothing before its first turn, so the start gate has only a quiet screen
to read. It trusts only the exact folders in `~/.gemini/antigravity-cli/settings.json` `trustedWorkspaces`, so
worktrees and subfolders of a trusted repository asked again (measured on 1.3.5): before each spawn lich adds the
folder when its checkout's top or main checkout is listed, and every confined folder whatever the list says, so a
folder once opened confined stays trusted outside the sandbox (`internal/terminal/antigravity_trust.go`).

A misspelled effort level starts without a word. On resume 1.1.25 comes back on the saved default its own
`/model` and `/effort` write (`settings.json`), not on the conversation's, so a model or level lich set at birth
falls back to that default.

## Hooks

An Antigravity card sends no reason with a `waiting` and keeps the generic "Waiting on you"
(`statusRung` in `native/cmd/lich-native/statusrung.go`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). It does not report `waiting` in the first place: its
permission prompt raises no lifecycle event that has been measured, so there is nothing to hang a reason on. A
bare card there means the harness never spoke, not that the block is trivial.

Antigravity 1.4.0 carries the conversation id on every `PreInvocation` (read in the binary, not run), which is
what reports a conversation switched inside the app.

## Transcript & cost

lich reads no usage from it: the conversation is filed as SQLite lich has no reader for, and a reader means one
for Antigravity's own schema, which is not a contract anybody promised to keep.

## Plugin install

Antigravity has a plugin CLI and lich installs around it (`internal/agentplugin/antigravity.go`): `agy plugin
install` takes a directory, and its only remote form clones that repository's default branch, while lich
installs a *release*, whose version is what a card reports and what the next update compares against. So lich
writes the customization directory itself (`~/.gemini/config/plugins/lich/`), with three consequences. The
installed version lives in the manifest lich writes rather than in a marker line, since the directory is lich's
outright, and a copy the user installed through `agy plugin install` carries no version, so it reads as not
installed. The registration's commands are relative, resolved against the directory holding `hooks.json`
(Antigravity runs a hook through `sh -c` from there and sets no plugin-root variable of its own, both measured
on 1.1.19), so moving that directory by hand breaks every report until the next install. And lich writes the
hooks and their scripts only: the plugin's skills come with `agy plugin install`, not with this, which is the
same thing already true of opencode, oh-my-pi and Crush.

## MCP

Nothing provider-specific.

## Account

`~/.gemini/oauth_creds.json` carries the same `email` claim Codex's id token does, and it is deliberately not
read (`internal/quota`, `Plan.Account`): the gauge comes from asking `agy` for `/usage` (`antigravityPlan`),
whose reply names no account, so the Antigravity gauge carries no name.

Antigravity keeps its Google login in the OS keyring and mints its own token, so lich runs `agy -p /usage
--output-format json` (measured on agy 1.2.5: no turn, no tokens, no conversation) with the session's own
environment and the `agy` on lich's PATH, never a configured binary that may be a wrapper. An agy older than
1.1.11 answers that as a prompt and spends a turn, so it is never asked; a newer one that answers with a turn
anyway stops being asked until lich restarts. A machine with no `agy` shows no Antigravity plan at all. That run
takes two to three seconds per reading.

Antigravity cannot have profiles: its `~/.gemini` is fixed by the CLI.

## Sandbox

Nothing provider-specific.

## Known limits

A long paste folds by a constant with no setting and no key that unfolds it, so Unfold long pastes has no switch
here.

Antigravity reads the terminal's background once at boot and never hears of a theme switch.
