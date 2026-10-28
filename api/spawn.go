package api

var (
	SpawnOpen         = define[OpenSessionOptions, OpenedSession]("spawn.Open")
	SpawnOpenSubagent = define[OpenSubagentOptions, OpenedSession]("spawn.OpenSubagent")
	SpawnClose        = define[CloseSessionOptions, ClosedSession]("spawn.Close")
	SpawnCloseProject = define[CloseProjectOptions, ClosedProject]("spawn.CloseProject")
	SpawnControl      = define[ControlSessionOptions, ControlledSession]("spawn.Control")
	SpawnAsk          = define[AskSessionOptions, SessionAnswer]("spawn.Ask")
	SpawnWorktrees    = define[WorktreesOptions, []WorktreeCheckout]("spawn.Worktrees")
	SpawnFolders      = define[FoldersOptions, []SessionFolder]("spawn.Folders")
	SpawnFile         = define[FileSessionOptions, FiledSession]("spawn.File")
	SpawnRenameFolder = define[RenameFolderOptions, RenamedFolder]("spawn.RenameFolder")
	SpawnColorFolder  = define[ColorFolderOptions, ColoredFolder]("spawn.ColorFolder")
	SpawnRename       = define[RenameSessionOptions, RenamedSession]("spawn.Rename")
)

// OpenSessionOptions is one Open call. It travels as a single JSON object rather than
// as positional arguments because a package manager replaces the lich binary
// under a running backend: a newer CLI's option the backend does not know is
// dropped, and one an older CLI does not send is its zero value, where a
// positional list refuses both on its count.
//
// From is the session the caller runs in, empty for the command line run
// outside one. It decides two defaults: the project the session lands in and the
// provider it runs, both taken from the caller unless named. A caller with no
// session of its own must name the project — there is nothing to inherit. It is
// also recorded on the row as the new session's origin, which outlives the
// request that created it: the card says where it came from for as long as it
// exists.
//
// Project is a project already on screen, by name, or a directory path — and
// a path is the only way to reach a project the window is not holding, which is
// what opening one from the command line means. See ensureProject.
//
// Worktree, when given, is the branch name of a new git worktree created off
// Base (the project's current branch when Base is empty); the session is rooted
// there and labelled after it, exactly as the window's own worktree flow does. A
// Base without a Worktree has nothing to branch, and is refused rather than ignored.
//
// Model, when given, is the model the provider is spawned on, in that provider's
// own spelling. It is recorded on the row so every later spawn repeats it.
// Effort, when given, is the reasoning effort, passed and recorded the same way.
// Ultracode turns Claude Code's ultracode on, at whatever effort the session
// runs; it is recorded the same way, and refused for every other provider.
//
// Folder, when given, is the folder the session is filed under from its first
// frame: it is written with the row, so the card arrives in the folder's block
// rather than among its checkout's cards and then jumps.
type OpenSessionOptions struct {
	From      string `json:"from"`
	Project   string `json:"project"`
	Kind      string `json:"kind"`
	Worktree  string `json:"worktree"`
	Base      string `json:"base"`
	Model     string `json:"model"`
	Effort    string `json:"effort"`
	Folder    string `json:"folder"`
	Ultracode bool   `json:"ultracode"`
}

// OpenSubagentOptions is one OpenSubagent call, an object for the reason
// OpenOptions is one. Its fields mean what Open's do; there is no project or
// folder, both of which are the caller's.
type OpenSubagentOptions struct {
	From      string `json:"from"`
	Kind      string `json:"kind"`
	Worktree  string `json:"worktree"`
	Base      string `json:"base"`
	Model     string `json:"model"`
	Effort    string `json:"effort"`
	Ultracode bool   `json:"ultracode"`
}

