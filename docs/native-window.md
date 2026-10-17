# Decision: a native window for lich, grown beside the web one

**Status: accepted on 2026-10-10. The native window lives in `native/` and opens with `lich native`; it is
experimental until the cutover below.**

## Why

lich's window is a web page in an embedded Chromium (`docs/chromium-shell.md`). It works, and it is fast enough;
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
  talks to an isolated lich backend over the same RPC, `/events` and `/ws` the web page uses. Expand and revert
  go through `project.FileLines` and `project.RevertLines` unchanged.

## Decision: strangle the web window, do not rewrite lich

The native window grows **in this repository, on `main`, beside the web window**, and replaces it only once
it covers daily use. No new repository and no long-lived branch: lich ships often, and a rewrite kept apart
from `main` either drowns in merges or freezes the web app for months.

- **The backend is the product and stays as it is.** Since #755 it outlives its window: closing the window can
  keep every PTY and session running, with the tray to bring a window back and `lich quit` to end it. A native
  window is one more client of that backend, the way the web page is.
- **`native/` is its own Go module**, built into its own binary, `lich-native`. It talks to the backend only
  through the contract the web page uses (RPC, `/events`, `/ws`, and `/blob` for an image's bytes), never by importing `internal/`. That keeps
  the two windows honest against one contract, and keeps the next point possible.
- **`lich native`** is how it starts. The subcommand finds the running lich the way `lich quit` and
  `lich focus` do (its runtime file: port and token), or starts `lich --no-window` in the background when none
  is running. It closes the web window (`system.CloseWindow`, which leaves lich running), then launches
  `lich-native` next to the `lich` binary with the backend's port and token in its environment. Closing the
  native window ends nothing: the backend and its sessions go on, and the web window can be opened on them
  again.
- **`lich gallery`** opens `lich-gallery` (built from `native/cmd/ui-gallery`), the UI kit's gallery, which needs no
  backend; it is looked up beside `lich` the same way, or at `LICH_GALLERY`.
- **`frontend/` and `shell/` stay untouched** until the cutover. The native window does not reuse the
  frontend's code; it is a reference for what to port, nothing more.

## cgo: an exception, the way `shell/` is one

lich's rule is pure Go, `CGO_ENABLED=0`, one static binary. `lich-native` cannot follow it: Gio needs cgo for
Wayland, X11 and EGL on Linux, and `libghostty-vt` is a C library. It is a separate binary for exactly that
reason, as the Rust `shell/` is: the `lich` binary stays pure Go and static, and nothing about `native/`
reaches its build. The native module gets its own CI job, with Gio's system packages and a `libghostty-vt`
built from a pinned Ghostty commit.

## Known limits while both windows exist

- **One window at a time.** `/events` and `/ws` accept a single client, and the newest replaces the one
  before. `lich native` closes the web window before it opens the native one; a web window opened later takes
  the endpoints back, and the native window says another window took over instead of going stale. Making both
  endpoints accept several clients is what using the two side by side would take; it is not needed to
  homologate the native window.
- **Settings › Window picks the window lich opens** (`window.kind`, web by default). A launch, the tray's
  Show lich, a second launch and `lich focus` all open the chosen one (`system.ShowWindow`), closing the other
  first: the web window through `system.CloseWindow`, the native one through the `window-close` event. The
  tray's second item opens the other window without changing the choice, and so does Switch now in the
  dialog that changing the choice offers. A native window that is missing
  beside `lich`, or ends before it attached, leaves the web window open in its place, with a toast saying why
  (`system.TakeWindowFallback`). The macOS window is built and run only on GitHub's runners
  (`native.yml`, `.github/scripts/mac-e2e.sh`): nobody here has looked at it on a Mac.
- **No replay of a session's working directory.** The directory a terminal moved to arrives only as an event;
  a window that connects later shows the checkout's path until the next `cd`. A session's status and usage are
  not part of the gap: the native window reads `terminal.TurnStates` right after attaching to `/events`. The web
  window has the cwd gap too, and still the status and usage one after a reload; the same RPC would close them.
- **Clickable notifications are Linux only.** The native window talks to the freedesktop notification server
  itself (`desktop_notes_linux.go`); on Windows and macOS it still raises the backend's plain `system.Notify`,
  with no click, no replacing and nothing taken down. On Wayland, Open brings the window up through
  xdg-activation (`Window.Activate` in the lichdotdev/gio fork) with the token the server hands over; a
  compositor may only mark the window urgent instead (Hyprland does unless `misc:focus_on_activate` is on).
  What each notification server makes of it:
  - **swaync** draws Open and Dismiss as buttons and hands over the activation token, so Open raises the
    window. It crops the icon to a circle, which is why lich's icon is a disc with its badge inside.
  - **mako** draws no buttons: a left click is Open, a right click dismisses, and `makoctl menu` lists both.
  - **GNOME Shell and KDE Plasma** draw the buttons.
  - **dunst** closes a note on a left click without running its action unless its config says
    `mouse_left_click = do_action, close_current`.
  Measured with swaync and mako on Hyprland; dunst, GNOME, KDE and niri are read from their documentation.
- **Translations are copied, not shared.** The native window's catalogs (`native/i18n/locales`) were seeded
  once from the web's TypeScript ones and keep their keys; from then on each window owns its own, so a copy
  fix made in the web window until the cutover lands in both. The interface language is the global setting
  `lich.appearance.language` (the system's language before anyone picks one), which the web window writes
  beside its localStorage copy, so a language picked there is the one the native window opens in.

## How the native window catches up

1. **This document.**
2. **The entry PR**: `spike/native` becomes `native/`; `lich native` and `lich --no-window` land; the native
   window says when another window took over; the CI job.
3. **One PR per block of parity**, each reviewed in a real window: the sidebar card and the footer, worktrees,
   several projects, pull requests, the terminal's selection, copy and paste, mouse, search and links, then the
   rest of what daily use turns up.

## Cutover

When the maintainer has used only the native window for a week without going back to the web one, the native
window becomes the default. Then `frontend/`, the embedded page and `shell/` are retired, and the question of
running the backend inside the native process (dropping RPC and WebSockets for in-process calls) is decided,
with the native window's own measurements in hand. Until then, both windows ship.
