# Contract: session model

Reports the model id a running provider session is using so lich can show it in
the session footer. See [README.md](README.md) for the shared transport
(`LICH_PORT` / `LICH_TOKEN` / `LICH_SESSION_ID`) and client rules every hook
follows.

## Request

```
POST http://127.0.0.1:${LICH_PORT}/session-model?token=${LICH_TOKEN}
Content-Type: application/json

{"session_id":"<LICH_SESSION_ID>","model":"<model id>"}
```

- `session_id` — the lich card, from `LICH_SESSION_ID`.
- `model` — a non-empty model id, trimmed by lich and limited to 256 bytes;
  NUL, CR and LF are rejected.

Responses: `204` ok · `401` invalid token · `400` invalid body · `500` lich
failed to report.

Both sides test against the payloads in
[`fixtures/session-model.jsonl`](fixtures/session-model.jsonl).

## Event → action mapping

| Provider | event | model value |
|----------|-------|-------------|
| Pi | `session_start` | `ctx.model.id` |
| Pi | `model_select` | `event.model.id` |

Pi reports on session start because restoring a session does not fire
`model_select`. Later model changes are reported as they happen. Reports are
best-effort and do not hold up the agent.

## lich server side

The endpoint accepts reports only for Pi sessions. The backend keeps the latest
model on the running session, emits `session-model` when it changes, and
includes it in the session snapshot so a reconnected window can restore the
footer. The model id is transient; it is not persisted across a later process
restart.
