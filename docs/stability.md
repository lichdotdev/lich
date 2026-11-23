# Stability promise

lich follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). This page says which parts of lich that
promise covers, so you know what is safe to build scripts, plugins and themes on.

## What is covered

A breaking change to anything in this section ships in a new major version, or goes through a deprecation first: the
old form keeps working, with a visible warning, for at least one minor release before it is removed.

- **The `lich` CLI**, except the experimental commands below. Subcommands, flags, the field names and meaning of
  `--json` output, the columns of `--csv` output, and exit codes. [cli.md](cli.md) is the reference for each of them.
- **MCP tools**, except the experimental ones below. The tool names and input schemas that `lich mcp` serves
  (`internal/cli/mcp.go`).
- **The hook contract** with [`lichdotdev/lich-plugin`](https://github.com/lichdotdev/lich-plugin): the endpoints and
  payloads written down in [hooks/](hooks/README.md). Which plugin
  releases a lich supports, and how the plugin's version number marks a contract change, is under [Versioning](hooks/README.md#versioning).
- **Themes.** The theme JSON format and the `lich-theme.json` pack manifest, as described in [themes.md](themes.md).
- **Project files.** `.worktreeinclude`.
- **Environment variables lich sets in every session:** `LICH_PORT`, `LICH_TOKEN`, `LICH_SESSION_ID`, `LICH_BIN`, `LICH_PROMPT_LANG`,
  `LICH_PROJECT_DIR` and `LICH_WORKTREE_PORT`.
- **Environment variables lich reads at startup:** `LICH_LISTEN_PORT` and `LICH_LOG_LEVEL`.

## What is experimental

These work, but they rest on another product's undocumented internals, which lich does not control. They may change
or stop working in any release, minor or patch, outside semver, and lich marks them where you meet them: in
`lich help`, in [cli.md](cli.md) and in the MCP tool descriptions. Build on them knowing that.

- **`lich control`**, and the MCP tool `control_session`. They drive a session drawn as a chat through that
  chat's own protocol (Claude Code's stream-json, Codex's app-server, ACP for the rest). What each chat takes,
  which models and efforts its picker offers and when a change applies, is behaviour measured on specific
  releases of each provider ([ceilings.md](ceilings.md)).
- **A Claude Code subagent opening as a lich card**, and `lich open --subagent`, which it runs on: since
  lich-plugin 0.22.0 the plugin's pre-tool hook asks lich at `/subagent` before Claude Code's own `Agent` call
  runs, and lich opens the task as a session and vetoes the call ([subagent-cards.md](hooks/subagent-cards.md)).

## What is internal

These can change in any release, minor or patch, without notice. Do not build on them.

- The HTTP endpoints the window talks to: `/rpc`, `/ws`, `/events`, `/restart` and `/debug`. A script that needs
  something from a running lich calls the `lich` CLI, which is covered.
- `runtime.json`.
- The SQLite database schema and the names of settings keys.
- The files lich writes into a provider's own config directory. lich rewrites them itself, so hand edits there do
  not last.
- The log format.

Internal does not mean your data is at risk. Your sessions, projects and settings are migrated forward on upgrade;
only the shape they are stored in is not a contract.

## Upgrades and downgrades

Upgrading from any earlier release to a newer one is supported, and lich migrates what it stores on first start.
Downgrading is not supported: an older lich may not read what a newer one wrote.
