package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
)

var errNotTextConflict = errors.New("This conflict is not between two text versions of the file, so it can only be resolved in a terminal or by an agent.")

// GitConflict reads a conflicted file's base, ours and theirs from the index
// (stages 1, 2 and 3) and lays out where each side changed the base. A file
// added on both sides has no base; one deleted on a side, a binary or a
// non-UTF-8 one is refused, since the resolver writes the text it shows.
func (s *Service) GitConflict(_ context.Context, opts api.CheckoutFileOptions) (api.GitConflict, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return api.GitConflict{}, err
	}
	stages, err := conflictStages(opts.Path, opts.Rel)
	if err != nil {
		return api.GitConflict{}, err
	}
	if stages[2] == "" || stages[3] == "" {
		return api.GitConflict{}, errNotTextConflict
	}
	var c api.GitConflict
	for _, side := range []struct {
		stage int
		text  *string
	}{{1, &c.Base}, {2, &c.Ours}, {3, &c.Theirs}} {
		if *side.text, err = blobText(opts.Path, stages[side.stage]); err != nil {
			return api.GitConflict{}, err
		}
	}
	ours, err := blobHunks(opts.Path, stages[1], stages[2], c.Ours)
	if err != nil {
		return api.GitConflict{}, err
	}
	theirs, err := blobHunks(opts.Path, stages[1], stages[3], c.Theirs)
	if err != nil {
		return api.GitConflict{}, err
	}
	c.Chunks = diff3Chunks(ours, theirs, lines(c.Ours), lines(c.Theirs))
	return c, nil
}

// conflictStages maps each index stage of rel to its blob, "" for a stage the
// index does not hold.
func conflictStages(path, rel string) (map[int]string, error) {
	out, err := readGit(path, "ls-files", "-u", "-z", "--", rel)
	if err != nil {
		return nil, err
	}
	stages := map[int]string{}
	for record := range strings.SplitSeq(out, "\x00") {
		// <mode> <oid> <stage>\t<path>
		meta, _, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			continue
		}
		if stage, err := strconv.Atoi(fields[2]); err == nil {
			stages[stage] = fields[1]
		}
	}
	if len(stages) == 0 {
		return nil, errors.New("That file has no conflict to resolve.")
	}
	return stages, nil
}

func blobText(path, oid string) (string, error) {
	if oid == "" {
		return "", nil
	}
	out, err := readGit(path, "cat-file", "blob", oid)
	if err != nil {
		return "", err
	}
	if len(out) > maxTextFileBytes || isBinary([]byte(out)) || !utf8.ValidString(out) {
		return "", errNotTextConflict
	}
	return out, nil
}

// hunk is one change of a side against the base, in lines counted from 0.
type hunk struct{ base, side api.GitLines }

// blobHunks diffs the base blob against a side's, with no context, so every
// hunk header is a change. With no base the whole side is one insertion.
func blobHunks(path, base, side, sideText string) ([]hunk, error) {
	if base == "" {
		if sideText == "" {
			return nil, nil
		}
		return []hunk{{side: api.GitLines{To: len(lines(sideText))}}}, nil
	}
	out, err := readGit(path, "diff", "--no-color", "--no-ext-diff", "--no-textconv", "-U0", base, side)
	if err != nil {
		return nil, err
	}
	var hunks []hunk
	for line := range strings.Lines(out) {
		if h, ok := parseHunkHeader(line); ok {
			hunks = append(hunks, h)
		}
	}
	return hunks, nil
}

// parseHunkHeader reads "@@ -a,b +c,d @@". A count left out is 1, and a run of
// none starts after its line rather than at it.
func parseHunkHeader(line string) (hunk, bool) {
	rest, ok := strings.CutPrefix(line, "@@ -")
	if !ok {
		return hunk{}, false
	}
	old, rest, ok := strings.Cut(rest, " +")
	if !ok {
		return hunk{}, false
	}
	side, _, ok := strings.Cut(rest, " @@")
	if !ok {
		return hunk{}, false
	}
	o, okOld := lineRun(old)
	n, okNew := lineRun(side)
	return hunk{base: o, side: n}, okOld && okNew
}

