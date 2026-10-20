---
name: lich-build-dev
description: Build, run and verify lich from an agent session without taking down the user's own lich window. Use before building anything (the Go binary, the native window in `native/`), before running a window on its own backend (`native/slice.sh`, `lich native`, `lich gallery`), before running tests or the local gate, and before verifying a UI change in a running lich.
---

# lich build, rig and local gate

What the gate *is* lives in the root `CLAUDE.md` › Local Gate; every task in `task --list`; the window in
`docs/native-window.md`; first-build prerequisites in `CONTRIBUTING.md`. This skill is the procedure for running
them from inside a lich session, and the traps that procedure avoids.

**You are running inside the user's live lich.** Every session on their screen, this one included, is a child of
its backend. Never `pkill`/`killall`/`kill` by pattern (`lich`, `lich-native`, `slice.sh`): the pattern matches
the user's own lich, and the backend's death takes every session down. Kill only a PID or process group you
started and wrote down.

## 1. First steps in a fresh worktree

```bash
W=$(git rev-parse --show-toplevel)       # use absolute paths from here on
which -a go                              # only mise paths (~/.local/share/mise/...)
# libghostty-vt: built once per machine at native/GHOSTTY_COMMIT; native/slice.sh's header says how
export PKG_CONFIG_PATH=$(dirname "$(find ~/.cache/lich-native-slice/libghostty-vt -name '*.pc' | head -1)")
```

## 2. The local gate, in order

One step at a time, each inside the memory jail from `CLAUDE.md` › Local Gate (`cap` below is that command):
gate steps run in parallel have killed the user's window. Never start two of these at once. Keep `$T`
short: `go test` hands it to the tests as their TMPDIR, and the sandbox suite's Unix sockets break past the
108-byte path limit.

```bash
W=$(git rev-parse --show-toplevel); T=/var/tmp/lich-gate; mkdir -p "$T"
cap() { systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=2G --nice=10 \
  env GOTMPDIR="$T" TMPDIR="$T" GOFLAGS=-p=2 "$@"; }
cd "$W" && pwd &&
  test -z "$(gofmt -l .)" &&
  cap go vet ./... &&
  cap go test ./... &&
cd "$W/native" && pwd &&
  test -z "$(gofmt -l .)" &&
  cap go vet ./... &&
  cap go test ./... &&
  echo GATE GREEN
```

- `native/` needs `PKG_CONFIG_PATH` from section 1; without it the cgo steps fail on a missing `libghostty-vt`.
- Touched an OS seam or a `_test.go` build tag: the cross-compile loop in `CLAUDE.md`, under `cap`.

## 3. A window on its own backend

`native/` is a separate Go module and binary (`docs/native-window.md`). Never point it at the user's lich: the
backend's `/events` and `/ws` serve one client each, so a native window on the user's backend takes the sockets
from their real window. `native/slice.sh` runs an isolated backend instead: its own config dir and DB
(`~/.cache/lich-native-slice/config`), port 47901, separate from the user's lich. The backend outlives the window, so its sessions survive between runs.

```bash
W=$(git rev-parse --show-toplevel); T=/var/tmp/lich-gate; mkdir -p "$T"
cap() { systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=2G --nice=10 \
  env GOTMPDIR="$T" TMPDIR="$T" GOFLAGS=-p=2 "$@"; }
ss -ltnp 'sport = :47901'                       # empty, or a slice backend is already up (see below)
# PKG_CONFIG_PATH from section 1; this checkout's backend and window, built in the jail
cd "$W" && cap go build -o "$T/lich" . &&
cd "$W/native" && cap go build -o cmd/lich-native/lich-native ./cmd/lich-native &&
LICH_BIN="$T/lich" setsid "$W/native/slice.sh" "$W" > "$T/native.log" 2>&1 < /dev/null & echo $! > "$T/native.pid"
kill "$(cat "$T/native.pid")"                   # close the window: your PID only
"$W/native/slice.sh" stop                       # stop the slice backend and its sessions
```

- The argument opens that checkout as the project on the backend's first run; later runs reuse its projects.
- A slice backend already on 47901 is reused as is, whatever binary it runs. Backend code changed, or it is not
  yours? Ask before `slice.sh stop`: it may hold the user's review window and sessions.
- Without `LICH_BIN` the backend is the installed `lich` on PATH, enough when only `native/` changed.
- Drawing changed: `timeout 15 cmd/lich-native/lich-native -runtime
  ~/.cache/lich-native-slice/config/lich/runtime-dev.json -shot out.png -shot-after 4s` writes a frame; the
  window stays open after it, hence the `timeout`. It takes the backend's sockets for those seconds, so not
  while the user has a native window on the same backend. Look at the PNG; the user's review in a real window
  is the one to trust.
- The UI kit gallery (`native/cmd/ui-gallery`, every `native/ui` primitive, no backend and no libghostty-vt)
  is the place to review a primitive. Build it in the jail, `cap go build -o "$T/lich-gallery" ./cmd/ui-gallery`
  from `native/`, then open it with `LICH_GALLERY="$T/lich-gallery" "$T/lich" gallery` (a lich built from this
  checkout; the installed one may predate the command) or run it directly under `setsid`, writing its PID down.
  `-shot out.png` (with `-shot-after`, `-shot-height` for the whole scrolled page) writes a frame and exits.

## 4. Traps

- **rtk rewrites commands and fakes exit codes.** A rewritten command can print a clean summary and still exit
  non-zero, or the other way round. Prefix `rtk proxy` when the exit code or the full output matters. Output
  that looks cut is the same hook: `rtk proxy <cmd>`.
- **Read exit codes, not tails.** `| tail` hides the exit status; redirect to a log under `$T` and echo `$?`.
- **cwd drifts.** A failed `cd native && ...` leaves the shell in another directory, sometimes another
  checkout. Use `$W` and start a step with `pwd &&` when it matters.
- **Two Go toolchains.** mise exports `GOROOT`; a stray `/usr/bin/go` obeys it and fails intermittently with
  stdlib mismatches. `which -a go` must list only mise paths.
