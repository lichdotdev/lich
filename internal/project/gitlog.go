package project

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// gitLogBudget bounds one read of the history. Ordering the history walks all
// of it, which on a very large repository without a commit-graph takes seconds.
const gitLogBudget = 30 * time.Second

// maxGitLogPage is the most commits one call returns, whatever a caller asks.
const maxGitLogPage = 2000

// logFields is how many NUL-terminated fields gitLogFormat prints per commit.
const logFields = 6

// gitLogFormat ends every field in NUL. A subject cannot hold one, so it needs
// no escaping; tformat puts a newline between commits, trimmed when parsing.
const gitLogFormat = "tformat:%H%x00%P%x00%an%x00%at%x00%D%x00%s%x00"

// gitPlainOutput undoes what a user's git config does to output lich parses:
// log.showSignature prints gpg's text among the fields, diff.external and
// textconv replace the diff with some other program's idea of one.
var gitPlainOutput = []string{"--no-color", "--no-show-signature", "--encoding=UTF-8", "--no-ext-diff", "--no-textconv", "--decorate=full"}

// allHistory is what an empty Ref reads: not --all, which also walks refs/stash
// and whatever else a tool keeps under refs/.
var allHistory = []string{"--branches", "--remotes", "--tags", "HEAD"}

var hexPrefix = regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`)

// gitRead is runGit under a budget, for a read of unbounded size.
func gitRead(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitLogBudget)
	defer cancel()
	cmd := commandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("Reading the history took longer than %s and was given up on.", gitLogBudget)
	}
	return "", gitFailure(args, stderr.String(), err)
}

// GitLog returns a page of the history, in an order where every commit comes
// before its parents, which is what the graph is drawn from.
func (s *Service) GitLog(ctx context.Context, opts api.GitLogOptions) (api.GitLogPage, error) {
	page := api.GitLogPage{Commits: []api.GitCommit{}}
	if _, ok := gitQuiet(opts.Path, "rev-parse", "--verify", "--quiet", "HEAD"); !ok && opts.Ref == "" {
		return page, nil
	}
	limit := min(max(opts.Limit, 1), maxGitLogPage)
	out, err := gitRead(ctx, opts.Path, gitLogArgs(opts, limit, exactCommit(opts))...)
	if err != nil {
		return page, err
	}
	commits, err := parseGitLog(out)
	if err != nil {
		return page, err
	}
	page.More = len(commits) > limit
	page.Commits = commits[:min(len(commits), limit)]
	return page, nil
}

// exactCommit is the commit a search names by hash, "" for a search of the
// messages. Asked of git, so a hex word that names no commit is still searched
// for as text.
func exactCommit(opts api.GitLogOptions) string {
	if !hexPrefix.MatchString(opts.Query) {
		return ""
	}
	out, ok := gitQuiet(opts.Path, "rev-parse", "--verify", "--quiet", "--end-of-options", opts.Query+"^{commit}")
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
}

// gitLogArgs asks for one commit more than the page, whose presence is how the
// page knows another follows.
func gitLogArgs(opts api.GitLogOptions, limit int, exact string) []string {
	args := slices.Concat([]string{"log"}, gitPlainOutput, []string{"--date-order", "--format=" + gitLogFormat})
	if exact != "" {
		return append(args, "--max-count=1", "--end-of-options", exact)
	}
	args = append(args, "--max-count="+strconv.Itoa(limit+1))
	if opts.Skip > 0 {
		args = append(args, "--skip="+strconv.Itoa(opts.Skip))
	}
	if opts.Query != "" || opts.Author != "" {
		args = append(args, "--fixed-strings", "--regexp-ignore-case")
	}
	if opts.Query != "" {
		args = append(args, "--grep="+opts.Query)
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if opts.Since > 0 {
		args = append(args, "--since="+strconv.FormatInt(opts.Since, 10))
	}
	if opts.Ref != "" {
		args = append(args, "--end-of-options", opts.Ref)
	} else {
		args = append(args, allHistory...)
	}
	if opts.File != "" {
		args = append(args, "--", opts.File)
	}
	return args
}

// GitAuthors lists who wrote the history, the one with most commits first, by
// the names GitLog prints and its Author filter matches.
func (s *Service) GitAuthors(ctx context.Context, opts api.GitAuthorsOptions) ([]string, error) {
	if _, ok := gitQuiet(opts.Path, "rev-parse", "--verify", "--quiet", "HEAD"); !ok && opts.Ref == "" {
		return []string{}, nil
	}
	args := slices.Concat([]string{"log"}, gitPlainOutput, []string{"--format=%an"})
	if opts.Ref != "" {
		args = append(args, "--end-of-options", opts.Ref)
	} else {
		args = append(args, allHistory...)
	}
	out, err := gitRead(ctx, opts.Path, args...)
	if err != nil {
		return nil, err
	}
	commits := map[string]int{}
	for line := range strings.SplitSeq(out, "\n") {
		if line != "" {
			commits[displayText([]byte(line))]++
		}
	}
	authors := slices.Collect(maps.Keys(commits))
	slices.SortFunc(authors, func(a, b string) int {
		return cmp.Or(commits[b]-commits[a], strings.Compare(a, b))
	})
	return authors, nil
}

func parseGitLog(out string) ([]api.GitCommit, error) {
	fields := strings.Split(out, "\x00")
	// The last split is what follows the final NUL: a newline, or nothing.
	fields = fields[:len(fields)-1]
	if len(fields)%logFields != 0 {
		return nil, fmt.Errorf("git log printed %d fields, not a multiple of %d", len(fields), logFields)
	}
	commits := make([]api.GitCommit, 0, len(fields)/logFields)
	for f := range slices.Chunk(fields, logFields) {
		c := api.GitCommit{
			Hash:    strings.TrimSpace(f[0]),
			Parents: strings.Fields(f[1]),
			Author:  displayText([]byte(f[2])),
			When:    parseUnix(f[3]),
			Subject: displayText([]byte(f[5])),
		}
		c.Refs, c.Head = parseDecorations(f[4])
		commits = append(commits, c)
	}
	return commits, nil
}

func parseUnix(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64) // git prints digits, and an unreadable date sorts as the epoch
	return n
}

// parseDecorations reads %D under --decorate=full: "HEAD -> refs/heads/main,
// tag: refs/tags/v1, refs/remotes/origin/main". A remote's symbolic HEAD names
// no branch of its own and is left out.
func parseDecorations(d string) (refs []api.GitRef, head bool) {
	refs = []api.GitRef{}
	if d == "" {
		return refs, false
	}
	for part := range strings.SplitSeq(d, ", ") {
		if part == "HEAD" {
			head = true
			continue
		}
		name, onHead := strings.CutPrefix(part, "HEAD -> ")
		name = strings.TrimPrefix(name, "tag: ")
		head = head || onHead
		if ref, ok := parseRefName(name); ok {
			ref.Head = onHead
			refs = append(refs, ref)
		}
	}
	return refs, head
}

func parseRefName(full string) (api.GitRef, bool) {
	if name, ok := strings.CutPrefix(full, "refs/heads/"); ok {
		return api.GitRef{Name: name, Kind: api.GitRefLocal}, true
	}
	if name, ok := strings.CutPrefix(full, "refs/tags/"); ok {
		return api.GitRef{Name: name, Kind: api.GitRefTag}, true
	}
	if name, ok := strings.CutPrefix(full, "refs/remotes/"); ok && !strings.HasSuffix(name, "/HEAD") {
		return api.GitRef{Name: name, Kind: api.GitRefRemote}, true
	}
	return api.GitRef{}, false
}

// GitRefs lists the repository's branches and tags. Run on every poll of an
// open panel, so it stays one for-each-ref and one rev-parse.
func (s *Service) GitRefs(ctx context.Context, opts api.PathOptions) (api.GitRefs, error) {
	out, err := gitRead(ctx, opts.Path, "for-each-ref", "--sort=refname",
		"--format=%(refname)%00%(objectname)%00%(HEAD)%00%(worktreepath)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return api.GitRefs{Refs: []api.GitRef{}}, err
	}
	head, _ := gitQuiet(opts.Path, "rev-parse", "--verify", "--quiet", "HEAD")
	return parseGitRefs(out, strings.TrimSpace(head)), nil
}

func parseGitRefs(out, head string) api.GitRefs {
	sum := sha256.Sum256([]byte(head + "\n" + out))
	refs := api.GitRefs{Refs: []api.GitRef{}, Head: head, Stamp: hex.EncodeToString(sum[:])}
	for _, line := range splitLines(out) {
		f := strings.Split(line, "\x00")
		if len(f) != 4 {
			continue
		}
		if ref, ok := parseRefName(f[0]); ok {
			ref.Head, ref.Worktree = f[2] == "*", f[3]
			refs.Refs = append(refs.Refs, ref)
		}
	}
	return refs
}

// GitCommit reads one commit's message and the files it changed.
func (s *Service) GitCommit(ctx context.Context, opts api.GitCommitOptions) (api.GitCommitDetail, error) {
	detail := api.GitCommitDetail{Refs: []api.GitRef{}, Files: []api.GitCommitFile{}}
	if !hexPrefix.MatchString(opts.Hash) {
		return detail, fmt.Errorf("%q is not a commit hash", opts.Hash)
	}
	args := slices.Concat([]string{"show"}, gitPlainOutput, gitDiffOfCommit,
		[]string{"--numstat", "-z", "--format=%H%x00%an%x00%ae%x00%at%x00%D%x00%B%x00", "--end-of-options", opts.Hash})
	out, err := gitRead(ctx, opts.Path, args...)
	if err != nil {
		return detail, err
	}
	return parseGitCommit(out)
}

// gitDiffOfCommit shows a merge against its first parent, the change it brought
// to the branch; left to the default a clean merge shows no files at all.
var gitDiffOfCommit = []string{"-M", "--diff-merges=first-parent"}

// commitHeaderFields is how many NUL-terminated fields the show format prints.
const commitHeaderFields = 6

func parseGitCommit(out string) (api.GitCommitDetail, error) {
	detail := api.GitCommitDetail{Refs: []api.GitRef{}, Files: []api.GitCommitFile{}}
	fields := strings.Split(out, "\x00")
	if len(fields) < commitHeaderFields {
		return detail, fmt.Errorf("git show printed %d fields, want at least %d", len(fields), commitHeaderFields)
	}
	detail.Hash = strings.TrimSpace(fields[0])
	detail.Author = displayText([]byte(fields[1]))
	detail.Email = displayText([]byte(fields[2]))
	detail.When = parseUnix(fields[3])
	detail.Refs, _ = parseDecorations(fields[4])
	detail.Message = strings.TrimSpace(displayText([]byte(fields[5])))
	detail.Files = parseNumstatZ(fields[commitHeaderFields:])
	return detail, nil
}

// parseNumstatZ reads --numstat -z, already split on NUL: "add\tdel\tpath", or
// for a rename "add\tdel\t" with the old and new path as the next two entries.
// A binary file counts "-" on both sides.
func parseNumstatZ(entries []string) []api.GitCommitFile {
	files := []api.GitCommitFile{}
	for i := 0; i < len(entries); i++ {
		cols := strings.SplitN(strings.TrimLeft(entries[i], "\n"), "\t", 3)
		if len(cols) != 3 {
			continue
		}
		f := api.GitCommitFile{Path: cols[2], Binary: cols[0] == "-"}
		f.Added, _ = strconv.Atoi(cols[0])   // "-" for a binary file, which counts nothing
		f.Deleted, _ = strconv.Atoi(cols[1]) // likewise
		if f.Path == "" && i+2 < len(entries) {
			f.OldPath, f.Path = entries[i+1], entries[i+2]
			i += 2
		}
		files = append(files, f)
	}
	return files
}

// GitCommitDiff is one file's patch in a commit, one file so a commit that
// rewrites a generated tree does not cross the RPC whole to show one path.
func (s *Service) GitCommitDiff(ctx context.Context, opts api.GitCommitDiffOptions) (string, error) {
	if !hexPrefix.MatchString(opts.Hash) {
		return "", fmt.Errorf("%q is not a commit hash", opts.Hash)
	}
	args := slices.Concat([]string{"show"}, gitPlainOutput, gitDiffOfCommit, []string{"--format=", "--patch", "--end-of-options", opts.Hash, "--", opts.File})
	if opts.OldFile != "" {
		args = append(args, opts.OldFile)
	}
	out, err := gitRead(ctx, opts.Path, args...)
	if err != nil {
		return "", err
	}
	if len(out) > maxPreviewBytes {
		return "", fmt.Errorf("This change is %d MiB, too large to show.", len(out)>>20)
	}
	return displayDiff(out), nil
}
