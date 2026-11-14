# Writing a lich extension

An extension is a separate program that lich starts with its backend and talks to over JSON lines on stdin and
stdout. It draws full screens (surfaces) in lich's window by sending a tree of components, which lich draws with
its own UI kit. Extensions are compiled Go binaries: lich never clones or builds source.

## Directory layout

Each extension is one directory under lich's config directory:

```
<config dir>/lich/extensions/<id>/
  manifest.json
  <executable>
  locales/<tag>/<namespace>.json   (optional, see Translating an extension)
  ...anything else the extension reads
```

`<config dir>` is `$XDG_CONFIG_HOME` (or `~/.config`) on Linux, `~/Library/Application Support` on macOS and
`%AppData%` on Windows. A development build of lich (`LICH_DEV` set) reads `extensions-dev` instead. The directory
name is the extension's id. A symlinked directory works too, the way to develop an extension in place.

`extensions.Restart` reads the directory again, so an edited `manifest.json` is what runs after it. One that no
longer validates shows invalid with its reason and does not start; fix it and call Restart again, which Settings >
Extensions offers in the menu of a row that is not running. A directory added by hand is found when lich starts.
Changing `id` does not rename the extension, it makes the manifest invalid, since the id must equal the directory
name.

lich writes one file of its own into a directory it installed: `.lich-source.json`, recording the repository and
release tag, or the folder, it came from. Do not ship a file by that name.

## The manifest

`manifest.json` is the `Manifest` type in [`extension/manifest.go`](../extension/manifest.go):

```json
{
  "formatVersion": 1,
  "id": "hello",
  "name": "Hello",
  "version": "0.1.0",
  "minLichVersion": "0.90.0",
  "executable": "hello",
  "surfaces": [
    { "id": "main", "title": "Hello", "icon": "hand", "iconFile": "assets/logo.png" },
    { "id": "issue", "title": "Issue", "icon": "ticket", "place": "dock" }
  ],
  "permissions": ["sessions:read"]
}
```

- `formatVersion` is a positive integer no newer than this lich reads (`extension.ManifestFormat`).
- `id` is lowercase letters, digits, dots, underscores or dashes, starts with a letter or digit, is at most 64
  long, and equals the directory name.
- `minLichVersion`, when set, is `MAJOR.MINOR.PATCH`; an older lich lists the extension as invalid.
- `executable` is a path relative to the directory and must resolve inside it, symlinks included, to a regular
  file. On Windows name the `.exe`.
