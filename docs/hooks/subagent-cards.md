# Contract: subagent cards

Turns a provider's own subagent call into a lich session. When the agent calls
its native subagent tool (Claude Code's `Agent`, Codex's `spawn_agent`,
opencode's `task`, ...), the plugin's pre-tool hook asks lich first. lich opens
the task as a session of its own, a card the user can watch and take over, and
answers with a veto; the hook turns that into the provider's own refusal, so the
native subagent never runs and the model reads why. The worker's report comes
back later as a `worker-finished` envelope ([notification.md](notification.md)).

It replaces the Claude Code mod's agent-cards (`hooks/agent-cards.js`) and works
the same on every provider whose hooks run, in the chat and in the terminal
alike, since a hook fires before the tool whatever drives the session.

See [README.md](README.md) for the shared transport and the client rules; this
hook is the one exception to "never block", below.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/subagent?token=${LICH_TOKEN}
Content-Type: application/json
X-Lich-Plugin: <plugin release>

{"session_id": "<LICH_SESSION_ID>",
 "provider_session_id": "<the conversation that called the tool>",
 "nested": false,
 "tool": "spawn_agent",
 "tool_use_id": "exec-2cb201ef-3cca-4785-8c49-cc0de8ca01db",
 "tasks": [{"title": "List the files", "prompt": "List the files in the current workspace...",
            "type": "explorer"}],
 "model": "",
 "isolation": "",
 "background": false}
```

- `session_id`: required, `LICH_SESSION_ID`.
- `provider_session_id`: the provider's id for the conversation that fired the
  hook, as in [session-state.md](session-state.md). lich allows a call from any
  conversation other than the one bound to the session.
- `nested`: `true` when the call comes from inside a native subagent rather than
  from the session's own conversation (Claude Code's `agent_id` on the payload,
  a child `sessionID` on opencode, a child `conversationId` on Antigravity). lich
  allows those: a native subagent it did not convert has no card to hang a child
  from.
- `tool`: the provider's tool name, as the hook received it.
- `tool_use_id`: the provider's id for this call, empty when it gives none.
- `tasks`: one entry per subagent the call starts, never empty. Most tools start
  one; oh-my-pi's `task` and Kiro's `use_subagent` take a list, and each entry
  becomes a session. `title` is the short name the call gives (empty when it
  gives none), `prompt` the task, `type` the subagent type (empty when the tool
  has none).
- `model`: the model the call asked for (Claude Code's `model`), empty for the
  provider's default. The worker runs on the asker's provider, so this is that
  provider's spelling, handed to `spawn.OpenSubagent` as `Model`.
- `isolation`: `worktree` when the call asked for a checkout of its own (Claude
  Code's `isolation`), else empty.
- `background`: the call asked to run in the background (Claude Code's
  `run_in_background`).

The body is held to 64 KiB. A prompt over the 8 KB `send_to_session` takes is
refused as a whole (`allow`), so the native subagent runs it instead of a card
running half of it.

## Response

`200` with a decision, always JSON:

```
{"decision": "deny",
 "reason": "lich opened this task as the session \"Fix #412\" (ticket 4b9bc94e). Its report reaches you as a <lich-notification kind=\"worker-finished\">. Do not retry the tool and do not do the task yourself.",
 "sessions": [{"id": "c41f", "label": "Fix #412", "ticket": "4b9bc94e"}]}
```

```
{"decision": "allow"}
```

- `deny`: lich opened one session per task, handed each its task and holds an
  errand for each. `reason` is the text the model reads, written by lich in the
  session's prompt language; the client passes it on untouched. `sessions`
  names what was opened, for logs.
- `allow`: lich did nothing, and the native subagent runs. lich allows a call
  from a nested conversation, a session already as deep as cards nest
  (`maxSubagentDepth`), a type the provider's row below keeps native, a session
  whose user turned subagent cards off, and any call it cannot open in time.

Other responses: `401` invalid token · `400` invalid body · `405` not a POST.
The client reads every one of them, and any failure to get an answer, as
`allow`.

### Timing

lich answers once the sessions exist (their records and labels), not once their
agents are up: the task is handed over by the relay when each agent is ready,
as `open_session` with a prompt does today. The client waits 10 seconds. If the
client's request ends before lich has written its answer (a timeout, the hook
killed), lich closes the sessions it opened for that request, so a card and a
native subagent never do the same task.

### Repeats

`/subagent` is idempotent by `tool_use_id`: a second request for the same call
gets the same answer and opens nothing. A call with no `tool_use_id`, or a new
one carrying the same prompt from the same session while that prompt's worker
is still running (a model retrying after the veto, measured once on Cursor),
gets `deny` with the session already open.

## Client rules for this hook

The general rule says a hook never blocks the turn. This one blocks the tool
call it fires for until lich answers, because the answer is the decision:

- Missing env vars → `allow` at once (outside lich the plugin changes nothing).
- Any error, timeout or non-`200` → `allow`. lich being down never stops an
  agent from delegating.
- A `deny` becomes the provider's own veto with `reason` as its text, per the
  table below. The hook never writes text of its own.

### What the model and the window see

Before, the mod answered the `Agent` call with a success (`async_launched`) and
the model read "running in the background". A hook can only refuse, so the call
now ends as a tool **error** carrying `reason`, with the provider's own prefix
(table below), and a chat draws it as a failed call. lich knows every
`tool_use_id` it vetoed, so the chat view draws that call as "opened as session
X" with a link to the card instead of as a failure.

## Event → veto mapping

| Provider | Hook | Tool | Fields → `tasks` | Veto the client returns | The model reads |
|---|---|---|---|---|---|
| Claude Code | `PreToolUse` | `Agent` | `description` → title, `prompt`, `subagent_type` → type | `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":R}}` | `PreToolUse:Agent hook error: R` |
| Codex | `PreToolUse` | `spawn_agent` | `message` → prompt, `agent_type` → type | same JSON as Claude Code | `Tool call blocked by PreToolUse hook: R` |
| opencode | plugin `tool.execute.before` | `task` | `description`, `prompt`, `subagent_type` | `throw new Error(R)` | `R` |
| oh-my-pi | extension `tool_call` | `task` | each of `tasks[]`: `name` → title, `task` → prompt, `agent` → type; `context` prepended to each prompt | `return {block: true, reason: R}` | `R` |
| Antigravity | `PreToolUse` | `invoke_subagent` | `Prompt`, `TypeName` → type | stdout `{"decision":"deny","reason":R}` | `tool call denied by pre-tool hook: R` |
| Kiro CLI | `preToolUse` | `subagent` (v2, ACP) and `use_subagent` (classic chat, `InvokeSubagents` only) | `subagent`: `task` → prompt, first stage's `name` → title; `use_subagent`: each `subagents[].query` → prompt | exit 2, `R` on stderr | `PreToolHook blocked the tool execution: R` |
| Crush | `PreToolUse` (crushrc) | `agent` | `prompt` | exit 2, `R` on stderr | `Tool call blocked by hook. Reason: R` |
| Cursor CLI | the Claude Code install's `PreToolUse`, terminal only | `Task` | `description`, `prompt`, `subagent_type` | Claude Code's JSON, or `{"permission":"deny","user_message":R}` from a Cursor hook | `Task blocked by preToolUse hook: R` |

Measured on 2026-11-22, with cheap models and one or two samples per row:

| Provider | Version | What was measured |
|---|---|---|
| Claude Code | 2.1.330 | stream-json, in auto, default, plan and bypass modes |
| Codex | 0.170.0 | `codex exec` in bypass mode, and `codex app-server` |
| opencode | 1.19.4 | `opencode run` and `opencode acp` |
| oh-my-pi | 18.8.9 | `omp -p` and `omp acp` |
| Antigravity | 1.5.11 | print mode and ACP |
| Kiro CLI | 2.21.0 | classic chat (`use_subagent`) and ACP (`subagent`), one sample each |
| Crush | 0.100.0 | against a stand-in model: hook, veto and text, not a real model's behaviour |
| Cursor CLI | 2026.10.19 | `cursor-agent -p` only; its ACP runs no hooks |

Where measured, the veto lands before any permission question.

Types kept native (always `allow`): on Claude Code every `subagent_type` but
`general-purpose`, as the mod did. The other providers' rows have none yet.

## lich server side

- `/subagent` is a hook endpoint like `/hook`: the session's token, the same
  confinement (`internal/scope`).
- A `deny` opens each task with the path `lich open --subagent` takes
  (`spawn.OpenSubagent`): in the asker's checkout, or a worktree of its own for
  `isolation: worktree`, filed with the asker, `OriginSessionID` set. The
  provider is the asker's; the errand is a subagent errand
  (`relay.SendSubagent`), kept past `ticketTTL` while the worker runs.
- The task reaches the worker as a `task` envelope. A worker drawn as a chat
  gets `answer="last-message"`: its chat driver sees the turn end and answers
  the errand with the turn's final message. A worker in a terminal gets
  `answer="last-message"` when `terminal.LastTurnSaid` reads its provider (every
  one but Cursor) and lich answers at its `done` report; otherwise
  `answer="reply"` and the ticket.
- The worker's outcome reaches the asker as a `worker-*` envelope
  ([notification.md](notification.md)), in the asker's chat queue or typed.
- The parked-worker, depth and stop rules of today's subagent cards
  (ceilings.md, Only a Claude Code subagent becomes a lich card) carry over
  unchanged, minus everything that existed only because the mod did the
  answering.

## Versioning and fixtures

A new endpoint is a contract change: the plugin's minor moves, lich raises its
ceiling in `internal/agentplugin/compat.go`, and both sides test against
`fixtures/subagent-cards.jsonl` (requests accepted and refused, and each kind of
answer). A plugin older than this contract still carries the mod's agent-cards,
which converts Claude Code calls by the old route; the release carrying this
contract removes it, and lich moves its floor to that release, so the two
routes never run in one session.

## Retired with this contract

- The mod's agent-cards and worker answer ([mod-answer.md](mod-answer.md),
  `/mod/answer`).
- `LICH_SUBAGENT_DEPTH` and `LICH_SUBAGENT_CARDS` as the plugin's switches: lich
  decides, so a setting change needs no variable and reaches running sessions.

## Known ceilings

- **Cursor CLI's chat runs no hooks** (`cursor-agent acp`, measured with the
  folder trusted, neither Cursor's own hooks nor the Claude Code install's): its
  native subagent runs and shows as a row in the chat's subagent list, fed by
  the `cursor/task` request its ACP sends once the subagent has finished. It
  never becomes a card there.
- **Codex trusts a hook by its hash**: a release that changes the plugin's
  `PreToolUse` entry asks the user to trust it again before it runs; until then
  Codex subagents stay native.
- **oh-my-pi's ACP server takes no flags** and loads extensions only from
  `$PI_CODING_AGENT_DIR/extensions/` (`.js` or `.ts`), so the plugin's extension
  must be there for the chat.
- **Crush was measured against a stand-in model**: the hook, the veto and the
  text are measured; whether a real model then stops was not.
- **The reason is all that holds the model back**: a reason that does not say
  where the work went made Cursor's model retry the tool once. lich's text has
  to name the session and the envelope that will come.
