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
  "surfaces": [{ "id": "main", "title": "Hello", "icon": "hand" }],
  "permissions": ["sessions:read"]
}
```

- `formatVersion` is a positive integer no newer than this lich reads (`extension.ManifestFormat`).
- `id` is lowercase letters, digits, dots, underscores or dashes, starts with a letter or digit, is at most 64
  long, and equals the directory name.
- `minLichVersion`, when set, is `MAJOR.MINOR.PATCH`; an older lich lists the extension as invalid.
- `executable` is a path relative to the directory and must resolve inside it, symlinks included, to a regular
  file. On Windows name the `.exe`.
- Surface ids are non-empty and unique. A surface is opened from the window's top bar.
- `permissions` lists what the extension's [calls](#calling-lich) use, each one of `extension.Permissions`. A name
  this lich does not know makes the manifest invalid.

An extension that fails these checks is listed with its error and never started.

## The protocol

lich starts the executable with the extension's directory as its working directory and lich's environment. Each
line on stdout is one JSON message; each line lich writes to stdin is one too. The Go types, the components and
the JSON of each are in [`extension/`](../extension/), and `extension/testdata/` records the wire form of every
component and message. Import `github.com/lichdotdev/lich/extension` rather than writing the JSON by hand.

- From the extension: `render` (a surface's tree and shortcuts), `toast`, `toast.dismiss`, `openExternal`,
  `clipboard`, `focus` and `call` (see [Calling lich](#calling-lich)).
- From lich: `surface.opened`, `surface.closed`, `event` (a handler the tree named fired), `language` (see
  Translating an extension), `result` (the answer to a `call`) and `lich.event` (an event the extension subscribed
  to).

Stderr goes to lich's log.

A `box` lays its children along its axis. A child `box` with `grow` takes that share of the room left; a `scroll`,
`thinScroll`, `splitPane` or `emptyState` takes the room left on its own, and an empty state sits centered in it.

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
list it left; the list it left sends nothing. A drop over no list of the group sends nothing at all.

Nothing moves on screen until the extension renders again: lich keeps both lists as they were sent, so the
extension moves the item in its own data and answers with a render. Give every list of a group a `key`, and keep
item keys unique across the group. A list takes a drop over the room its box gives it, past its last item too, so
an empty column still takes one.

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
session's browser, the tab beside that session, as `browser_open` does. Both name the session by its id, and both
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
`extension.DataDirEnv`. Never write into the extension's own directory: an update is a remove and an install, and
the install replaces that directory whole. The data directory survives both, so the next install of the same id
finds what the last one saved. It goes only when the person ticks "Also delete what it saved" while removing the
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
- At most 8 calls run at once per extension, each for at most 2 minutes, and a call needs the permission its
  method names in the manifest. A `result` or `lich.event` lich cannot queue because the extension stopped reading
  its stdin is dropped and logged.
- Any exit lich did not ask for is a crash, exit 0 included. lich restarts after 1 second, doubling up to 30, and
  gives up after 5 crashes in a row each within 30 seconds of its start.
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

An id that is already installed is refused, at both steps; to update, remove the extension and install it again.
The remove keeps its [data directory](#keeping-data), so the new install picks up what the old one saved.

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
