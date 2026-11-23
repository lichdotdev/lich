# Hook contract fixtures

The contracts in the parent directory are prose; these are the same contracts as
**bytes**. One file per contract, one case per line, each naming a payload and
what lich does with it.

They exist because the two sides of every hook live in different repositories:
lich owns the server side, the plugin ([`lichdotdev/lich-plugin`](https://github.com/lichdotdev/lich-plugin))
owns the scripts that build the payloads. A field renamed on one side used to
break the other silently — nothing in either repo's tests would go red. Both
sides asserting against the same lines turns that into a failing test on
whichever side moved first.

## Files

| Fixture                | Contract                                     | Endpoint           |
|------------------------|----------------------------------------------|--------------------|
| `session-state.jsonl`  | [session-state.md](../session-state.md)      | `/hook`            |
| `session-start.jsonl`  | [session-start.md](../session-start.md)      | `/session-start`   |
| `session-title.jsonl`  | [session-title.md](../session-title.md)      | `/session-title`   |
| `session-touched.jsonl`| [session-touched.md](../session-touched.md)  | `/session-touched` |
| `mod-control.jsonl`    | [mod-control.md](../mod-control.md)          | `/mod/acks`        |
| `mod-commands.json`    | [mod-control.md](../mod-control.md)          | `/mod/commands`    |
| `mod-usage.jsonl`      | [mod-usage.md](../mod-usage.md)              | `/mod/usage`       |
| `mod-answer.jsonl`     | [mod-answer.md](../mod-answer.md)            | `/mod/answer`      |
| `mod-status.json`      | [mod-status.md](../mod-status.md)            | `/mod/status`      |
| `subagent-cards.jsonl` | [subagent-cards.md](../subagent-cards.md)    | `/subagent`        |
| `notification.json`    | [notification.md](../notification.md)        | none: what lich writes to an agent |

`mod-commands.json`, `mod-status.json` and `notification.json` are the files that are not cases:
each is a response body lich sends, to `GET /mod/commands` (one command of every
shape) and `GET /mod/status` (every list and every value a field takes), which
lich asserts its encoding against and the mod asserts it can read.
`notification.json` is the envelopes lich hands an agent, one per kind and view
(`chat` or `terminal`), byte for byte; only lich asserts against it, since the
plugin builds none.

The file is named after the contract, not the endpoint — `/hook` predates the
naming and keeps its path for the plugin releases already pointing at it.

## Case format

One JSON object per line:

| Field    | Meaning                                                              |
|----------|----------------------------------------------------------------------|
| `name`   | What the case is. Unique within its file; used as the subtest name.  |
| `body`   | The request body, as JSON. Exactly what a client POSTs.              |
| `raw`    | The request body as a **string**, for bodies that are not valid JSON. |
| `accept` | lich accepts it (`204`, or `200` where the hook reads an answer). The field values it holds *after* parsing. |
| `reject` | lich rejects it (`400`). Why, in prose.                              |
| `answer` | Only where the hook reads lich's answer (`subagent-cards.jsonl`, `session-touched.jsonl`): the `200` body, as JSON, compared whole. In `session-touched.jsonl` an accept case without one is answered `204`. |
| `before` | Optional, where the hook reads lich's answer: the bodies of requests made first, in order, for an answer that depends on them (a repeat, a `wait`, another session's edit). |
| `minutes` | Optional, in `session-touched.jsonl`: the minutes lich's clock moves between the last `before` request and the `body`, `0` when absent. |
| `depth`  | Optional, with `answer`: the asking session's subagent depth, `0` when absent. |
| `kind`   | Optional, with `answer`: the asking session's provider, `claude` when absent. |

Every line carries exactly one of `body` / `raw`, and exactly one of `accept` /
`reject`.

`accept` is the payload **normalized**, not echoed: it is what the contract's
defaulting and trimming produce, so it is where `provider` defaults to `claude`
and a title loses its surrounding whitespace. It lists only the fields a case is
about — a field absent from `accept` is not asserted.

`reject` prose is documentation, not a matcher. lich's error strings are its own
and change freely; what the fixture fixes is *that* the payload is refused.

## Using them

- **lich (server side)** — `internal/terminal/fixtures_test.go` runs every case
  twice: through the contract's parse function, and as a real POST to a live
  transport, asserting the status code and the normalized fields. A contract
  without a fixture file fails that test, so a new hook cannot ship without one.
- **lich's decisions** — `internal/subagent` replays each `subagent-cards.jsonl`
  case with an `answer`: its `before` requests first, then its `body`, against a
  workspace where the asker runs `kind` at `depth` and each session opened is,
  in order, `w1` "Fix #412" (ticket `4b9bc94e`) and `w2` "Docs sweep" (ticket
  `7c1d2e3f`), and compares the answer whole.
- **lich's edit notes**: `internal/relay` replays each accepted
  `session-touched.jsonl` case the same way, `before`, then `minutes`, then
  `body`, in one checkout where `s1` "Relay fixes" (Claude Code), `s2` "Docs
  sweep" (Codex, without lich's tools) and `s3` "Auth fix" (opencode) run and
  `s4` "Old branch" is closed, in English, and compares the answer whole.
- **The plugin (client side)** — the payload a script builds must match an
  `accept` case's `body` for the shape it sends. Vendor the files or read them
  from this repo at
  `https://raw.githubusercontent.com/lichdotdev/lich/main/docs/hooks/fixtures/<name>.jsonl`.

## Changing them

A fixture moves for the same two reasons a test does: the contract changed, or
the fixture asserted something the contract never promised. Name which one in
the diff.

Adding a case is not a contract change — cover a payload the contract already
describes and ship it alone. Editing or deleting a case is: it means the
contract moved, so the prose above moves first, then this file, then lich's
server side, then the plugin. Never loosen a `reject` into an `accept` to get a
green run.
