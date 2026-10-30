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
A local file inside the session's checkout, typed or sent as an absolute path
or a `file:` URL, loads over http instead (`internal/preview`): the backend
serves that checkout on `127.0.0.1`, at its `LICH_WORKTREE_PORT` plus 1000, so
each checkout is its own origin, and a path outside the checkout is refused.
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
form's value hands it to the agent; lich reads nothing else.

## Dialogs

An `alert`, `confirm`, `prompt` or `beforeunload` the page opens stays open
until someone answers it, and until then the page runs no script and takes no
input. lich answers no dialog on its own:

- The pane shows it as a card over the top of the page, naming the site that
  opened it, with OK and Cancel (Leave and Stay for a `beforeunload`) and a
  field for a `prompt`. The person answers there.
- The agent's call that opened it answers at once, rather than hanging, with
  the page as it stood and a `dialog open:` line; any other call on that page
  fails with a message naming the dialog. `browser_dialog` (`lich browser
  dialog accept|dismiss`) answers it, and the pane plays the agent's cursor
  pressing the card's button. `text` is what a `prompt` returns on OK; left
  out, it returns its default text.
- Whichever answers first wins. A dialog the person answered reaches the
  agent's next page answer as a `dialog answered:` line.

Accepting a `beforeunload` less than a second after it opened waits out the
rest of that second: Chrome aborts the navigation it held when it is accepted
sooner, and reloads the page, which asks again. On a busy machine that second
can still be too soon; lich then takes the leave up again once the reload
lands, so Leave goes where it was going.

## File inputs

A click on a file input opens the system's file chooser, the one the footer's
attach button opens, for one file or several as the input asks; what the
person picks fills the input, and Cancel leaves it as it was. It opens only
while a pane shows the page: an agent's click on a file input in a page nobody
is looking at leaves it empty, and no tool fills one.

## Languages of the page answer

The line tags of a page answer (`url:`, `title:`, `wait:`, `text:`,
`interactive`, `downloaded:`, `downloading:`, `console error:`, `dialog
open:`, `dialog answered:`) stay English: the tool descriptions name them. The
prose around them, the element list's legend, the filled and checked marks,
the note that more elements wait below, and what a held dialog asks of the
agent, follows the session's prompt language.

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

## Tabs

The browser keeps a list of tabs and draws one, the front tab: a headless
Chrome draws no frames for a tab behind another. The pane's header shows the
pane's name and a + while there is one tab, and the tabs in its place once
there are more.

- What a tab opens in a new tab or window (`target=_blank`, `window.open`)
  joins the list right after it, in front when its opener was. Closing it,
  from the page's own `window.close()` or a ×, brings its opener back as it
  was, its script's state and login intact; a tab nobody opened gives way to
  the one beside it. The last tab cannot be closed.
- The + opens a blank tab in front with the address field focused.
- The agent lists them with `browser_tabs` (`lich browser tabs`), by index,
  and opens, selects or closes one with `browser_tab` (`lich browser tab new
  [url] | select <i> | close <i>`), which answers with the page then in front.
  Every other tool acts on the front tab. The pane plays the agent's cursor
  pressing the tab, its × or the +.
- A tab opened behind the front one shares its renderer when it keeps its
  opener, so a dialog there holds the front tab too: it reaches the pane and
  the agent at once, and the answer goes to the tab that opened it.

## Providers

Claude Code, Codex and opencode receive the tools at spawn. Antigravity,
oh-my-pi, Crush, Cursor CLI and Kiro receive them from the lich MCP server the
plugin registers. Downloads and dialogs reach all eight the same way: in the
answer of the same tools, and `browser_dialog` for all of them. Tabs too:
`browser_tabs` and `browser_tab` reach all eight.
