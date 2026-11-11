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

- From the extension: `render` (a surface's tree and shortcuts), `toast`, `toast.dismiss`, `openExternal` and
  `clipboard`.
- From lich: `surface.opened`, `surface.closed` and `event` (a handler the tree named fired).

Stderr goes to lich's log.

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

Either way an id that is already installed is refused; to update, remove the extension and install it again.

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
