# Decision: the session browser is a tab of frames

**Status: the card's Browser item opens it.** Right-click a session card →
**Browser**, above Open in. That opens a tab beside the session: its terminal
and its page, split as any layout is. The agent drives the same page through
`lich browser` and the `browser_*` MCP tools.

## Why frames and not a webview

lich's window is native (`docs/native-window.md`). On Linux it speaks Wayland
directly, without GTK, so no system webview fits inside it; Windows and macOS
would take one, Linux would not. The page is a **headless Chromium** on
`chromium.FindBrowser` (the system's Chrome, Edge, Chromium, Brave, Vivaldi
or Helium), with its own user-data directory. `browser.Attach` starts its
screencast at the pane's size; each frame reaches the window as
`browser:frame:<session>` and is acked only once sent, so a window slow to
draw slows the stream. The pane's pointer, keys, committed text (an input
method's or a paste) and toolbar go back through `browser.Mouse`, `.Key`,
`.Text` and `.Nav`, which light nothing up: only the agent's actions emit
`browser-agent`, which the pane shows as the agent chip and a mark where it
clicked.

## One browser per card

- the card's Browser item → its tab, starting a headless browser at
  `about:blank` when there is none. A session in another layout is asked
  about before it moves.
- the agent's `browser_open` first → a headless browser, and a toast in the
  bottom-right corner whose Open shows that same page in the tab
- **Open in a window** → the page reopens as a headed window on the same
  profile, with its cookies (session ones included); its tab goes. chromedp
  cannot show a headless process in place, so what lives only in the page, a
  half-filled form or sessionStorage, does not survive. Browser on the card
  then brings that window forward.

`browser-state` tells the window when a browser starts, closes, moves to a
window or loads another page; `browser.Running` is what a window reads when
it attaches. A browser no pane shows and no one used for five minutes is
closed; one a pane shows, or a window, is not. The tab takes no other
session, and closing its page closes the browser.

## What the agent may load

`http`, `https`, `about:blank`. Not `file:`, `javascript:`, `data:`, `chrome:`.
Page info never includes form values. Screenshots are files. Click, type and
press are real input events. `press` is named keys only. Arbitrary JavaScript
in the page is not a tool.

## Notes on the page

The crosshair on the pane's toolbar stops the pointer reaching the page: the
element under it is outlined, the outline sliding to the next element in 160
ms as the pointer moves, and a click opens a note on it. Notes wait in a
tray floating over the bottom of the page, which never resizes for it, each
element pinned where it stands, and Send types them into the session's prompt
as one message: each element's label, page, a
selector `browser_click` takes, its React component when the page is a
development build, its markup without any field's value, and the note. A note
opened from the tray or its pin scrolls its element into view first, unless it
already shows whole. An element inside a list, sidebar or dialog that scrolls
is outlined only where that box shows it, and one the box hides has no pin. The
page only answers what is where (`browser.ElementAt`, `browser.Locate`); the
outline, the pins and the note are the window's, so nothing is injected into
the page.

## Console errors

Each page answer (`browser_info`, `browser_click`, `browser_press`) names the
errors the page's console showed since the agent's last one, once each, as
`console error:` lines: a `console.error` call, an uncaught exception with
where it was thrown, or an error Chrome logs itself, such as a resource that
failed to load. Each is one line cut to 200 characters; past the first five, a
`(N more console errors)` line counts the rest. Warnings, info and log
messages are left out. The text is what the page logs, so a page that logs a
form's value hands it to the agent; lich reads nothing else. These lines are
English, outside the prompt-language catalogs, like the rest of the page answer.

## Downloads

A file the page downloads lands in the user's Downloads folder
(`XDG_DOWNLOAD_DIR`, from the environment or `user-dirs.dirs`, on Linux;
`~/Downloads` elsewhere) under the name the page suggests, made unique the way
browsers do (`report (1).csv`): nothing already there is overwritten. Chrome
writes it under its download id into a staging folder inside the session's own
profile, and lich moves it to Downloads once it is complete; across
filesystems the move is a copy. What Chrome downloads on its own, with no page
announcing it, stays in staging and is deleted with the profile. A
click or key press that starts one answers with a `downloaded:` line, its path
and size, after waiting up to ten seconds for it; one still running answers
`downloading:`, and the next page read says when it ends. A file Chrome
finished but lich could not move into Downloads answers `download failed:`
with the path it would have taken. `browser_open` and
`browser_navigate` on a URL that answers with a file download it the same way:
Chrome aborts that navigation and the page stays where it was, so the answer
is the handle with `downloads` rather than an error. That answer also
waits until 2.5 seconds after the download began, because Chrome reloads the
page about 1.5 seconds after an `<a download>` and the reload aborts a
navigation started before it. `browser-download` tells the
window when a file is complete, and the window shows a toast naming it with a
button that opens its folder. The same holds in a window opened with Open in a
window.

## Providers

Claude Code, Codex and opencode receive the tools at spawn. Antigravity,
oh-my-pi, Crush, Cursor CLI and Kiro receive them from the lich MCP server the
plugin registers. Downloads reach all eight the same way: in the answer of the
same tools.
