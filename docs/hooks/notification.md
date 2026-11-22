# Contract: notification envelope

Everything lich hands an agent arrives in one envelope, the same on all eight
providers: a task, a worker's result, a worker blocked on a permission, a merge,
an errand that ran out, and every other note lich types today. It replaces the `[lich]` notes in prose and the
`<task-notification>` the Claude Code mod rendered
([mod-control.md](mod-control.md), A prompt as a notification).

The envelope is what lich writes; the plugin never builds one. The plugin's side
is the briefing: the text each provider is handed at spawn says what the
envelope is and how to answer it, in the prompt language
(`LICH_PROMPT_LANG`). A briefing that describes this shape is the plugin's half
of the contract.

## Shape

```xml
<lich-notification version="1" kind="worker-finished" from="Fix #412" ticket="4b9bc94e">
<status>answered</status>
<result>The handshake raced on reconnect; fixed in relay/conn.go, tests added.</result>
</lich-notification>
```

- One root element, `lich-notification`, with `version` and `kind` always set.
- Attributes carry what identifies the notice; child elements carry text.
- Tag names, attribute names and every enumerated value (`kind`, `status`) are
  English and never translated. Text inside the elements is in the prompt
  language when lich wrote it, and verbatim when an agent did (a result, a
  task).
- `version` is `1`. A change that adds a kind, an attribute or an element keeps
  `1`; one that renames or removes any of them moves it.

### Attributes

| Attribute | On | Meaning |
|---|---|---|
| `version` | every kind | `1` |
| `kind` | every kind | one of the kinds below |
| `from` | every kind but `merged` | the label of the session the notice is about (the asker on `task` and `message`, the worker otherwise) |
| `session` | every kind but `merged` | that session's lich id, which `send_to_session` and `close_session` take |
| `ticket` | `task`, `message` with a reply owed, every `worker-*`, `expired` | the errand's ticket |
| `answer` | `task` | how the worker's answer is taken: `last-message` (its turn's final message is the answer, nothing to call) or `reply` (call `reply_to_session` with the ticket) |
| `truncated` | `worker-finished` | how many characters of the result were cut; absent when nothing was |
| `pr`, `branch` | `merged`, and `notice` about a pull request | the pull request number and its branch |
| `topic` | `notice` | what the notice is: `checks-failed`, `checks-passed`, `review`, `scheduled`, `usage-reset`, `compacted`, `unanswered`, `loop` |

### Kinds

| Kind | Sent to | Children |
|---|---|---|
| `task` | a worker, when its task is handed over | `<prompt>` |
| `message` | any session, a peer's `send_to_session` | `<prompt>` |
| `worker-finished` | the asker | `<status>answered</status>`, then `<result>` in a chat or `<summary>` in a terminal (see Delivery) |
| `worker-failed` | the asker | `<status>failed</status>`, `<error>`: the API error, refusal or empty final turn that ended it |
| `worker-stopped` | the asker, when someone other than the asker closed the worker | `<status>stopped</status>`, `<by>`: the label of who closed it, or `user` |
| `worker-waiting` | the asker, once per block | `<status>waiting</status>`, `<request>`: what the worker is blocked on (a permission, a question) |
| `expired` | the asker, when a ticket ran out unanswered | `<status>expired</status>` |
| `merged` | every live session of the checkout, and the session that opened it | `<summary>` |
| `notice` | the session it concerns | `<summary>`, and `<prompt>` when the notice carries a task (a scheduled prompt) |

`notice` carries every other note lich types today (`internal/prompt`): a
pull request's checks or a new review, a scheduled prompt (late or on time), a
usage limit that reset, the errands a compaction may have summarised away, a
turn that ended with requests unanswered, a loop warning between two sessions.
`topic` is its one structured field; a new topic is a new value, not a new
version.

`worker-waiting` tells the asker; it does not hand it the decision. Answering a
worker's permission from the asker is not part of version 1.

### Text

- Attribute values are escaped: `&` as `&amp;`, `<` as `&lt;`, `>` as `&gt;`,
  `"` as `&quot;`.
- Element text goes in **unescaped**, byte for byte as written. The model reads
  the envelope as text, not through a parser, so an escaped `a && b` would reach
  it as `a &amp;&amp; b`; a `</result>` inside a worker's message still reaches
  the model as written (measured with the mod's `<task-notification>` on Claude
  Code 2.1.289, [mod-control.md](mod-control.md)). The envelope is therefore
  not well-formed XML by design, and nothing should parse it as XML.
- Tabs and newlines stay as they are; nothing is wrapped in CDATA.
- `<result>` holds the worker's final message, cut at 4000 characters, the cut
  `terminal.LastTurnSaid` already makes. The rest
  is collected whole with `wait_for_answer` and the ticket; `truncated` says how
  much is waiting there.
- `<prompt>` holds the task as sent, under the 8 KB `send_to_session` already
  caps it at.

## Delivery

| Where the session runs | How the envelope arrives |
|---|---|
| Chat (GUI), the seven providers with one | the chat's next user message, queued while a turn runs and sent when it ends; never written into the running turn |
| Terminal (TUI), every provider, Crush always | typed at the prompt by the relay, with every trap the typed-delivery ceilings describe |

In a terminal, a `worker-finished` leaves `<result>` out and the envelope's
`<summary>` says to collect it with `wait_for_answer` and the ticket, as the short
`[lich]` note does today: a long report typed into a TUI is what the relay avoids
(a provider may collapse it, and on Windows a newline inside it may submit half).

Several envelopes ready at once go in one message, oldest first, one blank line
apart. A `task` is never batched with another kind.

## Which errands use it

Every errand, not only a native subagent turned into a card: a task handed over
by `open_session` with a prompt, by `send_to_session` or by `lich send` arrives
as a `task` envelope, and its worker's outcome reaches the asker as a `worker-*`
envelope. `answer="last-message"` applies to all of them wherever lich can read
the worker's final message (its chat driver, or `terminal.LastTurnSaid` in a
terminal), so a worker on any provider answers by finishing its turn, the way
Paseo's `notifyOnFinish` works. `reply_to_session` stays for the worker lich
cannot read (Cursor in a terminal) and for answering before the turn ends.

## Fixtures

[`fixtures/notification.json`](fixtures/notification.json) holds the envelopes
lich writes, byte for byte, one per kind and view, with the escaping and the
batching cases, in English (the prompt language's source). lich's composer is
asserted against it, as its `/mod/status` body is against `mod-status.json`.
The plugin builds no envelope, so it has no side to test; its briefing is
checked by reading.

## lich server side

- One new composer in `internal/relay` builds every envelope; no other code writes
  `<lich-notification`. The texts lich writes inside it come from the prompt
  catalogs, like every agent-facing string.
- The chat drivers (`internal/chat`) take an envelope as a user message: Claude
  Code's stream-json `user` message, Codex's app-server `turn/start` input,
  ACP's `session/prompt`. The queue is the chat's own (`queued`).
- `[lich]` notes and the mod's `<task-notification>` route are removed in the
  same change.

## Known ceilings

- An agent can still ignore an envelope: nothing in it is enforced, it is text
  the model reads. The briefing is the only thing teaching it the shape.
- A TUI delivery is typed, so a long envelope reaches a provider that collapses
  a paste (Claude Code's `[Pasted text #1 +40 lines]`) as a placeholder on
  screen; the model still gets the text.
- A Codex or Kiro session gets the short spawn briefing, not lich's MCP
  instructions (ceilings.md), so on those two the envelope's explanation must
  fit in that briefing.
