package api

var (
	ProjectGitLog        = define[GitLogOptions, GitLogPage]("project.GitLog")
	ProjectGitAuthors    = define[GitAuthorsOptions, []string]("project.GitAuthors")
	ProjectGitRefs       = define[PathOptions, GitRefs]("project.GitRefs")
	ProjectGitCommit     = define[GitCommitOptions, GitCommitDetail]("project.GitCommit")
	ProjectGitCommitDiff = define[GitCommitDiffOptions, string]("project.GitCommitDiff")

	ProjectGitCheckout     = define[GitCheckoutOptions, None]("project.GitCheckout")
	ProjectGitCreateBranch = define[GitCreateBranchOptions, None]("project.GitCreateBranch")
	ProjectGitRenameBranch = define[GitRenameBranchOptions, None]("project.GitRenameBranch")
	ProjectGitBranchOnly   = define[GitBranchOptions, int]("project.GitBranchOnly")
	ProjectGitDeleteBranch = define[GitBranchOptions, GitDeletedBranch]("project.GitDeleteBranch")

	ProjectGitMergePreview = define[GitMergeOptions, GitMergePreview]("project.GitMergePreview")
	ProjectGitMerge        = define[GitMergeOptions, GitMergeResult]("project.GitMerge")
	ProjectGitMergeUndo    = define[GitMergeUndoOptions, None]("project.GitMergeUndo")
	ProjectGitConflict     = define[CheckoutFileOptions, GitConflict]("project.GitConflict")
	ProjectGitResolveFile  = define[GitResolveFileOptions, None]("project.GitResolveFile")
	ProjectGitMergeAbort   = define[PathOptions, None]("project.GitMergeAbort")
	ProjectGitMergeCommit  = define[PathOptions, None]("project.GitMergeCommit")
	ProjectGitMarkResolved = define[CheckoutFileOptions, None]("project.GitMarkResolved")
)

// A GitRef's Kind, read from its full name: a remote branch called origin/x and
// a local branch called origin/x are both spelled the same short.
const (
	GitRefLocal  = "local"
	GitRefRemote = "remote"
	GitRefTag    = "tag"
)

// GitRef is a branch or a tag. Name is the short name; Head marks the branch
// HEAD is on.
type GitRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Head bool   `json:"head,omitempty"`
	// Worktree is where a local branch is checked out, "" for none. GitRefs
	// fills it; a log row's refs leave it empty.
	Worktree string `json:"worktree,omitempty"`
}

// GitLogOptions asks for a page of a repository's history. An empty Ref is
// every branch, remote branch and tag. Query, Author, Since and File narrow it;
// a narrowed page lists commits whose parents it may not hold.
type GitLogOptions struct {
	Path   string `json:"path"`
	Ref    string `json:"ref,omitempty"`
	Query  string `json:"query,omitempty"`
	Author string `json:"author,omitempty"`
	Since  int64  `json:"since,omitempty"` // unix seconds, 0 for no bound
	File   string `json:"file,omitempty"`
	Skip   int    `json:"skip,omitempty"`
	Limit  int    `json:"limit"`
}

// GitAuthorsOptions names the history GitAuthors reads, Ref empty for the same
// everything GitLogOptions reads.
type GitAuthorsOptions struct {
	Path string `json:"path"`
	Ref  string `json:"ref,omitempty"`
}

// GitCommit is one row of the history.
type GitCommit struct {
	Hash    string   `json:"hash"`
	Parents []string `json:"parents"`
	Author  string   `json:"author"`
	When    int64    `json:"when"` // unix seconds, author date
	Subject string   `json:"subject"`
	Refs    []GitRef `json:"refs"`
	// Head marks the commit HEAD is at, a detached HEAD included.
	Head bool `json:"head,omitempty"`
}

// GitLogPage is a page of the history, newest first; More says another page
// follows it.
type GitLogPage struct {
	Commits []GitCommit `json:"commits"`
	More    bool        `json:"more"`
}

// GitRefs is every branch and tag of a repository. Head is the commit HEAD is
// at, "" in a repository without commits.
type GitRefs struct {
	Refs []GitRef `json:"refs"`
	Head string   `json:"head"`
	// Stamp changes when any ref or HEAD moves, for a poll to tell a new
	// history from the one it holds without reading the log again.
	Stamp string `json:"stamp"`
}

