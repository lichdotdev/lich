# Claude Code

`claude` (`internal/providers.Registry`), the default provider and the companion plugin's home. Its config
directory is `$CLAUDE_CONFIG_DIR`, else `~/.claude`, with the account in `~/.claude.json` beside it. A
conversation is one JSONL at `projects/<slug>/<id>.jsonl`, and a sub-agent's is a file of its own under
`projects/<slug>/<id>/subagents/`. The limits these sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich appends to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags` →
`relay.SpawnBriefing`): a session is spawned with `--append-system-prompt` carrying lich's own briefing, so text
the user never wrote is in every session's prompt and in `/proc/<pid>/cmdline`.

In a fresh worktree Claude Code asks whether the folder is trusted, and text typed into that question with an
Enter behind it exits the CLI (measured 2026-10-08), which is why the start gate waits for its session-start
report (`internal/terminal/startgate.go`).

Trust (`internal/claudeprofile/trust.go`): Personal needs nothing carried, since Claude Code finds the answer
for a worktree in its main checkout and for a folder in its parents (measured on 2.1.302). A confined session
skips the question with `CLAUDE_CODE_SANDBOXED=1`.

A misspelled effort level warns and runs at the default. On resume the model comes back from the conversation
(2.1.274) and the effort does not: it resumes on the saved default that Claude Code's own `/effort` writes
(`settings.json`), so a level changed inside the session survives while one lich set at birth falls back.

Ultracode (`ultracodeArgs`) is a Claude Code mode, standing multi-agent workflow orchestration, and no other
provider has one, so `lich open --ultracode` and `open_session`'s `ultracode` are refused for every other kind
and Settings offers the switch on Claude Code alone. lich passes it as `--settings '{"ultracode":true}'` rather
than `--effort ultracode`, which pins the effort to xhigh. Claude Code drops it on `--resume` (measured on
2.1.288), so unlike the model and effort it is passed on every spawn, and a fork inherits it from the session
whose conversation it branches.

## Hooks

Claude Code is the only provider that says what a session is waiting for
(`statusRung` in `native/cmd/lich-native/statusrung.go`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). Its `Notification` carries a `message` written for a
human, so the card reads "Claude needs your permission to use Bash".

A `/resume` typed inside Claude Code reports the new conversation at the switch (measured 2026-10-17), so the
two-cards-one-conversation warning arrives at once.

## Transcript & cost

The full readout: per-turn token counts, and the window its mod reports, else one named from the model
(`windowForModel`).

A conversation lich spawned is told from a plain `claude` run by the name lich spawns it under
(`relay.IsRosterName` on the first `custom-title`), which covers the conversations whose rows were deleted before
the tombstones of schema version 8: measured on a real workspace, it marked 490 of the 539 Claude conversations
lich held no row for, and the 49 left had no title at all, which is a plain `claude` run.

## Plugin install

Driven through Claude Code's own plugin CLI (`internal/agentplugin`), so lich writes none of the files itself
and does not track the installed version in a marker line of its own.

## MCP

The session is handed lich's own MCP server on its command line (`--mcp-config`,
`providers.AcceptsMCPServer`), so a tool added here is in that session's list on the next spawn.

## Account

The plan gauge names the account a session spends (`internal/quota`, `Plan.Account`): lich answers Claude
Code's profile route with the credentials token. The usage route is the one Claude Code's own CLI polls.

A long-lived OAuth token (`claude setup-token`) carries `user:inference` alone, so the usage route answers it 403
and the gauge sends one `max_tokens: 1` message for the rate-limit headers on the response
(`internal/quota/claude.go`). That request carries Claude Code's system prompt verbatim because the API rejects an
OAuth token without it — the same coupling as the user agent, and it fails closed, as a failed reading. Headers
carry the two account-wide windows and no plan name, so such a session shows no model-scoped weekly cap and no
"Max 5x" badge.

## Profiles

A profile is a `CLAUDE_CONFIG_DIR` of its own with a real `claude auth login` (`internal/claudeprofile`). The
login the dialog starts is `claude auth login --claudeai`, which opens the browser itself and takes the callback
on a localhost port of its own (measured on 2.1.296); the address it prints, which redirects to a page that shows
a code, and the field that takes the code are for a browser that cannot reach this machine.

Shared through links (`internal/claudeprofile/profile.go`, `internal/profilestore/layout.go`): `settings.json`,
`CLAUDE.md`, `agents/`, `commands/`, `skills/`, `hooks/` and the marketplace clones (`plugins/marketplaces`) point
at Personal's, and Claude Code writes through a link to its target (measured on 2.1.296). The plugin set itself
(`installed_plugins.json`, `known_marketplaces.json`) is per profile, because an organization account may forbid
plugins Personal runs. An entry Personal gains after a profile was made is linked at that profile's next spawn. On
Windows the directories are junctions, but a file link needs Developer Mode: without it `settings.json` and
`CLAUDE.md` are copies, named on the profile (`copied`), and they drift from Personal's on the first edit to
either. A hard link was measured and drifts just the same, Claude Code replacing the file on write. The global
`.claude.json` is a copy everywhere, minus the account and its caches, since Claude Code replaces it through a
rename that would turn a link into a file anyway.

Trust in a profile: the copied `.claude.json` freezes the trust answers at its birth, so before each spawn into a
profile lich writes `hasTrustDialogAccepted` for the checkout's top when Personal trusts it or the main checkout
of the worktree it is. A folder outside any git checkout is not carried into a profile: lich asks git, not the
parents. The write is a read, an edit and a rename beside sessions that rewrite the same file; Claude Code
re-reads it before each write of its own (a key written mid-session survived its later writes), so the window
left is what either side writes between the other's read and rename.

The plugin cache (`internal/claudeprofile/plugincache.go`): before every spawn, and in the startup pass of the
lich plugin, each version a profile's `installed_plugins.json` names under its `plugins/cache` is moved into
`plugin_cache/` beside the profiles (or gives way to the copy already there) and replaced by a link to it. Claude
Code's orphan sweep, run at every session start, marks each cached version its own record does not name
(`.orphaned_at`) and deletes it once the mark is 14 days old, but it passes over a version that is a link, marking
and deleting nothing through it, and `plugin update`, `uninstall` and a reinstall over a link leave the store's
copy alone (all measured on 2.1.299, a plugin running from a linked version included). A version Claude Code
downloads stays a plain directory of the profile, which its own record protects, until the next spawn links it.
Personal's cache is Personal's alone and is left as it is, so a version both use is on disk twice. A profile made
when the whole cache was a link to Personal's is moved at its next spawn or lich start: each version it names is
copied from Personal's into the store, and one Personal's sweep already deleted stays missing. `.in_use` markers
of every profile running a version land in the store's copy, which a confined session reaches because the store
is bound read-write beside the profile (`internal/terminal/sandbox.go`). On Windows each version is a junction,
which Claude Code's sweep reads as a link too (read in the source, not run on Windows), and a version Windows will
not move while a file in it is open stays the profile's own until a later spawn. `CLAUDE_CODE_PLUGIN_CACHE_DIR`
moves Claude Code's whole `plugins/` directory, installed set, marketplaces and cache, not only the cache
(measured on 2.1.300), so a profile's spawn, login and plugin install drop it and a profile's plugins stay inside
its own directory; Personal keeps it, and lich reads Personal's installed set where it points
(`claudeprofile.PluginsDir`). A new profile seeds its marketplaces from, and links its clones to, Personal's
plugins directory where the variable points. A profile made before the variable was set keeps the
`plugins/marketplaces` link it was made with, and clones each marketplace again into a directory Personal no
longer reads; removing that link has the next spawn link it where the variable points.

The lich plugin follows Personal's (`internal/agentplugin/profiles.go`): `claude plugin install` installs into a
logged-out home (measured on 2.1.296), so every profile gets it, logged in or not. The pass lays each profile out
as a spawn would before asking its version (`EnsureShared`), which also moves a profile's plugin cache off
Personal's at a start. A profile's `plugin marketplace remove` deletes the marketplace clone it links to
Personal's (the plugin cache stays, measured on 2.1.296).

Continue in (`claudeprofile.MoveConversation`, `terminal.ContinueIn`) moves what Claude Code keys by the
conversation id: the transcript, its subagent directory, any file a hook filed beside it under the same name,
and `file-history/`, `session-env/` and `tasks/` under that id, the set a throwaway conversation left on 2.1.296,
which resumed with its history and checkpoints both ways. The prompt history (`history.jsonl`) and the project's
`lastSessionId` stay in the profile it left, so the arrow-up history does not follow. An async hook outlives the
Claude Code it ran for (its SessionEnd hook runs in a session of its own, measured on 2.1.304), so lich cannot
wait for it: a hook can file its entry again in the profile the conversation just left, and moving back replaces
that leftover instead of refusing. A move across filesystems copies, Windows volumes included. A card that has not
had its first prompt has no transcript yet, so it is not moved but started afresh in the target profile.

## Sandbox

Nothing provider-specific.

## Known limits

Unfold long pastes pastes twice wherever lich predicts the fold, because a repeat of the same text unfolds the
`[Pasted text #N +M lines]` chip. The prediction is Claude Code's rule copied from its 2.1.296 bundle (more than
800 characters, or more than `min(rows - 10, 2)` newlines, up to 100 000), and it assumes the paste goes to the
prompt.

A theme switch reaches it: Claude Code turns on mode 2031 (driven in a PTY).
