# lich

`lich` is an **ADE (agentic development environment) for AI coding agents**; the agents it runs, Claude Code,
Codex and the rest, are what this codebase calls harnesses. A pure-Go backend drives a native window, `native/`
(Gio, libghostty-vt), over a loopback contract (`docs/native-window.md`). It is built for
other developers to use, not only its author: docs, errors and defaults answer to a stranger. Linux first;
Windows and macOS are supported, with no hardware here for either.

**This file is instruction, not documentation.** Rules, gates and the workflow live here. What the project *is*
lives in the code, `docs/` and `CHANGELOG.md` — never restate any of it here.

## Where things are written down

- **Deliberate limits and the traps they set**: `docs/ceilings.md`. Read it before changing anything a session
  touches — spawn and naming, hooks, transcripts and cost, git and worktree flows, persistence, plugin install.
  A change that sets a new trap or closes an old one moves that file in the same PR.
- **Session hooks**: `docs/hooks/` is the canonical, contract-first spec. lich owns the server side; the scripts
  live in the companion repo `lichdotdev/lich-plugin`, so an endpoint or payload change breaks a repo this one
  cannot see — move the contract first, then both sides.
- **Every task**: `task --list`. `native/slice.sh` runs a window on its own backend, DB and port; it never
  touches an installed lich's workspace. That isolation is one `pkill` wide: a pattern like `lich` matches the
  running lich's own backend, and killing it ends every session under it. Kill a rig by the PID it was
  launched with.
- **Building, a rig window and the local gate from an agent session**: `.claude/skills/lich-build-dev/`, the
  procedure and the traps of running them inside the user's own lich.
- **Translations**: two independent languages, the interface and the prompt language (what lich and
  lich-plugin hand agents). `.claude/skills/lich-i18n/` maps where a string lands and every point a new
  language touches, and the rules for writing a message.
- **User-facing feature history**: `CHANGELOG.md`.

## Rules of the codebase

- Go 1.27, pure Go: `CGO_ENABLED=0` and a fully static binary are a constraint, not a default. The window,
  `native/`, is the one exception: its own Go module and binary, `lich-native`, built with cgo (Gio,
  libghostty-vt), talking to the backend only over RPC, `/events`, `/ws` and `/blob`, never importing
  `internal/` (`docs/native-window.md`).
- OS-specific code is selected by build tags behind small seams, never by runtime checks — the PTY is the model
  (`internal/terminal`).
- No literal user-visible or agent-facing text: a string a person reads on a lich screen, or that lich types
  into a session, goes through the catalogs and lands in **every** locale in the same PR (`lich-i18n` skill).
  English is the source; tests stay English. Error text, `CHANGELOG.md`, MCP tool descriptions and CLI help
  stay English by design; the What's new dialog reads a release's translations from `changelog/<locale>.md`.
- An RPC method is declared once in `api` and exposed with `rpc.Handle` in `internal/routes`: the backend,
  `lich` and `native/backend` compile against that one declaration, and `api/testdata/contract.golden` records
  its shape (`go test ./api -update` after a change meant to move it). Its options are one struct, never
  positional arguments. The service method it routes has the method's own shape,
  `func(context.Context, Options) (Result, error)`, and the route hands it over as `s.Method`, no closure; one
  that cannot fail keeps its body in an unexported method its own package calls. An `/events` event is declared there too, emitted with `events.Send` and read with
  `backend.Decode`. There is no codegen.
