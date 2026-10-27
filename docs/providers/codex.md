# Codex

`codex` (`internal/providers.Registry`). Its config directory is `$CODEX_HOME`, else `~/.codex`, and it files a
rollout by the date it started: `sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<id>.jsonl`. The limits these sit
under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags`): Codex has
no per-spawn append flag, so the point the briefing makes exists only in lich's MCP instructions.

In a fresh worktree Codex asks whether the folder is trusted, and text typed then is lost to a second screen
(measured 2026-10-08). Codex (0.161.0) reports nothing before its first turn, not even at its prompt in a trusted
folder, so the start gate has only a quiet screen to read, a trust, hooks or init question included.

Trust is keyed by the main checkout's top, so its worktrees already count, and its profiles merge Personal's
trust (`codexprofile/config.go`). A confined session skips the question with
`-c projects={"<main checkout>"={trust_level="trusted"}}` (the dotted spelling still asks; measured on 0.144.5 and
0.162.1).

A misspelled effort level starts without a word. On resume both the model and the effort come back from the
conversation (0.154.0).

## Hooks

Codex's `PermissionRequest` carries only the thing being asked about (`tool_name`, `tool_input`) and no message
of its own, so a waiting card reads a bare `Bash` (`statusRung` in `native/cmd/lich-native/statusrung.go`; the
mapping table is in [`../hooks/session-state.md`](../hooks/session-state.md)). That says which card to open and
not what it will ask.

A resume typed inside Codex (0.162.1) is reported with its next prompt, so a turn may already have run in both
cards before the two-cards-one-conversation warning arrives.

## Transcript & cost

The full readout: the window is 95% of the rollout's default or configured `model_context_window`, and the
tokens a running `total_token_usage`. The running model the card's tooltip names is `turn_context.model`. The
cache line under the window reads `last_token_usage`: `cached_input_tokens` out of `input_tokens`, with
`cache_write_input_tokens` taken as outside it, the way the cost is priced. That last reading is not measured,
since every rollout seen so far files the write as zero.

Codex is the one provider lich prices itself. `internal/pricing/prices.json` bakes a fixed slice of OpenAI rates
beside the Claude ones, read from the same LiteLLM table the refresh (`internal/pricing/pricing.go`) reads.
`task pricing:refresh` reprices the slice from that table, and a monthly job (`.github/workflows/pricing.yml`)
runs it and opens a pull request when a rate moved — deliberately not a gate, since an upstream price change is
nobody's pull request to fix. What the job cannot notice is a model the floor never carried: that list is
hand-picked. A conversation that ran `/model` has no cost from that turn on (`codexCostScan.mixed`): the one
running total spans both models and the rollout never splits it, so no rate prices it — absent, the way an
unpriced line is absent, rather than a number billed at whichever model happened to go last.

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
user has left is worse than one under no name. The usage route is the one Codex's own CLI polls.

## Profiles

A profile is a `CODEX_HOME` of its own with its own `codex login` (`internal/codexprofile`). Codex keeps the
plugin set (`[plugins."<key>"]`, `[marketplaces.<name>]`) in the same `config.toml` as every other setting, and
reads no second user file it could be split into: a `managed_config.toml` in `CODEX_HOME` is ignored, the other
layers are `/etc/codex` and a project's `.codex/config.toml`, and `-c` would hide the plugin set from Codex's own
writes (measured on 0.162.1). So a profile's `config.toml` is a file of its own, and before every spawn lich
writes it again (`config.go`) as Personal's tables minus Personal's plugin set, then the profile's plugin set,
then the project trust (`[projects.*]`), hook trust (`[hooks.state.*]`) and dismissed notices (`[notice]`) the
profile was given and Personal was not. The cut is by table header and copies tables as they are written; plugin
or marketplace keys Personal writes as dotted keys above its first table (Codex never does) are every profile's.
The plugin and marketplace downloads (`plugins/cache`, `.tmp/marketplaces`) are the profile's own too, which
costs a clone per profile and is what lets `plugin remove` and the lich plugin's pinned reinstall (`plugin
marketplace remove` then `add`) run there without deleting Personal's copy; a profile made when they were links
has them removed at its next spawn. Plugins are ported with `codex plugin add` run as the profile, after
Personal's declaration of the marketplace, ref included, is copied into the profile's file and fetched with
`plugin marketplace upgrade`; a marketplace Personal does not declare is left to Codex. Per profile besides:
`auth.json`, `sessions/`, `history.jsonl`, `session_index.jsonl`, the SQLite state and `installation_id`; linked
from Personal: `hooks.json`, `AGENTS.md`, `skills/` and `rules/`. `CODEX_SQLITE_HOME`, `CODEX_API_KEY`,
`CODEX_ACCESS_TOKEN` and `OPENAI_API_KEY` are dropped from a profile's spawn: the first would put every profile's
thread database in one place and the rest would override the login. A `cli_auth_credentials_store` of `keyring`
in Personal's `config.toml` sends the login to the OS keyring, whose entry per `CODEX_HOME` was not measured. On
Windows `hooks.json` and `AGENTS.md` are copies without Developer Mode, as for Claude Code. Every reader of
rollouts (resume, usage, search, the History tab) scans all of them and resumes under the profile that holds the
rollout.

The lich plugin follows Personal's (`internal/agentplugin/profiles.go`): `codex plugin add` installs into a
logged-out home (measured on 0.162.1), and a Codex profile's version costs one `codex plugin list` each. A Codex
profile made when `config.toml` was shared still links Personal's, so the pass, which lays each profile out as a
spawn would first (`EnsureShared`), would otherwise read it as holding Personal's plugin while its next spawn
rebuilt it with no plugin and no marketplace.

The login is a browser callback on a fixed port: `codex login` prints an address and waits for the page's
redirect on `localhost:1455` (measured on 0.162.1), so two logins cannot run at once, in lich or beside it, and a
browser that cannot reach this machine needs `codex login --device-auth` from a terminal, which is not the
command the dialog's "log in from a terminal" runs. The dialog decides whether the login took from `codex login
status`'s exit status, and reads "not logged in" in Codex's own words to tell a logged-out profile from a broken
`config.toml`. Its account and plan are the plan gauge's reading of that directory (`quota.CodexProfilePlan`, the
ChatGPT usage route, five minutes cached, the email claim of an id token that has not expired), so a profile
whose usage call fails is "Logged in" with no name.

Continue in (`codexprofile.MoveConversation`) moves the rollout under `sessions/<yyyy>/<mm>/<dd>/` and
`generated_images/<id>/`, and relies on Codex indexing it again. Codex's thread database keeps an absolute
`rollout_path` per thread, and a rollout alone in an empty home was found by id and written into that home's
database with its path rewritten (measured on 0.162.1, `thread/resume` and `exec resume`); the interactive TUI
resume of a moved conversation behind a real login was not exercised. The source's database row and
`history.jsonl` stay, so its own picker may list a thread whose rollout is gone.

## Sandbox

Nothing provider-specific.

## Known limits

A long paste folds by a constant with no setting and no key that unfolds it, so Unfold long pastes has no switch
here.

Codex reads the terminal's background once at boot and never hears of a theme switch.
