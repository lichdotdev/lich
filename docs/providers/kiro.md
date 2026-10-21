# Kiro CLI

`kiro-cli` (`internal/providers.Registry`), spawned as `kiro-cli chat`, since `--model`, `--effort` and `--trust-all-tools`
are the `chat` subcommand's while `--agent` and `--resume-id` are the root's. `~/.kiro` is `$KIRO_HOME`
else `$HOME/.kiro` (2.21.0, measured against a throwaway home), and holds the agents, the settings and
the conversations; the login token is a row in the SQLite store under `$XDG_DATA_HOME/kiro-cli/data.sqlite3`. A
conversation's metadata is `~/.kiro/sessions/cli/<id>.json` with its turns in the `.jsonl` beside it. The limits
these sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

**A lich-spawned Kiro session runs lich's agent, not the user's** (`internal/agentplugin/kiro.go`,
`internal/terminal/command.go`, `agentArgs`). Kiro keeps its hooks inside an *agent config*, and its built-in
`kiro_default` cannot be shadowed: a `kiro_default.json` in the agents directory is ignored outright (measured on
2.21.0). So the only place a hook can be registered is an agent lich writes itself, which the spawn then names
with `--agent lich`. Two things follow, and neither is visible on the card. The agent lich writes carries no
`prompt`, so a session loses the paragraph `kiro_default` adds about subagents, the planner and LSP: the model
still has every tool, it is just not told about those. And a default the user set with
`kiro-cli agent set-default` is **not** what a lich session runs; their own agent applies to Kiro started from a
terminal and not to Kiro started from a card. Deleting `~/.kiro/agents/lich.json` puts the session back on `kiro_default` and
silently ends every report with it: the spawn stops naming an agent it cannot find, which is the one case where
that costs a warning line rather than the reports.

Kiro's skip-permissions flag still stops once (`internal/terminal/command.go`, `skipPermissionFlags`). With the
switch on, lich passes `--trust-all-tools`, and Kiro's TUI opens on a full-screen confirmation ("No, exit / Yes,
I accept / Yes, and don't ask again") that has to be answered before the session starts. The flag is real and
every tool afterwards runs unconfirmed; it is the *first* screen that is not skipped, which is the one thing a
user who ticked the box does not expect. Kiro's own `chat.disableTrustAllConfirmation` setting turns it off for
good, and lich does not write it: that setting disables a safety confirmation for every Kiro on the machine,
including the ones lich never spawned.