// OpenedSession is one opened session: what the caller is told, and what the window is
// handed to draw the card. Both read the same struct because both need the same
// facts — a divergence between them is a card that does not match the session
// the agent was given.
//
// Name is the second name the session answers to, empty for a Claude Code one,
// which names itself (relay.RosterNameOf); Label is the name on its card. Both
// address it (see internal/relay). Path is the worktree
// the session lives in, empty for a session in the project's own directory —
// the store's own spelling. NextSeq is the project's label counter after this
// session took its number. OriginSessionID and OriginLabel name the session that
// asked for this one — empty when the caller was not a session at all.
// Confined is whether the session opened inside the sandbox. Nobody is at this
// end to be asked, so it is the rung's own answer (store.SandboxDefault), and
// saying it is what keeps a caller from discovering the empty home and the
// read-only machine by running into them. Folder is the folder it was filed
// under, "" for none.
type OpenedSession struct {
	ID              string `json:"id"`
	ProjectID       string `json:"projectId"`
	Project         string `json:"project"`
	Label           string `json:"label"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Path            string `json:"path"`
	NextSeq         int    `json:"nextSeq"`
	OriginSessionID string `json:"originSessionId"`
	OriginLabel     string `json:"originLabel"`
	Confined        bool   `json:"confined"`
	Folder          string `json:"folder"`
}

// CloseSessionOptions is one Close call, an object for the reason OpenOptions is one:
// an option a newer lich adds is dropped by an older backend, and one an older
// lich does not send is its zero value.
type CloseSessionOptions struct {
	From     string `json:"from"`
	Target   string `json:"target"`
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Force    bool   `json:"force"`
	Self     bool   `json:"self"`
}

// ClosedSession is what a caller is told about the session it closed.
type ClosedSession struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Project   string `json:"project"`
	Label     string `json:"label"`
	// Worktree is the checkout the session lived in, empty for a session in the
	// project's own directory.
	Worktree string `json:"worktree"`
	// Kept is the answer given to the worktree question: the checkout was left on
	// disk rather than removed. Meaningless unless the close had that question to
	// answer — a session sharing its checkout, or living in the project's own
	// directory, reports false while leaving everything where it was.
	//
	// Nothing here reports the park, because every close but a removal parks and
	// a field that is true on all of them tells a caller nothing.
	Kept bool `json:"kept"`
	// Removed is whether the checkout was deleted.
	Removed bool `json:"removed"`
}

// CloseProjectOptions is one CloseProject call, an object for the reason
// CloseSessionOptions is one. Project is an open project by name or by
// directory path; From is the caller's session, whose own project is refused.
type CloseProjectOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
}

// ClosedProject is what a caller is told about the project it closed.
type ClosedProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	// Sessions is how many sessions the project held: their terminals were
	// stopped, and their rows stay for the reopen.
	Sessions int `json:"sessions"`
}

// ControlSessionOptions is one Control call, an object for the reason CloseOptions
// is one.
type ControlSessionOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
	Action  string `json:"action"`
	Value   string `json:"value"`
	Args    string `json:"args"`
}

// ControlledSession is what a caller is told about a command it gave a session. Value
// is the prompt, model, effort or slash command's name the action carried, and
// CommandID the id the session's mod acks it under.
type ControlledSession struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Label     string `json:"label"`
	Action    string `json:"action"`
	Value     string `json:"value,omitempty"`
	CommandID string `json:"command_id"`
	State     string `json:"state"`
}

// AskSessionOptions is one Ask call, an object for the reason CloseOptions is one.
type AskSessionOptions struct {
	From     string `json:"from"`
	Target   string `json:"target"`
	Project  string `json:"project"`
	Question string `json:"question"`
}

// SessionAnswer is a session's answer to a side question.
type SessionAnswer struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Label   string `json:"label"`
	Answer  string `json:"answer"`
}

// WorktreesOptions is one Worktrees call, an object for the reason CloseOptions
// is one.
type WorktreesOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
}

// WorktreeCheckout is one of a project's git worktrees, as a caller outside the window
// needs to see it: what it is called, where it is, whether anything in it is
// uncommitted, and which sessions are open in it.
//
// Sessions is what makes the list actionable rather than informational. A
// checkout with none is one nobody is working in; the last session in one is the
// thing whose closing decides whether the checkout stays (see Close).
type WorktreeCheckout struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Dirty    bool     `json:"dirty"`
	Sessions []string `json:"sessions"`
}

// FoldersOptions is one Folders call, an object for the reason CloseOptions is
// one.
type FoldersOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
}

// SessionFolder is one of a project's folders as a caller outside the window sees it:
// its name, which is its whole identity (store.SetSessionFolder), and the
// sessions filed under it, by label.
type SessionFolder struct {
	Name     string   `json:"name"`
	Sessions []string `json:"sessions"`
}

// FileSessionOptions is one File call, an object for the reason CloseOptions is one.
type FileSessionOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
	Folder  string `json:"folder"`
}

// FiledSession is what a caller is told about the session it filed. Previous is the
// folder it left, "" when it was in none.
type FiledSession struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Label    string `json:"label"`
	Folder   string `json:"folder"`
	Previous string `json:"previous"`
}

// RenameFolderOptions is one RenameFolder call, an object for the reason
// CloseOptions is one. Folder is the name to change and To the new one.
type RenameFolderOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
	Folder  string `json:"folder"`
	To      string `json:"to"`
}

// RenamedFolder is what a caller is told about a folder it renamed or took apart:
// every session that moved, which is the part a merge into an existing folder
// would otherwise hide.
type RenamedFolder struct {
	Project  string   `json:"project"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Sessions []string `json:"sessions"`
}

// ColorFolderOptions is one ColorFolder call, an object for the reason
// CloseOptions is one.
type ColorFolderOptions struct {
	From    string `json:"from"`
	Project string `json:"project"`
	Folder  string `json:"folder"`
	Color   string `json:"color"`
}

// ColoredFolder is what a caller is told about a folder it painted: the sessions that
// took the colour, by label.
type ColoredFolder struct {
	Project  string   `json:"project"`
	Folder   string   `json:"folder"`
	Color    string   `json:"color"`
	Sessions []string `json:"sessions"`
}

// RenameSessionOptions is one Rename call, an object for the reason CloseOptions is
// one.
type RenameSessionOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
	Label   string `json:"label"`
}

// RenamedSession is what a caller is told about the session it renamed. Previous is
// the name that is gone, which is the half the caller cannot look up afterwards.
type RenamedSession struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Label    string `json:"label"`
	Previous string `json:"previous"`
}