- A fact more than one place shows (git status, a pull request, plan quota, a session's profile or status) has
  one store per window, fed by one reader, and every screen reads that store. A change to the fact reaches the
  store as an event on `/events`, emitted by the backend once the change is true (after the respawn, not before),
  never as a call from the code that made the change into the screens that show it. Writer emits, store listens,
  screens read: a new screen subscribes and nothing upstream learns it exists.

## Local Gate (before every commit / PR)

- **Run from inside a lich session, the gate runs in a memory jail** (Linux, systemd): an unbounded
  `go test ./...`, the cross-compile loop and the native build starve the lich you are running in
  (`docs/ceilings.md` › the gate kills the window). Run each step sequentially through:
  `T=/var/tmp/lich-gate; mkdir -p $T; systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=2G --nice=10 env GOTMPDIR=$T TMPDIR=$T GOFLAGS=-p=2 <step>`.
  Never start two gate steps at once. The directory stays short: `go test` hands GOTMPDIR to the tests as
  their TMPDIR, and a deeper one pushes the sandbox suite's Unix sockets past the 108-byte path limit.
- `gofmt -l .` clean (fix with `gofmt -w .`) and `go vet ./...` clean.
- `go test ./...` green.
- From `native/`, `gofmt -l .`, `go vet ./...` and `go test ./...` clean, with `PKG_CONFIG_PATH` pointing at a
  libghostty-vt built from the commit in `native/GHOSTTY_COMMIT` (`native/slice.sh` says how). Changed what it
  draws? Take a `-shot` and look at it; the maintainer's review in a real window is the one to trust.
- Shipped anything a user can see? Its `CHANGELOG.md` `[Unreleased]` entry lands in the same PR — the release notes
  are read from there, so an entry written later is an entry that missed its release.
- Touched an OS seam or a `_test.go` build tag? Run the same cross-compile loop CI runs:
  `for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done`
  Cross-compiling only proves it builds; the backend suite runs on real Windows and macOS runners on every PR,
  and that is the answer to trust.

## Hard Invariants

Non-negotiable rules. A violation means the work is not done.

1. **Test coverage ≥ 80%** on the backend. CI fails below the bar on its total
   (`.github/scripts/go-test-summary.sh`). An exclusion goes in that script, never into a skipped test.
   OS/framework boundaries (the PTY, the zenity subprocesses, WebSocket wiring, the `main` bootstrap) are the
   documented exception: cover the pure logic and leave the boundary itself alone.
2. **Tests answer to the contract, never the other way round.** Never weaken, skip, delete or rewrite a test to buy
   a green run or a coverage number. A test may change for exactly two reasons: the contract changed, or the test
   asserted something the contract never promised — name which one in the diff. When in doubt, change the product,
   not the test.
3. **A flake is a bug, and it has a root cause.** Never re-run CI until it goes green, never dismiss a failure as
   "unrelated" without proving it. Reproduce (`go test -count=200 -run TestX ./pkg/`), fix the cause, and measure
   the failure rate before and after.
4. **Clean code.** Small focused functions (< 50 lines); cohesive files (200–400 lines typical, 800 max); no
   nesting deeper than 4 levels; comments only for the *why*; errors handled explicitly, never swallowed; no magic
   values; no secrets in source.
5. **A session feature is traced across every provider.** `internal/providers.Registry` is the checklist —
   Claude Code, Codex, Antigravity, opencode, oh-my-pi, Crush, Cursor CLI, Kiro CLI. Anything a session
   touches (spawn flags, hooks, resume, transcripts, plugin install, MCP) is designed against all eight, and
   `docs/adding-a-provider.md` is the map of every file one lands in. Equal behaviour is not always possible —
   but the gap must be deliberate and written down in the same PR, in both places: a `docs/ceilings.md` bullet
   naming which providers are out and why, and a row in the Provider support table of `README.md` and each
   `README.<lang>.md`, written in what the user sees. A feature that silently works on a single provider is not
   done, and neither is one whose gap only a contributor can find.

## Releases

The version comes from the git tag (`git describe` in the Taskfile, env `VERSION` overrides) and is injected into
`build/linux/nfpm/nfpm.yaml` — never hand-edit it there. Before tagging:

- [ ] Local gate green, backend with `-race`.
- [ ] Cut the `CHANGELOG.md` section the Keep a Changelog way: `[Unreleased]` stays at the top, emptied, and its
      entries move under a new `## [X.Y.Z] - YYYY-MM-DD` heading — bracketed, ISO date, no `v`: the release job
      greps `^## \[X.Y.Z\]` and ships empty notes if it misses. Keep the sections in their canonical order
      (Added, Changed, Deprecated, Removed, Fixed, Security), and refresh the compare links, `[Unreleased]`
      included.
- [ ] Translate the cut section for the What's new dialog with the `lich-changelog-i18n` skill, alert blocks
      included, in the same commit: `TestTranslations` (`internal/patchnotes`) fails until every
      `changelog/<locale>.md` holds this release.
- [ ] Want the release to announce itself? Write GitHub alert blocks right under its heading, before the first
      `###`: `> [!IMPORTANT]` is the release's headline and opens the What's new dialog (one per release — a
      second one is demoted to a callout, which is the signal to fold it into the groups); `> [!WARNING]` is
      something the reader must do or avoid; `> [!NOTE]` is a detail worth a line. One or two sentences each,
      opening with a bold sentence that names its audience. The same blocks render as callouts on the release
      page. Most releases carry none.
- [ ] Push the `vX.Y.Z` tag — `.github/workflows/release.yml` does the rest, and reads the notes from that section.
