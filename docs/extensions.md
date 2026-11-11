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
  "surfaces": [{ "id": "main", "title": "Hello", "icon": "hand" }]
}
```

- `formatVersion` is a positive integer no newer than this lich reads (`extension.ManifestFormat`).
- `id` is lowercase letters, digits, dots, underscores or dashes, starts with a letter or digit, is at most 64
  long, and equals the directory name.
- `minLichVersion`, when set, is `MAJOR.MINOR.PATCH`; an older lich lists the extension as invalid.
- `executable` is a path relative to the directory and must resolve inside it, symlinks included, to a regular
  file. On Windows name the `.exe`.
- Surface ids are non-empty and unique. A surface is opened from the window's top bar.

An extension that fails these checks is listed with its error and never started.

## The protocol

lich starts the executable with the extension's directory as its working directory and lich's environment. Each
line on stdout is one JSON message; each line lich writes to stdin is one too. The Go types, the components and
the JSON of each are in [`extension/`](../extension/), and `extension/testdata/` records the wire form of every
component and message. Import `github.com/lichdotdev/lich/extension` rather than writing the JSON by hand.

- From the extension: `render` (a surface's tree and shortcuts), `toast`, `toast.dismiss`, `openExternal`,
  `clipboard` and `focus`.
- From lich: `surface.opened`, `surface.closed`, `event` (a handler the tree named fired) and `language` (see
  Translating an extension).

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
- Any exit lich did not ask for is a crash, exit 0 included. lich restarts after 1 second, doubling up to 30, and
  gives up after 5 crashes in a row each within 30 seconds of its start.
- To stop an extension lich closes its stdin and kills it 3 seconds later. Exit when stdin ends.
- An extension runs unsandboxed, as the user. See the Extensions section of [ceilings](ceilings.md).

## Installing

A person installs an extension from lich's Settings in one of two ways:

- **From a folder** on disk holding `manifest.json` and the executable at its root. lich copies it into the
  extensions directory (at most 1 GiB and 1000 entries, no symlinks anywhere in it), validates the copy as it does
  at startup, marks the executable runnable and starts it. The copy is what runs; the folder is left untouched.
- **From GitHub**, by pasting the repository URL, as below.

Either way an id that is already installed is refused; to update, remove the extension and install it again. The
remove keeps its [data directory](#keeping-data), so the new install picks up what the old one saved.

## Publishing a release

For an install from GitHub, the URL is `https://github.com/<owner>/<repo>` (with or without `.git`). lich installs the repository's latest release: the newest one that is neither a
draft nor a pre-release. That release carries:

- One archive per OS and architecture, named `<id>_<goos>_<goarch>.tar.gz` with Go's names (`linux_amd64`,
  `linux_arm64`, `darwin_arm64`, `darwin_amd64`, `windows_amd64`, ...). Each holds `manifest.json` and the
  executable at its root, with no top-level folder. Only regular files and directories are accepted: no symlinks,
  hard links or devices, no absolute paths and no `..`. The manifest's `id` must equal the archive's `<id>`.
- `checksums.txt` in the `sha256sum` format, one `<hex>  <file name>` line per archive.

lich downloads the archive for the machine it runs on and `checksums.txt`, refuses an archive whose sha256 does not
match, unpacks it, validates the manifest as it does at startup, marks the executable runnable and starts it. An
archive is at most 256 MiB compressed, 1 GiB and 1000 entries unpacked.
