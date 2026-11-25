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
| `inspect` | `page` | | WebKit's inspector on the front tab, in a window of its own |
| `popOut` | `page` | | the page in a window of its own, as Open in a window: same page, nothing reloads; its tabs' `xid` go 0 while it is out |
| `popIn` | `page` | | the page back out of its own window, for the pane to embed again |
| `raise` | `page` | | brings a popped-out page's window to the front |
| `clock` | | `ns` | the monotonic clock every event's `at` reads |

### Events

| event | params | when |
|---|---|---|
| `tabs` | `page`, `tabs` (`id`, `url`, `title`, `loading`, `front`, `xid`, `icon`) | a tab opens, closes, comes to the front, or changes URL, title, icon or loading |
| `load` | `page`, `tab`, `step` (`started`, `committed`, `finished`, `failed`), `url`, `at`, `error`, `download` | each main-frame load step; `failed` carries WebKit's error text in `error`, and `download` when the navigation turned into a download instead |
| `request` | `page`, `tab`, `id`, `type` (`fetch`, `xhr`, `document`, `script`, `stylesheet`, `image`, `font`, `media`, `other`), `started` or `finished`, `at` | each request of the front tab, for `Settle`: `finished` names the `id` that started, however it ended |
| `console` | `page`, `text` | an error the page's console shows, or one it did not catch |
| `dialog` | `page`, `dialog` (`type`, `message`, `default`), null once closed | a JavaScript dialog, `beforeunload` included |
| `files` | `page`, `multiple` | a file input asks for files |
| `download` | `page`, `guid`, `url`, `name`, `path`, `state`, `received`, `total` | each step of a download |
| `crashed` | `page` | the page's web process died |
| `closed` | `page` | the person closed a popped-out page's window, which ends the page |

## What stays Chrome's until the end

Windows and macOS keep the Chromium page until their engine lands here.
`internal/browser` picks the launcher at start: `lich-browser` found beside
the backend and an X display, or Chrome.
