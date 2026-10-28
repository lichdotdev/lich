# Decision: a native window for lich

**Status: accepted on 2026-10-10; cut over on 2026-10-20. The native window, `native/`, is lich's only
window: `lich` starts the backend and opens it. The web window, its embedded page and the Chromium shell are
gone.**

## Why

lich's window was a web page in an embedded Chromium. It worked, and it was fast enough;
the reason to move is not speed but feel. The tools lich sits next to on a developer's screen (Warp, the
oh-my-pi team's `tern`) are native apps, and a web page in a window reads as one: its fonts, its scrolling, its
focus and its menus are a browser's, not the desktop's. The goal is a lich that feels like an application
someone built for this machine.

## What the spike proved

A native lich is buildable on [Gio](https://gioui.org), in Go, against lich's existing backend:

- **Terminal**: `libghostty-vt` (Ghostty's terminal core, built from Ghostty `main` with Zig 0.16) parses the
  PTY stream and Gio draws it. Gio's own text stack draws each glyph as a vector path and cannot keep up with a
  full screen (`cat` of a 500MB colored file: 17ms average, 59ms worst frame at 3440×1440). Composing each row
  into an image on the CPU and drawing one quad per row brings it to 5ms average and 10.5ms worst, at the
  100Hz vsync floor. LazyVim, Claude Code and the full keyboard (kitty protocol included) work.
- **Code and diff views**: CodeMirror's role is taken by [godemirror](https://github.com/lichdotdev/godemirror),
  a Gio library grown from this spike and now published on its own: a read-only code view and a diff view
  (unified with an old | new gutter, side by side, expandable gaps, revert per chunk, intra-line changes,
  search). Layout costs about 0.3ms of CPU a frame.
- **The app**: `lich-native`, a vertical slice (sidebar, terminal, diff, card with live path and changes),
  talked to an isolated lich backend over the same RPC, `/events` and `/ws` the web page used. Expand and revert
  go through `project.FileLines` and `project.RevertLines` unchanged.

## Decision: strangle the web window, do not rewrite lich

The native window grew **in this repository, on `main`, beside the web window**, and replaced it once it
covered daily use. No new repository and no long-lived branch: lich ships often, and a rewrite kept apart
from `main` either drowns in merges or freezes the web app for months.

- **The backend is the product and stays as it is.** Since #755 it outlives its window: closing the window can
  keep every PTY and session running, with the tray to bring a window back and `lich quit` to end it. The
  window is a client of that backend.
- **`native/` is its own Go module**, built into its own binary, `lich-native`. It talks to the backend only
  through the backend's loopback contract (RPC, `/events`, `/ws`, and `/blob` for an image's bytes), never by
  importing `internal/`; the RPC's methods, the `/events` payloads and their types are the public `api`
  package both modules compile against. That keeps the window honest against one contract, and keeps the
  backend's own build free of cgo.
- **`lich`** starts the backend and launches `lich-native` found beside the `lich` binary (or the one
  `LICH_NATIVE` pins) with the backend's port and token in its environment. A window that cannot start
  shows an error dialog with the reason and the log path; lich then quits when no session is running, and
  otherwise keeps running (the tray, a second launch and `lich quit` still reach it).
- **`lich native`** opens the window on the running lich. The subcommand finds it the way `lich quit` and
  `lich focus` do (its runtime file: port and token), or starts `lich --no-window` in the background when none
  is running. Closing the window ends nothing: the backend and its sessions go on, and a window can be
  opened on them again.
- **`lich gallery`** opens `lich-gallery` (built from `native/cmd/ui-gallery`), the UI kit's gallery, which needs no
  backend; it is looked up beside `lich` the same way, or at `LICH_GALLERY`. The release packages leave it out:
  `task gallery` builds it into `bin/` and opens it.
- **`LICH_NATIVE_PPROF=127.0.0.1:6060`**, set where `lich native` starts, has `lich-native` serve Go's
  runtime profiles (`net/http/pprof`) on that address, for a heap or CPU profile of a real window:
  `go tool pprof http://127.0.0.1:6060/debug/pprof/heap`. Only a loopback address is accepted, and any
  process on the machine can read what the profiles carry.
- **`GOGC`** left unset, `lich-native` collects at 50 instead of Go's 100: a window with many sessions keeps a
  live heap of hundreds of MB, and at 100 half the resident size is slack the system swaps out. Set `GOGC` to
  choose another value.

## cgo: the one exception

lich's rule is pure Go, `CGO_ENABLED=0`, one static binary. `lich-native` cannot follow it: Gio needs cgo for
Wayland, X11 and EGL on Linux, and `libghostty-vt` is a C library. It is a separate binary for exactly that
reason: the `lich` binary stays pure Go and static, and nothing about `native/` reaches its build. The native module gets its own CI job, with Gio's system packages and a `libghostty-vt`
built from a pinned Ghostty commit.

## Known limits

- **One window at a time.** `/events` and `/ws` accept a single client, and the newest replaces the one
  before; the window that lost them says another window took over instead of going stale.
- **The macOS window is built and run only on GitHub's runners** (`native.yml`,
  `.github/scripts/mac-e2e.sh`): nobody here has looked at it on a Mac.