// GitCommitOptions names a commit of the checkout at Path.
type GitCommitOptions struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// GitCommitDiffOptions names one file of a commit. OldFile is where a renamed
// file came from: without it, git reads the rename as an added file.
type GitCommitDiffOptions struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	File    string `json:"file"`
	OldFile string `json:"oldFile,omitempty"`
}

// GitCommitFile is one file a commit changed. Binary has no line counts.
type GitCommitFile struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary,omitempty"`
}

// GitCommitDetail is what the history shows of the selected commit. Files of a
// merge are its changes against the first parent.
type GitCommitDetail struct {
	Hash    string          `json:"hash"`
	Author  string          `json:"author"`
	Email   string          `json:"email"`
	When    int64           `json:"when"`
	Message string          `json:"message"`
	Refs    []GitRef        `json:"refs"`
	Files   []GitCommitFile `json:"files"`
}

// GitCheckoutOptions switches the checkout at Path to Branch. A Remote branch
// ("origin/x") is checked out as the local branch that tracks it.
type GitCheckoutOptions struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Remote bool   `json:"remote,omitempty"`
}

// GitCreateBranchOptions makes branch Name at From, a ref or a commit, without
// checking it out.
type GitCreateBranchOptions struct {
	Path string `json:"path"`
	Name string `json:"name"`
	From string `json:"from"`
	// Upstream is written to the new branch's config, to make a deleted one again.
	Upstream GitUpstream `json:"upstream,omitzero"`
}

// GitUpstream is a branch's branch.<name>.remote, .merge and .pushRemote
// settings, each "" when unset.
type GitUpstream struct {
	Remote     string `json:"remote,omitempty"`
	Merge      string `json:"merge,omitempty"`
	PushRemote string `json:"pushRemote,omitempty"`
}

// GitMergeOptions names Ref, a branch or commit, to merge into the checkout at
// Path.
type GitMergeOptions struct {
	Path string `json:"path"`
	Ref  string `json:"ref"`
}

// GitMergePreview is what merging Ref would bring: its commits HEAD lacks, and
// the files it would conflict on.
type GitMergePreview struct {
	Commits   int      `json:"commits"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// GitMergeResult is a merge's outcome: the commit HEAD left and the one it
// reached, or, when it stopped, the files in conflict and no After.
type GitMergeResult struct {
	Before    string   `json:"before"`
	After     string   `json:"after,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// GitMergeUndoOptions takes the checkout at Path back from After to Before.
type GitMergeUndoOptions struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// A GitConflictChunk's Kind: which side changed the base there.
const (
	GitChunkOurs     = "ours"
	GitChunkTheirs   = "theirs"
	GitChunkBoth     = "both" // both sides made the same change
	GitChunkConflict = "conflict"
)

// GitLines is a run of lines, From included and To not, counted from 0; an
// empty run is the point between two lines.
type GitLines struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// GitConflictChunk is one place where a side changed the base, and the lines
// it spans in each of the three versions.
type GitConflictChunk struct {
	Kind   string   `json:"kind"`
	Base   GitLines `json:"base"`
	Ours   GitLines `json:"ours"`
	Theirs GitLines `json:"theirs"`
}

// GitConflict is a conflicted file's three versions as the index holds them,
// and the chunks where they differ, in order.
type GitConflict struct {
	Base   string             `json:"base"`
	Ours   string             `json:"ours"`
	Theirs string             `json:"theirs"`
	Chunks []GitConflictChunk `json:"chunks"`
}

// GitResolveFileOptions writes Content as the resolution of conflicted file Rel
// in the checkout at Path.
type GitResolveFileOptions struct {
	Path    string `json:"path"`
	Rel     string `json:"rel"`
	Content string `json:"content"`
}

// GitDeletedBranch is what a delete returns to make the branch again: the
// commit it pointed at and the upstream it tracked.
type GitDeletedBranch struct {
	Hash     string      `json:"hash"`
	Upstream GitUpstream `json:"upstream,omitzero"`
}

// GitRenameBranchOptions renames local branch Name to NewName.
type GitRenameBranchOptions struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	NewName string `json:"newName"`
}

// GitBranchOptions names a local branch of the checkout at Path.
type GitBranchOptions struct {
	Path string `json:"path"`
	Name string `json:"name"`
}
