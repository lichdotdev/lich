# Decision: the session browser is a pane of frames

**Status: the card's Browser item opens it.** Right-click a session card →
**Browser**, above Open in. That puts the page beside the session's terminal,
wherever that terminal is shown. The agent drives the same page through
`lich browser` and the `browser_*` MCP tools.

## Why frames and not a webview

lich's window is native (`docs/native-window.md`). On Linux it speaks Wayland
directly, without GTK, so no system webview fits inside it; Windows and macOS
would take one, Linux would not. The page is a **headless Chromium** on
`chromium.FindBrowser` (the system's Chrome, Edge, Chromium, Brave, Vivaldi
or Helium), with its own user-data directory. The browser draws at the scale
the window drew pages at when it launched (`launchScale`), so the frames are
the pane's own pixels. `browser.Attach` starts its
screencast at the pane's size; each frame reaches the window as
`browser:frame:<session>`, naming the tab it is of, and is acked only once sent, so a window slow to
draw slows the stream. Once the page sends no frame for `stillAfter`, the
backend sends one PNG capture of the front tab as the next frame
(`frameSink`), dropped if a JPEG came while it was taken or if it is over
`stillGrowth` times the JPEG it would replace; a frame byte for byte the last
one, which a capture draws, is no change, and any other frame within
`stillEcho` of a capture starts the settle over only `stillTries` times. The pane's pointer, keys, committed text (an input
method's or a paste) and toolbar go back through `browser.Mouse`, `.Key`,
`.Text` and `.Nav`, which light nothing up: only the agent's actions emit
`browser-agent`, which the pane shows as the agent chip and a mark where it
clicked.

## One browser per card

- the card's Browser item → its page beside its terminal, starting a headless
  browser at `about:blank` when there is none. A pane with no room for both
  refuses with a toast, as a split does.
- the agent's `browser_open` first → a headless browser, and a toast in the
  bottom-right corner whose Open shows that same page beside the terminal
- **Open in a window** → the page reopens as a headed window on the same
  profile, with its cookies (session ones included); its pane goes. chromedp
  cannot show a headless process in place, so what lives only in the page, a
  half-filled form or sessionStorage, does not survive. Browser on the card
  then brings that window forward.

`browser-state` tells the window when a browser starts, closes, moves to a
window or loads another page; `browser.Running` is what a window reads when
it attaches. A browser no pane shows and no one used for five minutes is
never closed on its own: its state turns `idle`, the window asks with a toast,
and Close (`browser.Close`) or Keep open (`browser.Keep`, five more minutes)
answers. Unanswered, it stays open and is not asked about again; any use
withdraws the question. One a pane shows, or a window, is never asked about.

## The page belongs to its session, not to a tab

The page makes no tab in the strip and is a pane of no stored layout
(`native/cmd/lich-native/pages.go`). The window keeps, per session, whether
its page is on screen and how it sits against the terminal (side by side or
stacked, and the divider), under `lich.native.pages`, and the stage draws the
page beside that session's terminal: alone in its project, or inside whatever
layout holds the session. So the session is in one place with its page, the
page follows it in and out of every split, and closing a layout's tab hides
or closes no browser.

The pane's header carries the two ways out, which are the person's to pick:

- **Minimize** takes the page off the screen and leaves the browser running,
  for the agent to keep using. The card's Browser item brings it back on the
  page it was on. Taking the page's pane out with the unsplit key, or the
  terminal's out of that pair when the session is in no layout, does the same.
- **Close** (the ×) closes the browser.

For as long as the Chromium runs, the sidebar's tree shows it as a row under
its session, the globe in a ring and the page's host: a minimized browser is
seen there, its name muted. A click on the row shows the page, and its menu
holds Show or Minimize, Open in a window and Close the browser.

An earlier build gave each page a locked layout tab holding only the session
and its page. The same session then showed in two tabs, that one and its
project's, and closing the tab left the browser running with nothing saying
so. A page pane such a build stored in a layout is dropped when the layouts
are read.

## What the agent may load

