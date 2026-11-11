package extension

var (
	Markdown = component[MarkdownProps]("markdown")
	CodeView = component[CodeViewProps]("codeView")
	DiffView = component[DiffViewProps]("diffView")
	FileTree = component[FileTreeProps]("fileTree")
)

// MarkdownProps draws Text as Markdown. OnOpen carries the URL of a clicked
// link in EventData.Text.
type MarkdownProps struct {
	Text   string  `json:"text"`
	OnOpen Handler `json:"onOpen,omitempty"`
}

// CodeBlock draws Content under a line of a CodeView, After counted from 0.
type CodeBlock struct {
	After   int  `json:"after"`
	Content Node `json:"content"`
}

// CodeViewProps draws Text as source code, highlighted by the extension of
// Filename. It fills the room its parent gives it. Size is in sp.
type CodeViewProps struct {
	Filename string      `json:"filename,omitempty"`
	Text     string      `json:"text"`
	Size     float64     `json:"size,omitempty"`
	Blocks   []CodeBlock `json:"blocks,omitempty"`
}

// DiffBlock draws Content under a line of a DiffView, Line counted from 1 in
// the file Side names. Key keeps the block's height across updates.
type DiffBlock struct {
	Key     string   `json:"key"`
	Side    DiffSide `json:"side,omitempty"`
	Line    int      `json:"line"`
	Content Node     `json:"content"`
}

// DiffViewProps draws the first file of Patch, a unified diff. Height is the
// diff's own height without its blocks, which add theirs.
type DiffViewProps struct {
	Patch  string      `json:"patch"`
	Split  bool        `json:"split,omitempty"`
	Height float64     `json:"height"`
	Size   float64     `json:"size,omitempty"`
	Blocks []DiffBlock `json:"blocks,omitempty"`
}

// FileStat is the lines a file added and deleted. A folder's total is the sum
// of the files under it, so only files are named.
type FileStat struct {
	Added   int `json:"added,omitempty"`
	Deleted int `json:"deleted,omitempty"`
}

// FileTreeProps draws Paths, slash-separated and relative, as a tree. Stats
// is keyed by path. A path in ClosedPaths is a folder drawn shut. OnSelect
// carries a clicked file's path in EventData.Text, OnToggle a folder's.
type FileTreeProps struct {
	Paths       []string            `json:"paths"`
	Stats       map[string]FileStat `json:"stats,omitempty"`
	Selected    string              `json:"selected,omitempty"`
	ClosedPaths []string            `json:"closedPaths,omitempty"`
	OnSelect    Handler             `json:"onSelect,omitempty"`
	OnToggle    Handler             `json:"onToggle,omitempty"`
}
