# lich-browser: the process that holds every page

**Status: contract, Linux first.** This replaces the headless Chromium of
`docs/browser-tab.md`. Windows (WebView2) and macOS (WKWebView) follow on the
same contract; Chrome, chromedp and the screencast go once all three run.

## Why a process of its own

The backend is pure Go (`CGO_ENABLED=0`) and cannot link a web engine. The
window can, but a page it held would die with it: today an agent browses with
the window closed. So the pages live in `lich-browser`, a cgo binary built in
`native/` beside `lich-native`, which the backend starts and owns:

- one page per session, the same one the agent drives and the person sees;
  nothing follows another page by URL;
- a crash of the engine takes `lich-browser` down, never the window or the
  backend, which restarts it and tells the agent its page is gone;
- its log is its own (`lich-browser.log` beside the backend's).

## Who talks to whom

```
backend ──stdin/stdout, JSON lines──▶ lich-browser ◀──X11── lich-native
   ▲                                                         │
   └──────────── RPC, /events (api) ─────────────────────────┘
```

The window never talks to `lich-browser` directly. Everything a page does
goes through the backend (`internal/browser`, whose `Page` it implements),
and the window only puts a page's X window where it draws the pane.

## Showing a page in the window (X11)

Each tab is an X window of `lich-browser`'s. With no window to sit in, it is a
mapped override-redirect toplevel off every screen, so WebKit keeps it
visible: timers, `requestAnimationFrame` and layout run as for a person.

The front tab's window reaches the window as `xid` on its tab in
`browser-state`. The window then, on its own X connection:

1. `XReparentWindow` into its own window, and `XAddToSaveSet`: a window that
   dies hands the page back to the root instead of taking it down;
2. places, maps, unmaps and cuts it (X SHAPE, bounding and input) as the
   in-process view did, so a cut and the frame that draws there still land in
   one commit;
3. before it closes, reparents every page back to the root.

`lich-browser` follows its windows' `ConfigureNotify` to size the GTK view,
and moves a window it finds reparented to the root back off screen.

The keyboard rules stay those of `native/internal/webview`: the window keeps
the keyboard; a click into a page gives it to that page until a press in the
window takes it back (the window's own `XSetInputFocus`). `lich-browser`
learns the window's X window from `window` to send it the keys a page gets
only for sitting under the pointer.

## Showing a page in the window (Windows, macOS)

`xid` and `window` carry each system's own handle; the rest of the wire is
the same.

- **Windows** (WebView2): each tab is an HWND of `lich-browser.exe`'s, a
  shown toplevel off screen with no window to sit in (Chromium's occlusion
  tracking is off, so the page runs as one in sight). The window makes it a
  child of its own with `SetParent` and places, shows, hides and clips it
  (`SetWindowRgn`) as the X11 path does; giving it back, the window makes it
  a toplevel off screen again, since Windows tells `lich-browser.exe` nothing.
  `lich-browser.exe` loads WebView2 through the official `WebView2Loader.dll`,
  shipped beside it; its header `WebView2.h` is vendored
  (`native/third_party/webview2`). Each page is an InPrivate profile of its
  own. Eval, input, `request` events and the file chooser go over the
  DevTools protocol (`CallDevToolsProtocolMethod`); input sent that way is
  trusted. The inspector is Edge's DevTools window, made a child of a tab
  window. `window` is kept and unused: an embedded page takes keys by
  Windows focus.
- **macOS** (WKWebView): one process cannot put a view in another's window,
  so each tab's WKWebView draws in an off-screen window of `lich-browser`'s
  and exports its layer as a `CAContext`; `xid` is that context's id. The
  window shows it in a `CALayerHost` over the pane, the way Chromium shows
  its GPU process, and sends the pane's pointer and keys over the wire
  (`browser.Mouse`, `.Key`, `.Text`, as the screencast does). `CALayerHost`
  is private API.

Wayland without XWayland: a later step draws pages off screen and hands the
window their frames by shared memory. Until then the window opens on
XWayland (`app.PreferX11`) as it does today.

## The wire

One JSON object per line. The backend sends requests; `lich-browser` answers
each one, in any order, and sends events in between.

```json
{"id": 7, "method": "eval", "params": {"page": "b1", "script": "document.title"}}
{"id": 7, "result": {"json": "\"Example\""}}
{"id": 8, "error": "page b9 is not open"}
{"event": "load", "params": {"page": "b1", "tab": "t1", "step": "finished", "url": "https://example.com/"}}
```

`page` is the backend's browser id. `tab` is `lich-browser`'s, and every
request without one acts on the page's front tab. Coordinates are CSS pixels
of the front tab's viewport; images are PNG, base64.

The types and method names are `browserwire` (repo root), which both sides
import; this table is what each one does.

### Requests

| method | params | result | does |
|---|---|---|---|
| `hello` | | `version` | first request; a version the backend does not know ends the process |
| `window` | `xid` | | the window's X window, 0 once it closed |
| `open` | `page`, `url`, `scale`, `w`, `h`, `downloads` | | a page with one tab on `url`, laid out at `w`×`h` CSS pixels drawn at `scale` while no window shows it. Its cookies and storage are its own and end with it. Downloads land in the `downloads` directory under the name the server suggests, made unique there as Chrome makes it (`name (1).ext`) |
| `close` | `page` | | ends the page and its tabs |
| `newTab` | `page` | | a blank tab in front |
| `selectTab` | `page`, `tab` | | brings `tab` to the front |
| `closeTab` | `page`, `tab` | | closes `tab`, unless it is the last |
| `navigate` | `page`, `url` | | loads `url` in the front tab |
| `reload` | `page` | | reloads past the cache |
| `back`, `forward` | `page` | | history |
| `eval` | `page`, `script` | `json` | runs `script` in the page's world as the DevTools console does: the value of its last expression, top-level `await` allowed, a `let` or `const` declared again by a later eval allowed; a promise is awaited. `json` is the value as JSON, empty for `undefined`; a throw is the error, its message as the page has it |
| `mouse` | `page`, `kind` (`move`, `down`, `up`, `wheel`), `x`, `y`, `button`, `clicks`, `mods`, `dx`, `dy` | | a native pointer event: the page sees `isTrusted`. `button` is the DOM's, 0 left, 1 middle, 2 right |
| `key` | `page`, `kind` (`down`, `up`), `key`, `code`, `mods` | | a native key event |
| `text` | `page`, `text` | | commits `text` as an input method does |
| `snapshot` | `page`, `bare` | `png`, `w`, `h` | what the front tab shows, at `scale` image pixels per CSS pixel; `w`, `h` its viewport. `bare` leaves the window's layer out (the element `LayerID`, which `draw` draws in) |
| `draw` | `page`, `script` | | runs `script` in the front tab's world to draw the window's layer over the page (notes, sketch, the agent's cursor), answered at once: while one runs, the next waits and a newer one takes its place |
| `zoom` | `page`, `scale` | | device pixels per CSS pixel |
| `size` | `page`, `w`, `h` | | the layout size while no window shows it |
| `dialog` | `page`, `accept`, `text` | | answers the open JavaScript dialog |
| `files` | `page`, `paths` | | answers the open file chooser; none cancels it |
| `inspect` | `page` | | WebKit's inspector of the front tab, as a `devtools` tab right after it, brought to the front; the same tab when it is open already |
| `popOut` | `page` | | the page in a window of its own, as Open in a window: same page, nothing reloads; its tabs' `xid` go 0 while it is out |
| `popIn` | `page` | | the page back out of its own window, for the pane to embed again |
| `raise` | `page` | | brings a popped-out page's window to the front |
| `clock` | | `ns` | the monotonic clock every event's `at` reads |

### Events

| event | params | when |
|---|---|---|
| `tabs` | `page`, `tabs` (`id`, `url`, `title`, `loading`, `front`, `xid`, `icon`, `devtools`) | a tab opens, closes, comes to the front, or changes URL, title, icon or loading. A `devtools` tab is the inspector of the tab before it, with an X window of its own; while it is `front` every request still acts on the tab it inspects. `selectTab` on it shows it, `closeTab` closes the inspector |
| `load` | `page`, `tab`, `step` (`started`, `committed`, `finished`, `failed`), `url`, `at`, `error`, `download` | each main-frame load step; `failed` carries WebKit's error text in `error`, and `download` when the navigation turned into a download instead |
| `request` | `page`, `tab`, `id`, `type` (`fetch`, `xhr`, `document`, `script`, `stylesheet`, `image`, `font`, `media`, `other`), `started` or `finished`, `at` | each request of the front tab, for `Settle`: `finished` names the `id` that started, however it ended |
| `console` | `page`, `text` | an error the page's console shows, or one it did not catch |
| `dialog` | `page`, `dialog` (`type`, `message`, `default`), null once closed | a JavaScript dialog, `beforeunload` included |
| `files` | `page`, `multiple` | a file input asks for files |
| `download` | `page`, `guid`, `url`, `name`, `path`, `state`, `received`, `total` | each step of a download |
| `crashed` | `page` | the page's web process died |
| `closed` | `page` | the person closed a popped-out page's window, which ends the page |

## What stays Chrome's until the end

All three systems hold their pages here. `internal/browser` picks the
launcher at start: `lich-browser` beside the backend and an X display on
Linux, `lich-browser` beside `lich` in Lich.app's `Contents/MacOS` on macOS,
`lich-browser.exe` beside `lich.exe` on Windows. Chrome stays only where none
is found, and for Wayland without XWayland, until pages are drawn off screen
there (above); then Chrome, chromedp and the screencast go.