- **The window catches up on what it missed by asking.** `/events` replays nothing, so right after attaching
  the window reads `terminal.TurnStates`: each live session's status, usage and the working directory (or
  tmux, ssh and the like) it last moved to.
- **Notifications are clickable on every system, each its own way.** On Linux the native window talks to
  the freedesktop notification server itself (`desktop_notes_linux.go`). On Windows it raises a toast under
  lich's AppUserModelID through Windows PowerShell (`desktop_notes_windows.go`): Open and a click on the
  toast go through the `lich-session:` link, registered per user, to `lich focus`, so they also work from
  the notification center after the window closed, and each toast costs about a second of a PowerShell
  process. On macOS it uses the Notification Center (`desktop_notes_darwin.m`), which answers only from
  inside Lich.app; a bare `lich-native` falls back to the backend's plain `system.Notify`. On Wayland, Open
  brings the window up through xdg-activation (`Window.Activate` in the lichdotdev/gio fork) with the token
  the server hands over; a compositor may only mark the window urgent instead (Hyprland does unless `misc:focus_on_activate` is on).
  What each Linux notification server makes of it:
  - **swaync** draws Open and Dismiss as buttons and hands over the activation token, so Open raises the
    window. It crops the icon to a circle, which is why lich's icon is a disc with its badge inside.
  - **mako** draws no buttons: a left click is Open, a right click dismisses, and `makoctl menu` lists both.
  - **GNOME Shell and KDE Plasma** draw the buttons.
  - **dunst** closes a note on a left click without running its action unless its config says
    `mouse_left_click = do_action, close_current`.
  Measured with swaync and mako on Hyprland; dunst, GNOME, KDE and niri are read from their documentation.
- **The interface language is a global setting.** The catalogs (`native/i18n/locales`) were seeded from the
  web window's and keep their keys. The language is `lich.appearance.language`, the system's language before
  anyone picks one.
- **Chinese needs a font the window may not have.** None of the faces it ships draws Han, so its interface
  shaper adds an installed Chinese face (found through fontconfig on Linux, Microsoft YaHei on Windows,
  PingFang SC or Hiragino Sans GB on macOS) whatever the language. With none installed, choosing 简体中文, or
  opening in it, first offers to download Noto Sans SC (Regular, Medium and Bold, about 25 MB, pinned and
  hash-checked in `native/fonts/download.go`) into lich's cache folder (`<user cache dir>/lich/fonts`); the
  language switches only once the font loads, and a launch that would open in Chinese opens in English until
  then.

## How the native window caught up

1. **This document.**
2. **The entry PR**: `spike/native` becomes `native/`; `lich native` and `lich --no-window` land; the native
   window says when another window took over; the CI job.
3. **One PR per block of parity**, each reviewed in a real window: the sidebar card and the footer, worktrees,
   several projects, pull requests, the terminal's selection, copy and paste, mouse, search and links, then the
   rest of what daily use turns up.

## Cutover

Decided by the maintainer on 2026-10-20: `frontend/`, the embedded page, `shell/` and `internal/chromium` were
retired together. `lich` opens the native window; the window
choice in Settings, `lich -- <Chromium flags>`, `--shell` and `LICH_SHELL` went with them. A release ships
`lich` and `lich-native`.

Still open: running the backend inside the native process (dropping RPC and WebSockets for in-process calls),
to be decided with the native window's own measurements in hand.
