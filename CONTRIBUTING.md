# Contributing

Bugs and ideas are welcome, and so are patches. This file is the short version;
`CLAUDE.md` holds the invariants and the deliberate limits behind them.

## Reporting

- **Something is broken** → the [bug form](https://github.com/lichdotdev/lich/issues/new?template=bug.yml).
  Settings → Help → *Report a bug* opens it with the version and platform
  already filled in, and *Open log folder* puts you next to the file to attach.
  Read the log before attaching it: it carries paths, project and branch names
  and your gh login — never a session token.
- **Something is missing** → the [feature form](https://github.com/lichdotdev/lich/issues/new?template=feature.yml).
  Describe the problem first; the shape you have in mind is optional, and a
  well-described problem often has a better answer than either of us reaches
  for first.
- **A security problem** → not an issue. See [SECURITY.md](SECURITY.md).
- **The session hooks** (titles, git refresh, resume) ship from
  [lich-plugin](https://github.com/lichdotdev/lich-plugin) — report those there.

## Getting it running

Prerequisites: **Go 1.27.0+** and **[Task](https://taskfile.dev)**. The
backend, `lich`, is pure Go (`CGO_ENABLED=0`) and needs no C toolchain.
`zenity` provides the folder picker on Linux.

The window, `lich-native`, is its own Go module in `native/`, built with cgo:
it needs a C compiler and `pkg-config`, Gio's system libraries (on
Debian/Ubuntu `libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev
libgles2-mesa-dev libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev`),
and a `libghostty-vt` built with **Zig 0.16** from the Ghostty commit in
`native/GHOSTTY_COMMIT`, with `PKG_CONFIG_PATH` pointing at it. The header of
`native/slice.sh` says how to build it; `native/build-windows.sh` cross-builds
the Windows window from Linux.

```bash
native/slice.sh <checkout>   # a window on its own backend, DB and port
task --list                  # everything else
```

`native/slice.sh` never touches the workspace of a lich you have installed.
Closing its window leaves the backend and its sessions running, as an
installed lich's backend outlives its window; `native/slice.sh stop` ends it.

## Before you open a pull request

Run the same gate CI runs:

```bash
gofmt -l .                      # must print nothing (fix: gofmt -w .)
go vet ./...
go test ./...
cd native && gofmt -l . && go vet ./... && go test ./...
```

Then:

- **Shipping something a user can see?** Add its `CHANGELOG.md` `[Unreleased]`
  entry in the same pull request. Release notes are read from that file, so an
  entry written later is an entry that missed its release.
- **Touched OS-specific code or a `_test.go` build tag?** Run the cross-compile
  loop — CI runs the backend suite on real Windows and macOS runners for every
  pull request, so that is where the answer comes from; this only saves you a
  round trip:
  ```bash
  for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done
  ```
- **Touched an RPC method's options or result?** Methods declared in `api` are
  shared by the backend and the window; run `go test ./api -update` and review
  the diff of `api/testdata/contract.golden`. The services not moved there yet
  are mirrored by hand in `native/backend`, so the mirror moves in the same
  change. There is no codegen.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org)
(`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`, `ci:`).
`main` takes squash and rebase merges only.

## Two rules worth stating outright

**Tests answer to the contract, never the other way round.** Never weaken, skip
or delete a test to buy a green run. A test changes for exactly two reasons:
the contract changed, or it asserted something the contract never promised —
say which one in the pull request.

**A flake is a bug with a root cause.** Don't re-run CI until it goes green.
Reproduce it (`go test -count=200 -run TestX ./pkg/`), fix the cause, and say
what the failure rate was before and after.
