# Contract: session touched

Signals that a session likely changed files on disk, so lich refreshes that
session's git status **immediately** instead of waiting for its steady poll. It
is a latency optimization, not a source of truth: lich polls git every ~3s
regardless, so a user without the plugin sees the same diff badge — just up to a
poll interval later.

It also tells the agent when a file it just wrote was written minutes earlier by
another lich session in the same checkout, the note lich-plugin's edit-guard mod
added on Claude Code alone (see Edit overlap).

See [README.md](README.md) for the shared transport (`LICH_PORT` / `LICH_TOKEN`
/ `LICH_SESSION_ID`) and the client rules every hook follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/session-touched?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id": "<LICH_SESSION_ID>",
 "paths": ["/abs/path/one.go", "/abs/path/two.go"],
 "hold": false}
```

- `session_id`: required, `LICH_SESSION_ID`.
- `paths`: optional, the files the tool wrote, each an absolute path. The client
  resolves a relative one against the hook's working directory (Codex's patch
  headers, Kiro's `tool_input.path`) and sends each file once. lich compares the
  strings as sent, so the same file must be spelled the same way by every
  client: absolute, with no `.` or `..` left in it. A tool that names no file
  (`Bash`, `shell`, `run_command`) sends no `paths`, and the request is only the
  git-status refresh it always was.
- `hold`: optional, `true` from a provider that cannot show text from the hook
  reporting the edit (Antigravity, Kiro). lich keeps the note instead of
  answering it, for the `collect` below.
- `provider_session_id` and `nested`, as in
  [subagent-cards.md](subagent-cards.md), may be sent and are read by no one: a
  nested CLI or a native subagent edits the same checkout as its card, so its
  edit is the card's.

A second shape takes a held note, from the hook that runs before the model is
called next:

```
{"session_id": "<LICH_SESSION_ID>", "collect": true}
```

A `collect` carries nothing else, and refreshes nothing: on Antigravity it runs
before every model call, and a git fetch each time would cost more than the
poll this hook beats.

Responses:

- `204`: nothing to say (a held touch always).
- `200` with `{"note": "<lich-notification ...>"}`: one or more edit-overlap
  envelopes ([notification.md](notification.md)), to be handed to the model the
  way the provider takes a hook's extra context (Delivery, below).
- `401` invalid token · `403` the token belongs to another session · `400`
  invalid body (an empty path, or a `collect` with `paths` or `hold`).

The body is bounded to 64 KiB, room for the paths of one large patch.

Both sides test against the payloads in
[`fixtures/session-touched.jsonl`](fixtures/session-touched.jsonl).

## Edit overlap

lich keeps, for every path a touch names, the last session that wrote it and
when. A path is absolute, so it names its checkout too: two worktrees never
share one.

- A touch from session A on a path session B wrote within the last **10
  minutes** answers A with a note naming B. B is not told; B hears of it if B
  writes that path again, since A is then the last writer.
- The window is the one lich-plugin's edit-guard mod used: it spans the turn in
  which the other session made its edit, and past it that work is likely
  finished and the file as read is the news.
- B must still be running (a live terminal or chat); an edit by a closed
  session says nothing. A's own earlier edits never do.
- The note is one `notice` envelope per other session, `topic="edit-overlap"`,
  `from` and `session` naming B, its `<summary>` naming the paths and how long
  ago B wrote the newest of them, and how to reach B: `send_to_session` where A
  has lich's tools, `"$LICH_BIN" send` where it does not. Several envelopes are
  one blank line apart, in the order the overlaps happened.
- The text is in the prompt language (`LICH_PROMPT_LANG`), the tag and its
  attributes in English.
- A held overlap waits for A's next `collect`, or for A's next touch that does
  not hold. It is told once, and dropped unsaid when B's edit leaves the window
  first or B stops running.

## Event → action mapping

| Provider    | Hook                                                         | Paths from                                                       | Note delivered by                                                |
|-------------|--------------------------------------------------------------|------------------------------------------------------------------|------------------------------------------------------------------|
| Claude Code | `PostToolUse` (`Edit`, `Write`, `NotebookEdit`, `Bash`)      | `tool_input.file_path` (absolute; `NotebookEdit` not measured) | `hookSpecificOutput.additionalContext`                         |
| Codex       | `PostToolUse` (`apply_patch`, `Bash`)                        | the patch's `*** Add File:`, `*** Update File:`, `*** Delete File:` and `*** Move to:` headers, relative to `cwd` | `hookSpecificOutput.additionalContext`              |
| Antigravity | `PostToolUse` (`write_file`, `edit_file`, `run_command`) with `hold` | `toolCall.args.Path` (absolute)                          | `PreInvocation`, a `collect`, as the `injectSteps` system message |
| opencode    | `tool.execute.after` (every tool)                            | `output.metadata.files[].filePath` (absolute), measured on `apply_patch` only | appended to `output.output`             |
| oh-my-pi    | `tool_result` (`write`, `edit`, `bash`)                      | `details.resolvedPath` (absolute)                                | the handler's `{additionalContext}`                              |
| Crush       | `PostToolUse` (`edit`, `write`, `multiedit`, `bash`), from Crush v0.99.0 | `tool_input.file_path` (absolute)                    | `{"version": 1, "context": "<note>"}` on stdout                  |
| Cursor CLI  | the Claude Code install's `PostToolUse`                      | as Claude Code                                                   | as Claude Code                                                   |
| Kiro CLI    | `postToolUse` (`write`, `shell`) with `hold`                 | `tool_input.path`, relative to `cwd`                             | `userPromptSubmit`, a `collect`, printed on stdout               |

The action is the same everywhere: refresh the session's git status, then
answer the note.

Fire it **only for tools that write to disk**, matching the provider's own
names. Do **not** fire on read-only tools (`Read`, `Grep`, `Glob`, Crush's
`view`): a git-status refresh per read would cost more than the poll it is
meant to beat. The tool name is on the hook's stdin payload if a single script
filters instead of per-tool matchers.

**Antigravity renamed its write tools.** 1.5.15 writes with `write_file` and
`edit_file` (measured, `toolCall.name`), where 1.1.19 used `write_to_file` and
`replace_file_content`; a matcher naming only the old pair matches nothing on a
current Antigravity, silently, so the registration names both pairs. Its
arguments are PascalCase (`Path`, `CommandLine`). Antigravity's tool names are
not derivable from its own documentation: that documentation says a name is the
step type lower-cased with `CORTEX_STEP_TYPE_` removed, and the binary's step
enum names `MCP_TOOL` where a real payload carries `call_mcp_tool`, so the names
are read off a real payload, never off the docs. Its `PostToolUse` output is not
read (an `{}` and the undocumented `overwriteResult` both left the result as it
was), which is why the note is held there and `PreInvocation`, which fires
before every model call and whose `injectSteps` system message the model reads,
collects it.

**Kiro's write tools are `write` and `shell`**, and its matchers are registered
as those two bare words rather than as one pattern. `*` matched every tool on a
real 2.21.0 run and `*` is not a valid regular expression, so its matchers are
glob-like rather than regex, and a bare literal is the one spelling that names
the same tool under glob, regex, exact or substring alike. An anchored
alternation copied from Crush would match nothing there, silently. Those are
the **TUI's** names, which is the engine lich spawns. The same two tools are
`fs_write` and `execute_bash` under `--no-interactive`, and `fs_read`,
`fs_write` and `execute_bash` are the names Kiro's own `--trust-tools` help
prints, so the help is not the place to read them from, the payload is. Kiro
shows the model nothing a `postToolUse` hook prints, on stdout or with exit 2,
so the note is held and the next `userPromptSubmit`, whose stdout Kiro adds to
the prompt, collects it.

opencode moves from `file.edited` to `tool.execute.after`: `file.edited` has no
way to hand text back, and the tool's own output does. `metadata.files` was
measured on `apply_patch` (1.19.4, GPT models), which is the tool opencode
writes with there; `write` and `edit`, which it uses with other models, were
not measured, so `file.edited` stays the refresh until they are, and a client
does not send the same edit twice. oh-my-pi filters on the result rather than the call so the
refresh sees the tree after the write; its `python` tool is left out on
purpose, being a general evaluator that would fire a refresh per computation.

## Registration

lich writes the registration itself for Crush (`crushrc`) and Kiro (its agent
file), and fetches Antigravity's `hooks.json` and scripts from the plugin
release (`internal/agentplugin`). From lich-plugin 0.22.0
(`agentplugin.EditNoteRelease`):

- **Crush**: the touched hook rides `PostToolUse` on a Crush that has it
  (v0.99.0); on an older Crush, or one whose version lich cannot read, it stays
  on `PreToolUse` and sends no `paths`, as before.
- **Kiro**: the `postToolUse` entries run `report-touched.sh hold`, and a
  `userPromptSubmit` entry runs `report-touched.sh collect`.
- **Antigravity**: the plugin's own `hooks.json` registers `PostToolUse` with
  `report-touched.sh hold` and `PreInvocation` with `report-touched.sh
  collect`; the script list lich fetches does not change.

An older plugin release registers what it always did and sends no `paths`, so
it gets the refresh and never a note.

## lich server side

- **Endpoint**: `internal/terminal/touched.go`, `transport.sessionTouched`:
  validates the token and body (`parseSessionTouched`), forwards the session id
  for the refresh unless the request only collects, and answers with the note
  the relay hands back.
- **UI push**: `internal/terminal/terminal.go`: emits the global app event
  `session-touched` (`{id}`).
- **Refresh**: `sessionTouched` (`native/cmd/lich-native/model_events.go`): resolves
  the session id to the path its card watches (its worktree, else the project path)
  and calls `refreshPath` (`native/cmd/lich-native/gitstate.go`), which fetches
  that path now, ahead of the poll tick. A no-op when the session lives in a
  project not on screen, so it costs no git call.
- **Edit overlap**: `internal/relay/editnote.go`, `Service.Touched`: who last
  wrote each path, the held overlaps, and the envelope.

## Known ceilings

- **Poll stays the baseline.** This never replaces the ~3s poll (see the git
  status ceiling in `docs/ceilings.md`); it only front-runs it. Changes from
  outside an agent (a shell session, an external editor) are still caught by the
  poll, not this hook.
- **No debounce.** A burst of edits fires one POST per tool, each an immediate
  git fetch. Cheap and idempotent (unchanged status short-circuits re-renders),
  but a trailing debounce could collapse bursts if it ever matters.
- **Session id, not path, for the refresh.** The hook sends the session id and
  lich/the window resolve the path, so a `cd` inside a Bash tool can't point the
  refresh at the wrong repository.
- **The edits live in memory.** A restart of lich forgets every edit and every
  held note: the first edit after one tells nobody, for up to the window.
- **A shell edit is not seen.** `sed -i`, a formatter or a script run through
  `Bash` names no path, so neither its writer is recorded nor its overlap told.
- **One writer per path.** Two other sessions writing the same file leave only
  the newer one, so A hears of one of them.
- **On Kiro the note waits for the next prompt.** It is read when the person
  (or lich) next submits one, possibly long after the edit, and is dropped if
  B's edit left the window by then.
- **On a Crush older than v0.99.0 the refresh runs one tool early and there is
  no note.** `PreToolUse` is its only event, so the report fires before the
  write lands and the immediate fetch sees the tree as it was. A Crush updated
  past v0.99.0 after the install keeps that registration until lich rewrites
  `crushrc` (an update, a reinstall or a repair).
