package project

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
)

// uncommittedHash is the commit git blame names a line nobody committed by.
const uncommittedHash = "0000000000000000000000000000000000000000"

// GitBlame says who last changed each line of a file of the checkout, as the
// file is in the working tree. A file git does not track yet, or one in a
// repository with no commit, is all uncommitted lines; outside a repository
// there is nothing to blame.
func (s *Service) GitBlame(ctx context.Context, opts api.CheckoutFileOptions) (api.GitBlame, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return api.GitBlame{}, err
	}
	empty := api.GitBlame{Lines: []string{}, Commits: map[string]api.GitBlameCommit{}}
	if _, ok := gitQuiet(opts.Path, "rev-parse", "--is-inside-work-tree"); !ok {
		return api.GitBlame{}, errors.New("This folder is not a git repository.")
	}
	_, hasHead := gitQuiet(opts.Path, "rev-parse", "--verify", "--quiet", "HEAD")
	_, tracked := gitQuiet(opts.Path, "ls-files", "--error-unmatch", "--", opts.Rel)
	if !hasHead || !tracked {
		return empty, nil
	}
	out, err := gitRead(ctx, opts.Path, slices.Concat(readOnlyGit, []string{"blame", "--porcelain", "--", opts.Rel})...)
	if err != nil {
		return api.GitBlame{}, err
	}
	blame := parseBlame(out)
	if err := s.blameMessages(ctx, opts.Path, blame); err != nil {
		return api.GitBlame{}, err
	}
	return blame, nil
}

// parseBlame reads git blame --porcelain: a header per line, "<hash> <orig>
// <final> [<count>]", the commit's fields the first time it shows, then the
// line itself after a tab.
func parseBlame(out string) api.GitBlame {
	b := api.GitBlame{Lines: []string{}, Commits: map[string]api.GitBlameCommit{}}
	hash, final := "", 0
	for line := range strings.Lines(out) {
		line = strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(line, "\t") {
			for len(b.Lines) < final {
				b.Lines = append(b.Lines, "")
			}
			if hash != uncommittedHash {
				b.Lines[final-1] = hash
			}
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		if h, n, ok := blameHeader(key, value); ok {
			hash, final = h, n
			continue
		}
		if hash == uncommittedHash {
			continue
		}
		c := b.Commits[hash]
		switch key {
		case "author":
			c.Author = value
		case "author-time":
			c.Time, _ = strconv.ParseInt(value, 10, 64)
		case "summary":
			c.Summary, c.Message = value, value
		default:
			continue
		}
		b.Commits[hash] = c
	}
	return b
}

// blameHeader reads a line's header: a full hash, then the line's number in
// the commit and in the file.
func blameHeader(key, value string) (hash string, final int, ok bool) {
	if len(key) != len(uncommittedHash) || strings.Trim(key, "0123456789abcdef") != "" {
		return "", 0, false
	}
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return "", 0, false
	}
	n, err := strconv.Atoi(fields[1])
	if err != nil || n < 1 {
		return "", 0, false
	}
	return key, n, true
}

// blameMessages reads each commit's whole message, which porcelain leaves at
// its summary, in one git log.
func (s *Service) blameMessages(ctx context.Context, dir string, b api.GitBlame) error {
	if len(b.Commits) == 0 {
		return nil
	}
	args := slices.Concat(readOnlyGit, []string{"log", "--no-walk=unsorted", "--no-color", "--encoding=UTF-8", "--format=%H%x00%B%x1e"})
	for hash := range b.Commits {
		args = append(args, hash)
	}
	out, err := gitRead(ctx, dir, args...)
	if err != nil {
		return err
	}
	for record := range strings.SplitSeq(out, "\x1e") {
		hash, message, ok := strings.Cut(strings.TrimLeft(record, "\n"), "\x00")
		c, known := b.Commits[hash]
		if !ok || !known {
			continue
		}
		c.Message = strings.TrimSpace(message)
		b.Commits[hash] = c
	}
	return nil
}
