# Pi

`pi` (`internal/providers.Registry`), from the Pi coding agent project. This is the standalone Pi CLI, with provider id `pi`; it is separate from oh-my-pi (`omp`). The CLI keeps its agent state under `PI_CODING_AGENT_DIR`, defaulting to `~/.pi/agent`; `PI_CODING_AGENT_SESSION_DIR` can move session files separately. The integration was checked against Pi 1.5.2 on Linux. The limits these sit under are in [`../ceilings.md`](../ceilings.md).

## Spawn

lich runs Pi with `--model`, `--thinking` and `--append-system-prompt` for the settings and briefing Pi accepts. A resumed conversation uses `--session <id>`; a fork uses `--fork <id>`. These flags were read from Pi 1.5.2's `pi --help`.

Pi has no lich skip-permissions flag, so Settings › Providers has no such switch for Pi. Pi's own permission settings remain in force.

## Extension and MCP

The lich plugin install ships a Pi extension and passes its path with `--extension` on each Pi spawn. The extension reports lifecycle events and registers the `lich` MCP server when the session transport is available. Its directory is read-only inside a confined session. Removing the lich plugin or the extension file stops the integration on the next spawn.

The extension is shipped in the lich binary rather than fetched from `lichdotdev/lich-plugin`: its client code and the hook contracts live here because Pi's extension API is not a shell-hook format. The install version still follows the compatible lich-plugin release, and the extension reports that version with each hook request.

## Hooks

Pi reports session start from `session_start`, turn state from `agent_start` and `agent_settled`, tool activity from tool execution events, file changes after `write`, `edit` and `bash`, and an explicit session name from `session_info_changed`. It reports `ctx.model.id` at session start and each `model_select`, so the footer shows the model in use and updates when it changes inside Pi. Quitting the Pi process reports idle. Pi has no hook for a permission wait or compaction, so lich cannot show those states. It does not report an automatically generated conversation title.

Pi's session-start report arrives at session initialization, before the first prompt. It does not prove that any startup question has been answered, so lich does not hold typed work behind it.

## Resume and history

Pi stores one JSONL file per conversation under `<session-dir>/<encoded-cwd>/<timestamp>_<id>.jsonl`. The history prompt is offered only when that file exists under the session's cwd; `PI_CODING_AGENT_SESSION_DIR` overrides the default `<agent-dir>/sessions`, and `PI_CODING_AGENT_DIR` moves the agent directory. Relative overrides resolve from the Pi session's working directory. Lich does not yet index Pi conversations started outside lich, search Pi transcripts, or read its context and cost totals.

## Known limits

Pi supports model and thinking effort at spawn, resume, fork, MCP tools, lifecycle state, and the shared lich session tools. It has no integration yet for account profiles, quota, measured context or cost, in-app control, side questions, session-card subagents, or transcript history/search. Model selection is reported live, but persistence of an effort/model choice across a later lich resume is not verified.