lich appends nothing to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags` has no entry
for Kiro), so the point the briefing makes exists only in lich's MCP instructions.

A misspelled effort level starts without a word. On resume the model comes back from the conversation (2.21.0);
the effort is unmeasured, since no model on the measuring account accepts one.

## Hooks

**Kiro enforces no hook timeout, and blocks the turn behind one** (`internal/agentplugin/kiro.go`): its TUI draws
"0 of 1 hooks finished" while a report runs, and a hook that slept four seconds ran to completion under both
`timeout_ms: 1000` and `timeout: 1` (2.21.0). So lich writes no timeout into the agent, a field that bounds
nothing being a promise the file does not keep, and what actually bounds a report is the request timeout inside
the script itself. A lich whose listener has gone away costs a Kiro turn that wait on every hook it fires, where
the other harnesses cut it off themselves.

**A Kiro session never reports `waiting` or `idle`** ([`../hooks/`](../hooks/)). Its five events are `agentSpawn`,
`userPromptSubmit`, `preToolUse`, `postToolUse` and `stop`, so lich closes session-start, session-state's
busy/tool/done rows and session-touched, and nothing else. A Kiro session sitting on a permission prompt reads as
`busy`, true, but it does not say what it is waiting for, and the card keeps the provider's mark until the PTY
itself goes, because there is no session-end event to clear it. That `busy` also holds the machine awake
(`internal/awake`, `turnLog.onOpen`): a Kiro session blocked on a human keeps it out of idle sleep until someone
answers.

An in-app `/chat` switch is reported at once (2.21.0: `agentSpawn` fires per switch), so the
two-cards-one-conversation warning arrives at the switch. A scripted run (`--no-interactive`, how an agent runs
one) carries no conversation id in any hook payload (measured on 2.21.0), so a nested `kiro-cli` run as a tool
inside another session still reports as its host (`internal/terminal/nested.go`).

## Transcript & cost

Context only. Kiro records `context_usage_percentage` against a `context_window_tokens`
(`internal/terminal/usage_kiro.go`), so the ring is real — but it files *every* token count as zero even on a
turn that spent them, so the tooltip's "n / window tokens" is **derived from the percentage**, not read. It is
Kiro's own percentage against Kiro's own window, and it is what keeps the tooltip agreeing with the ring; it is
not a token count anybody measured. Its spend never appears at all: `metering_usage` is denominated in
**credits**, whose dollar value depends on the account's plan, and lich's readout is dollars — so a Kiro session
shows a context ring and no cost, and the CLI's own footer is the only place that credit figure can be read.

Kiro writes `session_created_reason: "subagent"` on every conversation (2.21.0, measured on 15), so it is no
sub-agent filter and is not read.

## Plugin install

The hooks are the agent config named under Spawn, and there is nothing else to install: Kiro has no plugin
system, so the version lives in that file's `description` rather than in a marker line.

## MCP

Nothing provider-specific.

## Account

Kiro CLI carries no claim naming the account, measured on 2026-09-04 (`internal/quota`, `Plan.Account`): its
login is a row in `~/.local/share/kiro-cli/data.sqlite3` holding an opaque `aoa…` token, a `github` provider
label and an AWS profile ARN, nothing a person recognises as their account. Naming it would take a network call
against an unmeasured route, for a provider that has no gauge to hang the name under.

## Profiles

Linux only. Kiro (2.21.0, measured 2026-10-12 in throwaway homes) keeps its login in
`$XDG_DATA_HOME/kiro-cli/data.sqlite3` (`auth_kv`, beside its history and state tables; issue #766), and the TUI it
runs (`bun`, `node`, `tui.js`, a few hundred MB) in the same folder. `KIRO_HOME` moves `~/.kiro` (settings, agents,
steering, skills and the conversations, `sessions/cli/<uuid>.json` with the `.jsonl` beside it), which no profile
moves, so a conversation is every profile's. So a profile is a data home of its own (`XDG_DATA_HOME` pointed at
`<lich config dir>/profiles/kiro/<slug>/`, `internal/xdgprofile`, `internal/profilestore/linkhome.go`) made of links
to every entry of yours, `kiro-cli/` linked entry by entry so the TUI bundle is not downloaded again, except
`data.sqlite3` (with its `-wal`, `-shm` and `-journal`) and `.refresh.lock`, which are the profile's. `KIRO_API_KEY`
is dropped from a profile's spawn; the provider API-key variables are not.

`kiro-cli login` runs in a shell card (`xdgprofile.Service`), and nothing in lich sees it finish; the list is read
again when Settings opens and when the window regains focus after a minute. A logged-out `whoami --format json` is
`{"account":null}` with exit 1; the logged-in shape was not measured, so a profile reads as logged in whenever
`account` is not null and its address is `account.email` when there is one. Not measured: the logged-in TUI, an
enterprise (IdC) login that may read `~/.aws/sso/cache`, which no profile moves, and a TUI run from a linked bundle.
There is no plan per profile. Continue in moves nothing and only changes the session's profile, so a Kiro
conversation resumes in whichever profile its session is set to; one adopted from History runs in the project's.

Off Linux, read in the 2.32.0 macOS `.dmg` and Windows `.msi` on 2026-10-17 and never run: on macOS the `kiro-cli`
launcher keeps its tokens in the Keychain through `/usr/bin/security` (`kirocli:odic:token`,
`kirocli:odic:device-registration`, `kirocli:external-idp:token`, fixed names), so it has no profiles on macOS. On
Windows the login is in `data.sqlite3` under the local AppData folder, found through `SHGetKnownFolderPath`, which
no variable moves (the binary reads neither `LOCALAPPDATA` nor `APPDATA`), so it has no profiles on Windows either.
`KIRO_DATA_DIR` and `KIRO_HOME` leave `data.sqlite3` where it was (run on the 2.32.0 Linux build in a throwaway
home); `KIRO_TEST_DB_PATH` moves it but is a test hook, which lich does not lean on. Kiro's docs announce sign-ins
under `KIRO_HOME` "in V3", which would give it profiles on every platform with a variable of its own.

## Sandbox

Nothing provider-specific.

## Known limits

Unfold long pastes presses Tab behind a paste of more than 10 lines or 500 characters (2.21.0), which expands the
newest `N lines ▸` chip.

Kiro reads the terminal's background once at boot and never hears of a theme switch.