- `description`, when set, is one line of at most 160 characters saying what the extension does for the person
  using it. lich runs an extension without one; the [catalog](#listing-in-the-catalog) does not list it.
- Surface ids are non-empty and unique. A surface's `place` says where it lives: none for a full screen opened
  from the window's top bar, `dock` for a tab of the dock beside the terminal, `panel` for a panel rising from the
  footer in the Git panel's place (see [Dock and panel surfaces](#dock-and-panel-surfaces)). A lich older than
  `place` shows every surface as a full screen.
- A surface's `icon` names a glyph from the sets the window draws: a Lucide name, `lobe:name` or `devicon:name`.
  `iconFile`, when set, is the extension's own picture, drawn in place of `icon` on the top bar and in Settings ›
  Extensions: a PNG of at most 256 KB and 512 × 512 pixels, centered on a square, under the same path rules as
  `executable`. It keeps its own colors; one whose every visible pixel is black is a silhouette, drawn in the
  theme's colors as `icon` is, so export a black one to have it follow the theme. Keep `icon` set beside it: a
  lich older than `iconFile` draws that one.
- `permissions` lists what the extension's [calls](#calling-lich) use, each one of `extension.Permissions`. A name
  this lich does not know makes the manifest invalid.

An extension that fails these checks is listed with its error and never started.

## The protocol

lich starts the executable with the extension's directory as its working directory and lich's environment. Each
line on stdout is one JSON message; each line lich writes to stdin is one too. The Go types, the components and
the JSON of each are in [`extension/`](../extension/), and `extension/testdata/` records the wire form of every
component and message. Import `github.com/lichdotdev/lich/extension` rather than writing the JSON by hand.

- From the extension: `render` (a surface's tree and shortcuts), `toast`, `toast.dismiss`, `openExternal`,
  `clipboard`, `focus`, `footer` (see [Dock and panel surfaces](#dock-and-panel-surfaces)) and `call` (see
  [Calling lich](#calling-lich)).
- From lich: `surface.opened`, `surface.closed`, `event` (a handler the tree named fired), `language` (see
  Translating an extension), `context` (see Dock and panel surfaces), `result` (the answer to a `call`) and
  `lich.event` (an event the extension subscribed to).

Stderr goes to lich's log.

A `box` lays its children along its axis. A child `box` with `grow` takes that share of the room left; a `scroll`,
`thinScroll`, `splitPane` or `emptyState` takes the room left on its own, and an empty state sits centered in it.
A `textarea` is as tall as its text; with `grow` it takes the room left too, up to its `maxHeight`, and text past
that scrolls.

### Moving the keyboard

`focus` gives the keyboard to a field: the `input`, `inputGroup`, `textarea` or `inlineEdit` whose node carries
`key` on `surface`. It lands on the next frame that draws that field. A key no field has on that frame, a surface
that is not the one on screen, or a surface that opens after the message was sent drops it, and the extension is
not told. An `inlineEdit` holds the keyboard on its own while it edits, so for one `focus` changes nothing.

While a field holds the keyboard, only shortcuts with ctrl fire; a plain key is typed. Escape hands the keyboard
back to the surface, and plain shortcuts fire again. So `/` to search is a shortcut whose handler answers with
`focus`:

```go
search := extension.Input.New(extension.InputProps{Placeholder: "Search", OnChange: "query"}).WithKey("search")
out(extension.Render.New(extension.RenderData{
	Surface:   "main",
	Root:      extension.Box.New(extension.BoxProps{}, search, results),
	Shortcuts: []extension.Shortcut{{Keys: "/", Handler: "find"}},
}))

// on the event "find":
out(extension.Focus.New(extension.FocusData{Surface: "main", Key: "search"}))
```

### Moving items between lists

A `sortableList` reorders its keyed children and sends `onReorder` with the keys in their new order. Lists on one
surface with the same non-empty `group` also exchange items, wherever they sit in the tree: a card dragged out of
one list travels over everything, and let go over another list of the group it fires that list's `onMove`. The
event carries the item's `key`, the `index` it landed at in the receiving list and, in `from`, the `key` of the
list it left; the list it left sends nothing. A drop over no list of the group sends nothing at all. While the item
is over another list of the group, lich draws that list's room as the drop target, the way the sidebar lights the
folder a session would be filed into; the list it left is never lit.

Nothing moves on screen until the extension renders again: lich keeps both lists as they were sent, so the
extension moves the item in its own data and answers with a render. Give every list of a group a `key`, and keep
item keys unique across the group. A list takes a drop over the room its box gives it, past its last item too, so
an empty column still takes one. A `scroll` gives its children no end along its axis, so a list in one takes a
drop down to the end of what the scroll shows. That room is measured from the start of the scroll's own child: put
the list straight into the `scroll`, and a column's header above the `scroll`, not inside it.

```go
column := func(id string, cards []string) extension.Node {
	items := make([]extension.Node, len(cards))
	for i, c := range cards {
		items[i] = extension.Label.New(extension.LabelProps{Text: c}).WithKey(c)
	}
	list := extension.SortableList.New(extension.SortableListProps{Group: "board", Gap: 4,
		OnReorder: extension.Handler("reorder:" + id), OnMove: extension.Handler("move:" + id)}, items...).WithKey(id)
	return extension.Box.New(extension.BoxProps{Width: 220}, list)
}
board := extension.Box.New(extension.BoxProps{Direction: extension.DirectionRow, Gap: 12},
	column("todo", todo), column("doing", doing), column("done", done))

// on the event "move:done": take e.Key out of the column e.From, insert it in
// done at e.Index, and render again.
```

### Dock and panel surfaces

A `dock` or `panel` surface opens from an item of the window's footer, the way Code and Review open the dock and
the branch opens the Git panel. lich puts each new item at the end of the footer's left side; the person can move
it or hide it in Settings › Appearance › Footer, and a hidden one stays hidden across updates. A dock surface is a
tab after lich's own, with its icon and title, and the dock is never narrower than its tabs. A panel surface takes
the Git panel's place over the footer, one panel at a time, each with the height the person dragged it to.

Both follow the session the window has active. lich sends `context` to an extension with such a surface when the
active session changes, when the extension starts and when a window attaches, whether the surface is open or not:

```json
{"v":1,"kind":"context","data":{"session":"…","project":"…","path":"/home/me/repo/.worktrees/fix-upload"}}
```

`path` is the active session's checkout, or the project's own folder with no session, and it is the key for both
things the extension sends about it:

- `footer` sets the item's text for that checkout: `{"surface":"issue","path":"…","text":"LICH-123"}`. The footer
  shows the text of the active checkout, else the one sent with no `path`, else the icon alone. An empty `text`
  clears it, and a text over 80 characters is refused. A text for a checkout lich never named in `context` is
  dropped, and lich forgets every text when the extension fails or is removed.
- `render` with `path` is a tree drawn for that checkout. While another checkout is active, the window shows a
  placeholder until the tree for that one arrives, never the tree of the session before. A `render` with no
  `path` is drawn for every checkout.

Answer `context` with the footer text and, when the surface is open, a render for the new path. A message sent
before a window attached is lost (see [ceilings](ceilings.md)), and `context` is what tells the extension one did:
send footer texts in answer to it, not once at start. In the dock or a panel only a chord with ctrl fires a
shortcut, and only while a field of the surface holds the keyboard: the terminal beside it keeps every other key.

## Calling lich

An extension reads and acts on lich over the same stdin and stdout, with no token and no network: lich knows which
extension asked because it started the process. A `call` names a method and carries its params; lich answers each
one with a `result` under the same id, carrying `data` or `error`. An unknown method, a permission the manifest
does not list, params that do not decode, a call past the bound and a call past the timeout are all error results,
never dropped lines:

```json
{"v":1,"kind":"call","data":{"id":"1","method":"sessions.send","params":{"session":"api-3","prompt":"run the tests"}}}
{"v":1,"kind":"result","data":{"id":"1","data":{"ticket":"t1","target":"api-3","status":"answered","answer":"green"}}}
{"v":1,"kind":"result","data":{"id":"2","error":"sessions.open needs the permission \"sessions:write\", which the extension's manifest does not list"}}
```

The methods mirror the tools lich gives agents over MCP and mean what they mean there. Their params and results are
the Go types in [`extension/methods.go`](../extension/methods.go), and `extension/testdata/methods.golden.json`
records each one's JSON. A call comes from no session, as `lich` run outside one does: nothing defaults to "your
own" project or provider.

| Method | Permission | Mirrors | Params | Result |
| --- | --- | --- | --- | --- |
| `projects.list` | `projects:read` | the projects the window holds | none | `[{id, name, path}]` |
| `worktrees.list` | `projects:read` | `list_worktrees` | `{project}`, required | `[{name, path, dirty, sessions}]` |
| `sessions.list` | `sessions:read` | `list_sessions` | none | `[{label, name, project, kind, state, id, path, projectPath, tool, detail, reason, focused}]` |
| `sessions.open` | `sessions:write` | `open_session` | `{project, worktree, provider, prompt}`, project required | `{id, projectId, project, label, name, kind, path, confined, task, taskError}` |
| `sessions.send` | `sessions:write` | `send_to_session` | `{session, project, prompt, waitSeconds}` | `{ticket, target, status, answer}` |
| `sessions.answer` | `sessions:write` | `wait_for_answer` with a ticket | `{ticket, waitSeconds}` | `{ticket, target, status, answer}` |
| `sessions.ask` | `sessions:ask` | `ask_session` | `{session, project, question}` | `{id, project, label, answer}` |
| `services.list` | `services:read` | `list_services` | `{project}`, required | `[{id, name, command, startWithLich, state, exitCode, startedAt, stoppedAt}]` |
| `services.start` | `services:write` | `start_service` | `{project, service}` | the service, as listed |
| `services.stop` | `services:write` | `stop_service` | `{project, service}` | the service, as listed |
| `services.restart` | `services:write` | `restart_service` | `{project, service}` | the service, as listed |
| `services.output` | `services:read` | `read_service_output` | `{project, service}` | `{output}` |
| `git.status` | `git:read` | a card's git line | `{path}` | `{branch, head, files, added, deleted, upstream, ahead, behind, base, behindBase, conflicts}` |
| `git.pullRequest` | `git:read` | a card's pull request badge | `{path}` | `{pullRequest: {number, url, title, head, base}}`, `null` when none is open |
| `notify` | `notify` | lich's desktop notification | `{title, body}` | `{}` |
| `window.showCode` | `window` | `show_code` | `{session, path, line, endLine, comment}` | `{session, path, line, endLine}` |
| `window.openURL` | `window` | `browser_open` | `{session, url}` | `{url, title}` |
| `events.subscribe` | each event's own | | `{names}` | `{}` |

`sessions.send` holds for a quick answer for `waitSeconds`, at most 90, which is also what 0 means; a session still
working then comes back `pending` with a ticket. `sessions.open` with a prompt hands it over as `open_session`
does, holding 20 seconds, and reports a hand-over that failed in `taskError` beside the session, which is open
either way. `sessions.answer` waits on such a ticket the same way, up to 90 seconds, and comes back `pending` again
while the session still works. It takes only a ticket this extension's own `sessions.send` or `sessions.open`
returned `pending`; any other ticket is refused, and one is spent once its outcome is anything but `pending`.

`sessions.ask` puts a side question to a running Claude Code session, which answers from its own conversation
without stopping its turn, as `ask_session` does: other providers are refused, and so is a session without the
lich-plugin mod. It waits up to 90 seconds and has no ticket: a session that has not answered by then is an error,
and its later answer is dropped. Ask for a brief answer.

A service is named by its project and its name, matched ignoring case; an action answers with the service as it
is right after, so `services.stop` may still read `stopping`. `services.output` is one read of the console, terminal
escapes included: what a running service has printed so far, or what a stopped one printed in its last run.

`git.status` and `git.pullRequest` name a checkout by its directory and read what a card shows of it: the branch,
uncommitted changes, the commits ahead of and behind its upstream as `git status` counts them (no fetch), its
standing against the base branch, and the open pull request of the branch. The pull request
comes from lich's own watch when it has read that checkout, and from one `gh` read otherwise.

`notify` raises a desktop notification the way lich raises its own, title over body, each cut at 120 characters,
at most 3 a minute per extension; one past that is refused. `window.showCode` opens a file of a session's checkout
in the window's dock, at a line or a range, as `show_code` does. `window.openURL` loads an http or https page in a
session's browser, the page beside that session's terminal, as `browser_open` does. Both name the session by its id, and both
are refused when no lich window is open.

Two events: `session.status`, a session's new state (`{id, state, tool, detail, reason}`), with the permission
`events:session`, and `service.status`, a service that started, stopped or failed (`{id, name, projectId, state,
exitCode}`), with `services:read`. Subscribing checks every name first and starts none when one is refused. A
restarted extension starts with no subscriptions.

| Permission | Allows |
| --- | --- |
| `projects:read` | `projects.list`, `worktrees.list` |
| `sessions:read` | `sessions.list` |
| `sessions:write` | `sessions.open`, `sessions.send`, `sessions.answer` |
| `sessions:ask` | `sessions.ask` |
| `events:session` | subscribing to `session.status` |
| `services:read` | `services.list`, `services.output`, subscribing to `service.status` |
| `services:write` | `services.start`, `services.stop`, `services.restart` |
| `git:read` | `git.status`, `git.pullRequest` |
| `notify` | `notify` |
| `window` | `window.showCode`, `window.openURL` |

lich runs at most 8 calls of one extension at once, refusing the ninth, and gives each 2 minutes.

### The Go SDK

`extension.Conn` is the loop an extension would otherwise write: it correlates call ids, decodes results into their
types, delivers every other message from lich on one channel, `Messages()`, in order, and writes from any
goroutine. Every call takes a context and returns when it ends. Results never wait behind `Messages()`, so a
handler may call lich on the goroutine reading it. `Messages()` closes when lich closes stdin: exit then.

```go
conn := extension.NewConn(os.Stdin, os.Stdout)
ctx := context.Background()

sessions, err := conn.Sessions(ctx)
if err != nil {
	log.Fatal(err)
}
for _, s := range sessions {
	if s.State == "done" {
		_, err := conn.SendToSession(ctx, extension.SendParams{Session: s.ID, Prompt: "run the tests"})
		if err != nil {
			log.Print(err)
		}
		break
	}
}

if err := conn.Subscribe(ctx, extension.SessionStatus.Name); err != nil {
	log.Fatal(err)
}
for msg := range conn.Messages() {
	switch data := msg.Data.(type) {
	case extension.LichEventData:
		status, err := extension.SessionStatus.Decode(data)
		if err == nil && status.State == "waiting" {
			_ = conn.Send(extension.ShowToast.New(extension.ToastData{Title: "A session is waiting on you"}))
		}
	case extension.SurfaceData:
		// surface.opened or surface.closed: msg.Kind says which
	}
}
```

The manifest of this one lists `sessions:read`, `sessions:write` and `events:session`. Asking a session and printing its
answer needs only `sessions:ask`:

```go
answer, err := conn.Ask(ctx, extension.AskParams{Session: "api-3", Question: "Are the tests green? One line."})
if err != nil {
	log.Fatal(err)
}
fmt.Println(answer.Label, "says:", answer.Answer)
```

`extension.Call(ctx, conn, method, params)` calls any declared method; the helpers on `Conn` are that call for each one.

## Keeping data

An extension that saves something, a token or its settings, keeps it in its data directory:
`<config dir>/lich/extension-data/<id>` (`extension-data-dev` under `LICH_DEV`). lich creates it, readable only by
the user, before every start and passes its absolute path in `LICH_EXTENSION_DATA`, named in Go as
`extension.DataDirEnv`. Never write into the extension's own directory: an update replaces that directory whole,
and so does a remove and an install. The data directory survives both, so the next version of the same id finds
what the last one saved. It goes only when the person ticks "Also delete what it saved" while removing the
extension.

```go
dir := os.Getenv(extension.DataDirEnv)
token, err := os.ReadFile(filepath.Join(dir, "token"))
if errors.Is(err, fs.ErrNotExist) {
	// first run: nothing saved yet
}
err = os.WriteFile(filepath.Join(dir, "token"), token, 0o600)
```

## Translating an extension

An extension translates itself with the catalogs lich uses for its own screens, in the same format. Ship them
beside the manifest:

```
locales/en/extension.json
locales/en/board.json
locales/pt-br/extension.json
locales/pt-br/board.json
```

- A folder is a BCP 47 tag (`en`, `pt-br`, `zh-cn`; case does not matter) and a file is a namespace, a tree of
  JSON objects read by a dotted key: `board.json` holding `{"empty": {"title": "No cards"}}` is the key
  `board.empty.title`. English is the source: write `en` first and every other language from it.
- `{name}` in a message is a placeholder, filled by name. `{` is always one.
- An object holding `other` is a plural, its keys CLDR forms picked by the int `count`:
  `{"one": "{count} card", "other": "{count} cards"}`. lich tells apart the forms its own languages use: `one`
  for English and Spanish (1) and for Portuguese (0 and 1), `one`, `few` and `many` for Russian, and only
  `other` for Chinese, Japanese and Korean. Every other language reads `other`. Because of this rule, no key in a
  catalog can be named `other` unless its object is a plural.
- lich reads the name and each surface's title from the namespace `extension`: `manifest.name` and
  `manifest.surfaces.<surface id>.title`, so `locales/pt-br/extension.json` is
  `{"manifest": {"name": "Quadro", "surfaces": {"main": {"title": "Quadro"}}}}`. The top bar and Settings show
  the one for the interface's language, then English's, then the manifest's own text. A plural there is ignored.
- Discovery and install read `locales/` and list the extension as invalid, with the reason, when a catalog is
  not valid JSON or holds something other than strings and objects, when anything inside `locales/` is a link,
  or past 64 entries in `locales/`, 512 catalogs or 8 MiB of them. Only `.json` files in a language's folder are
  read.

lich does not translate anything else for the extension: toasts and the tree are the extension's to look up.
It tells the extension the language its interface is in three ways:

- `LICH_LANGUAGE` (`extension.LanguageEnv`) at start: the language chosen in Settings, or before one is chosen
  the language of the environment's locale. On macOS and Windows the system's language is not in the
  environment, so there an unset choice starts as `en` until the window says otherwise.
- `language` with `{"tag": "pt-BR"}`, to every running extension when the person changes the language, when an
  extension starts while a window is open, and when a window opens.
- `language` in every `surface.opened`.

The tag is one lich ships: `en`, `pt-BR`, `es`, `zh-CN`, `ja`, `ko` or `ru`.

`github.com/lichdotdev/lich/extension/i18n` reads the catalogs, with lich's own lookup and plural rules.
`OpenDir` loads `locales/`, `In(tag, key, args)` answers in a language, falling back to English and then to the
key itself, and `Language` tracks the current tag:

```go
import (
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/extension/i18n"
)

cat, err := i18n.OpenDir(".") // lich starts the extension in its own directory
if err != nil {
	log.Fatal(err)
}
lang := i18n.FromEnv()

// before any surface opens:
out(extension.ShowToast.New(extension.ToastData{Title: cat.In(lang.Tag(), "board.synced")}))

// for each message from lich:
switch d := msg.Data.(type) {
case extension.SurfaceData:
	lang.Set(d.Language) // empty on surface.closed, which Set ignores
case extension.LanguageData:
	lang.Set(d.Tag)
	// render the open surface again, in the new language
}

title := cat.In(lang.Tag(), "board.count", i18n.Args{"count": len(cards)})
```

## What lich enforces

- A line over 3 MiB, JSON nested over 512 levels, a tree over 5000 nodes or 64 deep, a kind that only travels
  from lich to the extension, and `openExternal` with a URL that is not http or https are dropped and logged. The
  extension is not told.
- `render` is coalesced to 30 a second per surface; the newest tree always arrives.
- A `footer` text over 80 characters is refused.
- At most 8 calls run at once per extension, each for at most 2 minutes, and a call needs the permission its
  method names in the manifest. A `result` or `lich.event` lich cannot queue because the extension stopped reading
  its stdin is dropped and logged.
- Any exit lich did not ask for is a crash, exit 0 included. lich restarts after 1 second, doubling up to 30, and
  gives up after 5 crashes in a row each within 30 seconds of its start. A process that starts while its surface
  is on screen is sent `context` and `surface.opened` for it again, since it never saw the first ones.
- To stop an extension lich closes its stdin and kills it 3 seconds later. Exit when stdin ends.
- An extension runs unsandboxed, as the user. See the Extensions section of [ceilings](ceilings.md).

## Installing

A person installs an extension from lich's Settings in one of two ways:

- **From a folder** on disk holding `manifest.json` and the executable at its root. lich copies it into a staging
  directory under the extensions directory (at most 1 GiB and 1000 entries, no symlinks anywhere in it), validates
  the copy as it does at startup and marks the executable runnable. The copy is what runs; the folder is left
  untouched.
- **From GitHub**, by pasting the repository URL, as below.

Either way the install is two steps, and nothing runs until the second. The first,
`extensions.PrepareFromDir` or `extensions.PrepareFromGit`, does all of the above and stops, returning the staged
copy's id, name, version, source, release and `permissions` under a token. lich then asks: the confirmation names
the extension and its version, where it comes from, that it runs as the person with access to their files and
accounts, and, under **It can also**, one line per permission the manifest lists, in lich's own words rather than
the extension's, with `sessions:write` set apart. An extension that lists none shows no list. Install sends the
token to `extensions.CommitInstall`, which moves the copy into place and starts it; Cancel sends it to
`extensions.DiscardInstall`, which deletes it. A token commits or discards once, and a staged copy nobody answers
is deleted after 15 minutes, or when lich next starts. The installed extension's status in Settings lists the same
permission lines on hover.

An id that is already installed is refused, at both steps.

## Updating

lich asks GitHub for the latest release of every extension installed from a repository when it starts and every
six hours after, and on `extensions.CheckUpdates`. A latest release whose tag is a later version than the one
installed is an update: the extension's `latest` carries the tag, and `extension-update` tells the window when that
changes. Tags are compared as `MAJOR.MINOR.PATCH`, with or without a leading `v`, so tag releases that way: a
repository whose tags are not versions is never offered as an update. A repository the
[catalog](#what-lich-does-with-a-listed-repository) lists is compared with the catalog's `version` instead, with no
request to GitHub, and its `latest` is the catalog's `tag`.

`extensions.PrepareUpdate` stages that release exactly as an install does, same checks, and returns the same
preview under a token, while the installed version keeps running. `extensions.CommitInstall` with that token stops
the installed version, moves its directory aside, moves the staged copy into its place and starts it; if the move
fails the old directory goes back and starts again. The [data directory](#keeping-data) is not touched. An update
has to ship the same id as the extension it replaces.

The window offers an update in a toast, once per release: Later puts that release off for good, and the row in
Settings > Extensions keeps its Update button either way. An update whose manifest lists only permissions the
installed version already has is installed on the click. One that lists a new permission opens the install
confirmation first, with the new lines marked. One installed from a folder or placed by hand has no
release to compare, and is updated by removing it and installing it again.

## Publishing a release

For an install from GitHub, the URL is `https://github.com/<owner>/<repo>` (with or without `.git`). lich installs the repository's latest release: the newest one that is neither a
draft nor a pre-release. That release carries:

- One archive per OS and architecture, named `<id>_<goos>_<goarch>.tar.gz` with Go's names (`linux_amd64`,
  `linux_arm64`, `darwin_arm64`, `darwin_amd64`, `windows_amd64`, ...). Each holds `manifest.json` and the
  executable at its root, with no top-level folder. Only regular files and directories are accepted: no symlinks,
  hard links or devices, no absolute paths and no `..`. The manifest's `id` must equal the archive's `<id>`.
- `checksums.txt` in the `sha256sum` format, one `<hex>  <file name>` line per archive.

lich downloads the archive for the machine it runs on and `checksums.txt`, refuses an archive whose sha256 does not
match, unpacks it, validates the manifest as it does at startup and marks the executable runnable, then asks as
[above](#installing) before it starts it. An
archive is at most 256 MiB compressed, 1 GiB and 1000 entries unpacked.

## Listing in the catalog

The catalog is the list of extensions the site and lich's Settings show. It lives in
[`lichdotdev/extensions`](https://github.com/lichdotdev/extensions), whose build reads every listed repository's
latest release and publishes the result as `catalog.json`, the `extension.Catalog` type in
[`extension/catalog.go`](../extension/catalog.go), at
`https://raw.githubusercontent.com/lichdotdev/extensions/main/catalog.json`.

Before asking, check every archive the way lich will: `lich extension check dist/<id>_linux_amd64.tar.gz` runs the
install's checks on an archive or an extension's folder without installing anything, and exits non-zero naming
what fails.

A repository is listed when:

- It is public on GitHub, neither a fork nor archived, and its latest release is tagged `MAJOR.MINOR.PATCH` (with
  or without a leading `v`) and published as [above](#publishing-a-release). A platform without an archive is
  listed as unavailable there.
- Every archive passes `lich extension check`, and the manifest sets `description`.
- Its `id` is not listed from another repository. An `id` starting with `lich` is kept for `lichdotdev`.

To ask, open a pull request on `lichdotdev/extensions` adding `"<owner>/<repo>"` to `extensions.json`. Its checks
run on your latest release and say what fails. A maintainer approves the first listing; after that each new release
is picked up within an hour, by the same checks and no person.

### What the catalog shows

The name and description come from the manifest, translated from `manifest.name` and `manifest.description` in
`locales/<tag>/extension.json`. The rest comes from `listing/`, a folder at the root of the repository, read at the
commit the release tag points to, never from the archive:

```
listing/
  listing.json
  cover.png
  screenshots/board.png
```

```json
{
  "categories": ["issues", "git"],
  "cover": "cover.png",
  "screenshots": [{ "file": "screenshots/board.png", "alt": "Board with issues grouped by state" }]
}
```

- `categories` names one or two of the catalog's: `agents` (driving and watching sessions), `git` (git and pull
  requests), `issues` (issue trackers and boards), `notes` (notes and documents), `services` (dev servers and logs)
  and `tools` (anything else). A new category is a pull request on the catalog, with its name in every language
  lich ships.
- `cover` is optional: PNG or JPEG, at most 1 MiB, 16:10, from 1600 × 1000 to 3200 × 2000 pixels.
- `screenshots` is optional, at most 6: PNG or JPEG, at most 2 MiB each, 16:10, at least 1280 × 800 pixels. `alt`
  is required, at most 200 characters: it is what a screen reader reads instead of the picture.
- Paths are relative to `listing/` and stay inside it.

A repository without `listing/listing.json` is listed under `tools`, with no cover or screenshots. The stars shown
are the repository's on GitHub.

### What the catalog holds to

The first time the catalog sees a version it records the sha256 of each archive, and that record never changes: an
archive replaced afterwards holds the extension at the last version that matched, with the reason in the entry's
`held`. So does a repository whose GitHub id changed, renamed, transferred or deleted and created again, until a
maintainer confirms it. A repository taken down for malware, deception or infringement leaves the list and is
named in `revoked`.

### What lich does with a listed repository

An install or update from a repository the catalog lists takes the catalog's `tag`, not GitHub's latest release, and
refuses an archive whose sha256 differs from the catalog's, on top of the release's `checksums.txt`; a platform the
entry has no `builds` hash for is refused, naming it. A newer release reaches lich once the catalog lists it. lich
reads the catalog when it starts and every six hours after, with the update check, and before an install or update
while it holds none. When the catalog cannot be read, lich installs as it does an unlisted repository. An installed
extension whose repository is revoked is stopped when lich next reads the catalog, listed with the reason
(`removed from the lich catalog: <reason>`), and never started again, nor updated; Remove still works and keeps its
data. An install of a revoked repository is refused with the reason.
