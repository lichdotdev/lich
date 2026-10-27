# oh-my-pi

`omp` (`internal/providers.Registry`). Its state directory answers to two variables and the profile wins:
`OMP_PROFILE` moves the whole directory to `~/.omp/profiles/<profile>/agent` and beats an explicit
`PI_CODING_AGENT_DIR`, which otherwise names it, falling back to `~/.omp/agent`. One JSONL holds a conversation,
in a directory named after the cwd it ran in: `sessions/<encoded cwd>/<timestamp>_<id>.jsonl`. The limits these
sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich appends to the agent's system prompt (`internal/terminal/command.go`, `briefingFlags` →
`relay.SpawnBriefing`): a session is spawned with `--append-system-prompt` carrying lich's own briefing, so text
the user never wrote is in every session's prompt and in `/proc/<pid>/cmdline`.

A misspelled effort level starts without a word. On resume both the model and the effort come back from the
conversation (18.0.10).

## Hooks

An oh-my-pi card sends no reason with a `waiting` and keeps the generic "Waiting on you"
(`statusRung` in `native/cmd/lich-native/statusrung.go`; the mapping table is in
[`../hooks/session-state.md`](../hooks/session-state.md)). It does not report `waiting` in the first place: it
declares an approval event no run was ever seen emitting, so there is nothing to hang a reason on. A bare card
there means the harness never spoke, not that the block is trivial.

A `/resume` inside omp (18.8.9) raises `session_switch`, which the plugin reports from 0.20.5 on, so the
two-cards-one-conversation warning arrives at the switch.

## Transcript & cost

Cost only: a USD total on every assistant turn, and no line carries a context window, so the footer shows the
figure and no ring. The cost tooltip also carries the newest assistant turn's cache line, from its
`usage.input`, `cacheRead` and `cacheWrite`: omp takes the cache out of `input` before writing it
(`calculateOpenAIUsageAccounting`), so the three add up to the turn's input. The figure is omp's own arithmetic, never re-priced here: the sum of its assistant turns, the
same walk its own status line makes, so a `task` sub-agent's spend is missing from lich's figure exactly as it is
from omp's. A turn written with no total at all withholds the session's whole number and borrows the
`unpriced-model` marker, whose tooltip then blames a network that would not have helped. That branch has never
been seen in the wild and gets no reason of its own until it is. omp writes a model on every assistant turn, but
its arm reads cost only, so the card's tooltip names no running model.

## Plugin install

oh-my-pi has no plugin CLI, so lich writes the released files itself (`internal/agentplugin`). omp records
nothing about what is installed, so the version lives in a marker line lich wrote: edit the file by hand and lich
reads it as not installed.

Both halves land in the state directory above, resolved by one rule that the install and the transcript reader
share through `internal/ompprofile` (`internal/agentplugin/omp.go`, `internal/terminal/transcript.go`; the
sandbox keeps a copy of its own, `internal/sandbox`). Get the precedence backwards and the install lands where
omp is not reading and every restored card silently starts fresh.

## MCP

omp's `mcp.json` registers lich's MCP server by the absolute path of the binary that installed it, and it is a
JSON document lich rewrites rather than appends to: every key survives, the user's formatting does not.

## Account

oh-my-pi runs on the user's own API keys, so there is no plan account to name (`internal/quota`, `Plan.Account`),
which is the same reason it has no gauge.

## Profiles

