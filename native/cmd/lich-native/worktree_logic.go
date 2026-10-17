package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
)

// Base options carry their group so one string names the selection, as
// WorktreeDialog's rowValue does: "local:main", "remote:origin/main",
// "worktree:/path/to/checkout".
const (
	baseLocal    = "local"
	baseRemote   = "remote"
	baseWorktree = "worktree"
)

func baseValue(group, id string) string { return group + ":" + id }

func splitBase(value string) (group, id string) {
	group, id, _ = strings.Cut(value, ":")
	return group, id
}

// issueHash and issueURL are issue.ts's: GitHub's spelling of a reference and
// the address copied out of the browser. A bare number is a branch name.
var (
	issueHash = regexp.MustCompile(`^#(\d+)$`)
	issueURL  = regexp.MustCompile(`^https?://[^/\s]+/[^/\s]+/[^/\s]+/issues/(\d+)(?:[/?#].*)?$`)
)

// parseIssueRef is the issue a typed name refers to, 0 for none.
func parseIssueRef(typed string) int {
	trimmed := strings.TrimSpace(typed)
	m := issueHash.FindStringSubmatch(trimmed)
	if m == nil {
		m = issueURL.FindStringSubmatch(trimmed)
	}
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func issueName(i lichclient.Issue) string { return fmt.Sprintf("%d %s", i.Number, i.Title) }

// issueBrief is what the issue's session finds at its prompt: one bracketed
// paste, left unsent.
func issueBrief(i lichclient.Issue) string {
	head := i18n.Prompt("prompts.issue.head", i18n.Args{"number": i.Number, "title": i.Title, "url": i.URL})
	if body := strings.TrimSpace(i.Body); body != "" {
		return bracketedPaste(head + "\n\n" + body)
	}
	return bracketedPaste(head)
}

// The bounds branch-name.ts grows a slug of typed words within, in words and
// in characters, so a run of short words still reads as a phrase and a pasted
// sentence does not become the branch.
const (
	slugMinWords = 2
	slugMaxWords = 5
	slugMinChars = 10
	slugMaxChars = 40
)

// validBranchName approximates `git check-ref-format --branch` for the
// common cases; the backend's git is the authority.
func validBranchName(name string) bool {
	if name == "" || name == "@" || strings.ContainsAny(name, "~^:?*[\\\x7f") {
		return false
	}
	for _, r := range name {
		if r <= ' ' {
			return false
		}
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") ||
		strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// toBranchName is branch-name.ts toBranchName: the typed name when git takes
// it, a kebab-case slug of its first words when not, "" for nothing usable.
func toBranchName(typed string) string {
	trimmed := strings.TrimSpace(typed)
	if trimmed == "" || validBranchName(trimmed) {
		return trimmed
	}
	words := strings.FieldsFunc(strings.ToLower(trimmed), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var picked []string
	length := 0
	for _, w := range words {
		size := utf8.RuneCountInString(w)
		next := length + 1 + size
		if length == 0 {
			next = size
		}
		haveMinimum := len(picked) >= slugMinWords && length >= slugMinChars
		if len(picked) >= slugMaxWords || (next > slugMaxChars && haveMinimum) {
			break
		}
		picked = append(picked, w)
		length = next
	}
	slug := []rune(strings.Join(picked, "-"))
	return strings.TrimRight(string(slug[:min(len(slug), slugMaxChars)]), "-")
}

// baseCheck is the freshness read of one local branch: pending until done is
// closed, then the backend's answer or why it could not give one.
type baseCheck struct {
	done      chan struct{}
	freshness lichclient.BaseFreshness
	err       string
}

func (c *baseCheck) finished() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// upstreamWins is whether a worktree off a local branch starts from its
// upstream instead: the upstream has commits the branch lacks and the branch
// has none of its own to lose.
func (c *baseCheck) upstreamWins() bool {
	f := c.freshness
	return c.err == "" && f.FetchError == "" && f.Upstream != "" && f.Behind > 0 && f.Ahead == 0
}

// startPoint is where a worktree for the chosen base starts. A local branch
// gives way to its upstream when that is newer, unless uncommitted changes are
// being carried: they are copied as whole files, so they only fit the commit
// they were made on. check is nil for anything but a local base.
func startPoint(base string, check *baseCheck, carry bool) (start string, remote bool) {
	group, id := splitBase(base)
	if group == baseRemote {
		return id, true
	}
	if group == baseLocal && !carry && check != nil && check.upstreamWins() {
		return check.freshness.Upstream, true
	}
	return id, false
}

// remoteOf is the remote an upstream lives on, "origin" for "origin/main".
func remoteOf(upstream string) string {
	remote, _, _ := strings.Cut(upstream, "/")
	return remote
}

// freshnessLine is what the dialog says under the base about where the
// worktree starts, and whether it is a warning. "" when the local branch is
// all there is or as new as its upstream.
func freshnessLine(branch string, check *baseCheck, carry bool) (text string, warn bool) {
	if check == nil {
		return "", false
	}
	if !check.finished() {
		return i18n.T("sidebar.worktreeLogic.checking"), false
	}
	f := check.freshness
	switch {
	case check.err != "":
		return i18n.T("sidebar.worktreeLogic.unreachableRemote", i18n.Args{"branch": branch}), true
	case f.Upstream == "":
		return "", false
	case f.FetchError != "":
		return i18n.T("sidebar.worktreeLogic.unreachable", i18n.Args{"remote": remoteOf(f.Upstream), "branch": branch}), true
	case carry && check.upstreamWins():
		return i18n.T("sidebar.worktreeLogic.keepsChanges", i18n.Args{"branch": branch}), false
	case check.upstreamWins():
		return i18n.T("sidebar.worktreeLogic.behind", i18n.Args{"count": f.Behind, "upstream": f.Upstream}), false
	case f.Ahead > 0:
		return i18n.T("sidebar.worktreeLogic.notPushed", i18n.Args{"count": f.Ahead, "branch": branch}), false
	}
	return "", false
}