`http`, `https`, `about:blank`. Not `file:`, `javascript:`, `data:`, `chrome:`.
A local file inside the session's checkout, typed or sent as an absolute path
or a `file:` URL, loads over http instead (`internal/preview`): the backend
serves that checkout on `127.0.0.1`, at its `LICH_WORKTREE_PORT` plus 1000, so
each checkout is its own origin, and a path outside the checkout is refused.
A file in the session's artifact folder (`LICH_ARTIFACT_DIR`, `internal/artifact`),
where the agent writes the pages it makes to show the user, loads the same way
from a listener of its own at `LICH_WORKTREE_PORT` plus 2000, so those pages
never share the checkout's origin.
Page info never includes form values. Screenshots are files, their longest
side at most 1280 pixels (`shotMax`); a click at x and y is in that image's
pixels and lands scaled back to the page's. An annotated screenshot reads the
page first and draws each element's index on the image (`chrome_annotate.go`);
that read is not told, as its list never reached the agent in text. A click, key, tab or dialog
answer whose element list is the one the agent's last page answer gave says
so in one line instead (`internal/browser/told.go`); `browser_info` always
answers with the whole list of what it reads. With `within`, a CSS selector, it
reads the first element it matches (its text and the buttons and links under it,
not the element itself) and a selector that matches nothing is an error; the
actions that follow answer with the whole page. Click, type and
press are real input events. A click, or the click a target of type or press
makes, by index or selector fails before dispatch when the element is disabled
or another one covers its click point, naming it (`covered by <div#consent-banner>`). `press` is named keys only. `browser_eval` runs
the agent's script in the front tab (`browser.Eval`) and answers with its
value, a string as it is and anything else as JSON, cut at 2000 characters; it
can read any field's value, so page info's rule ends where the agent asks.

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

What the page wrote (`title:`, `console error:`, `dialog answered:` and
`dialog open:`, `text:`, the element list and its legend) sits between
`<page 3f9a1c2e>` and `</page 3f9a1c2e>`, a fresh random nonce per answer;
`url:`, `wait:`, the download lines and what a held dialog asks stay outside.
`browser_eval` and `lich browser eval` wrap their value the same way.

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
is the handle with `downloads` rather than an error. A download a page asks
for in a browser's first seconds reloads the page when Chrome starts it, so
the agent's next click or key press waits for that reload when the answer
before it showed the page as it was, and a navigation the reload aborts is
loaded again. `browser-download` tells the
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

## DevTools

The toolbar's DevTools button opens Chrome's own DevTools for the front tab
as one more tab of the pane, right after the tab it inspects, and selects it;
with one open, the button is lit and only selects it. While the DevTools tab is
selected the pane draws DevTools in the page's place and sends it the pointer,
the keys and the pane's size; the notes button is gone, and back, forward,
reload and the address still act on the page. Closing the page tab closes its
DevTools.

- It is the browser's own front-end, served on its debugging port
  (`inspector.html?ws=...`), loaded by a target lich creates in a headless
  window of its own (`internal/browser/chrome_devtools.go`): as a tab behind
  the page it drew no frames. Its own screencast of the page and its Paused in
  debugger bar over the page are turned off before it loads.
- The agent never sees it. `browser_tabs` lists the pages only, with the page
  DevTools inspects in front, every tool keeps acting on that page, the
  agent's cursor is not drawn over DevTools, and a tab action naming a DevTools
  tab is refused (`agentTabs`, `Service.TabAct`). Its URL, which reaches the
  whole browser, is in no state the window or the agent receives.
- What DevTools opens in a new window (a Learn more link, a request opened in
  a new tab) joins the tabs as a page, after the page it inspects, behind:
  DevTools stays on the pane and the agent on its page. The agent lists the new
  tab like any other.
- While a page is stopped in DevTools' debugger, its DevTools tab carries a
  pause glyph. Lich hears the pause on its own session of the page: it enables
  the debugger there with breakpoints off, only while DevTools is open, so it
  never pauses the page itself. A paused page answers a script with the
  document as it stood and takes a click or a key as done and loses it, so
  every agent tool fails at once naming the pause, and the pane's own input to
  the page does nothing. Closing DevTools lets the page go on.

## Providers

Claude Code, Codex and opencode receive the tools at spawn. Antigravity,
oh-my-pi, Crush, Cursor CLI and Kiro receive them from the lich MCP server the
plugin registers. Downloads and dialogs reach all eight the same way: in the
answer of the same tools, and `browser_dialog` for all of them. Tabs too:
`browser_tabs` and `browser_tab` reach all eight. DevTools changes none of
the agent's tools, so it is the same on all eight.