A profile is omp's own (`internal/ompprofile`): `OMP_PROFILE=<slug>` puts everything omp keeps for that profile
under `~/.omp/profiles/<slug>/` (`agent/` with `agent.db`, `sessions/`, `blobs/` and its settings; beside it
`logs/`, `run/` and `plugins/`), which is omp's location and not lich's: lich's own part is a name in a sidecar
under its config dir. A profile made with `omp --profile x` outside lich is not listed, but its slug is taken, and
`default` is reserved because omp reads `OMP_PROFILE=default` as the default profile. A profile links Personal's
`config.yml`, `models.yml`, `mcp.json`, `ssh.json`, `AGENTS.md`, `SYSTEM.md`, `RULES.md`, `PERSONALITY.md` and the
`agents`, `commands`, `extensions`, `hooks`, `modules`, `prompts`, `rules`, `skills`, `themes` and `tools`
directories (measured on 18.8.9: a linked `skills/` resolves in the profile and an unlinked profile sees none, and
omp writes `config.yml` through the link and leaves it in place), so the lich extension and MCP registration
written into Personal's agent directory are the profile's too, and oh-my-pi needs no plugin pass per profile. Not
linked: `agent.db` (the logins), `sessions/`, `blobs/`, omp's databases (`models.db` is about 7 MB and is built
again by each profile) and the plugins `omp plugin install` puts beside `agent/`, so an omp plugin is per profile,
as Codex's are. `~/.omp/install-id`, the global daemons and the collab registry are shared by every profile
whatever the variable says (read in the source). Personal is read as `omp` reads it with lich's environment: the
profile `OMP_PROFILE` names there, else `PI_CODING_AGENT_DIR`, else `~/.omp/agent`; a profile ignores
`PI_CODING_AGENT_DIR`. `PI_CONFIG_DIR` and the `XDG_*_HOME/omp` layout `omp config init-xdg` makes are not read,
here or by the provider's plugin install and transcript reader, which share the rule. On Windows the file links
are copies without Developer Mode, as for Claude Code.

`omp login` is an interactive picker of providers (`omp login <provider>` takes an id, and each provider's own
OAuth flow follows), so the dialog hands the command to a shell card and ends (`ompprofile.LoginCommand`,
`ompprofile.Service`): nothing in lich sees that login finish, so nothing is emitted for it, and the profile list
is read again when Settings opens and when the window regains focus after a minute. A profile is "logged in" when
`omp usage accounts --json` lists an OAuth account in it (measured on 18.8.9: local, exit 0 with an empty list
when there is none, no network); a login that is only an API key, stored or in the environment, reads as logged
out. One profile holds several accounts, one per provider or more, so the row names every address (the `email:`
part of each identity key) and each provider rather than one plan, and the Continue in rows carry no usage.
`OMP_AUTH_BROKER_URL` and `OMP_AUTH_BROKER_TOKEN` are dropped from a profile's spawn, because a broker named there
is read before the profile's own `agent.db`; a broker in the shared `config.yml` (`auth.broker.url`) or in
`~/.omp/auth-broker.token` is every profile's too, and so is a key a shared `models.yml` sets for a provider,
which omp reads before any OAuth login. The provider API-key variables in lich's environment are not dropped,
because omp's own order puts a stored OAuth login first and the environment key only after it (`KeyCascade.peek`,
read in the source).

Continue in (`ompprofile.MoveConversation`) moves the session file and its artifacts, and copies its blobs: resume
looks only in the active profile's own `sessions/`, so a conversation outside it is `Session "<id>" not found`
(measured), and moving the file is what makes it resumable in the other profile, both ways. What moves is
`sessions/<cwd>/<timestamp>_<id>.jsonl` and the directory beside it named after the file (the sub-agents' sessions
and tool output), and every `blob:sha256:` image the files refer to is copied into the target's `blobs/`, copied
and not moved because other sessions of the source may refer to it. A blob the source no longer has is skipped.
The prompt history (`history.db`) stays in the profile it left. Every reader of sessions (resume, usage, search,
the History tab) scans all profiles and resumes under the one that holds the session.

## Sandbox

A confined oh-my-pi is spawned with `OMP_SKIP_SETUP=1`: a profile's `config.yml` links into Personal, which the
sandbox does not mount, and without it the session opened on the setup wizard.

## Known limits

oh-my-pi asks for mode 2031 in its shipped code, so a theme switch should reach it (not driven).