func lineRun(spec string) (api.GitLines, bool) {
	start, count, hasCount := strings.Cut(spec, ",")
	from, err := strconv.Atoi(start)
	if err != nil {
		return api.GitLines{}, false
	}
	n := 1
	if hasCount {
		if n, err = strconv.Atoi(count); err != nil {
			return api.GitLines{}, false
		}
	}
	if n == 0 {
		return api.GitLines{From: from, To: from}, true
	}
	return api.GitLines{From: from - 1, To: from - 1 + n}, true
}

// lines splits text the way git counts lines: a final newline ends the last
// line rather than starting an empty one.
func lines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.SplitAfter(strings.TrimSuffix(text, "\n"), "\n")
}

// diff3Chunks lines both sides' hunks up on the base: hunks of the two sides
// that overlap or touch there make one chunk, a conflict unless both wrote the
// same lines.
func diff3Chunks(ours, theirs []hunk, oursLines, theirsLines []string) []api.GitConflictChunk {
	var chunks []api.GitConflictChunk
	i, j := 0, 0
	oursShift, theirsShift := 0, 0 // side line minus base line, past the hunks consumed
	for i < len(ours) || j < len(theirs) {
		var c api.GitConflictChunk
		var mine, yours []hunk
		c.Base = nextBase(ours, theirs, i, j)
		for {
			grew := false
			for i < len(ours) && touches(ours[i].base, c.Base) {
				mine, c.Base, grew = append(mine, ours[i]), span(c.Base, ours[i].base), true
				i++
			}
			for j < len(theirs) && touches(theirs[j].base, c.Base) {
				yours, c.Base, grew = append(yours, theirs[j]), span(c.Base, theirs[j].base), true
				j++
			}
			if !grew {
				break
			}
		}
		c.Ours, oursShift = sideRun(c.Base, mine, oursShift)
		c.Theirs, theirsShift = sideRun(c.Base, yours, theirsShift)
		switch {
		case len(yours) == 0:
			c.Kind = api.GitChunkOurs
		case len(mine) == 0:
			c.Kind = api.GitChunkTheirs
		case slices.Equal(oursLines[c.Ours.From:c.Ours.To], theirsLines[c.Theirs.From:c.Theirs.To]):
			c.Kind = api.GitChunkBoth
		default:
			c.Kind = api.GitChunkConflict
		}
		chunks = append(chunks, c)
	}
	return chunks
}

// nextBase is the base run of whichever side's next hunk comes first.
func nextBase(ours, theirs []hunk, i, j int) api.GitLines {
	switch {
	case i == len(ours):
		return theirs[j].base
	case j == len(theirs):
		return ours[i].base
	case theirs[j].base.From < ours[i].base.From:
		return theirs[j].base
	}
	return ours[i].base
}

func touches(a, b api.GitLines) bool { return a.From <= b.To && b.From <= a.To }

func span(a, b api.GitLines) api.GitLines {
	return api.GitLines{From: min(a.From, b.From), To: max(a.To, b.To)}
}

// sideRun is where a chunk's base run lands in a side, from the hunks of that
// side inside it, or from the shift the earlier hunks left when it has none.
func sideRun(base api.GitLines, hunks []hunk, shift int) (api.GitLines, int) {
	if len(hunks) == 0 {
		return api.GitLines{From: base.From + shift, To: base.To + shift}, shift
	}
	first, last := hunks[0], hunks[len(hunks)-1]
	run := api.GitLines{
		From: first.side.From - (first.base.From - base.From),
		To:   last.side.To + (base.To - last.base.To),
	}
	return run, last.side.To - last.base.To
}

// GitResolveFile writes content over a conflicted file and stages it, which is
// how git records the conflict resolved.
func (s *Service) GitResolveFile(_ context.Context, opts api.GitResolveFileOptions) (api.None, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return api.None{}, err
	}
	if !slices.Contains(readWorkTree(opts.Path).conflicted, opts.Rel) {
		return api.None{}, errors.New("That file has no conflict to resolve.")
	}
	if err := os.WriteFile(filepath.Join(opts.Path, opts.Rel), []byte(opts.Content), 0o644); err != nil {
		return api.None{}, fmt.Errorf("write %s: %w", opts.Rel, err)
	}
	_, err := runGit(opts.Path, "add", "--", opts.Rel)
	return api.None{}, err
}
