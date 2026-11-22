package main

// How the @ list ranks a checkout's paths, ported from Claude Code 2.1.330's
// file index (its FileIndex search): a path matches when it holds the query's
// characters in order, and scores by runs, gaps, starts of words and shortness.

import (
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// The weights of Claude Code's file index.
const (
	mentionLimit       = 15 // the suggestions it shows
	mentionQueryMax    = 64 // the query characters it reads
	mentionRunBonus    = 4  // a character right after the one before
	mentionGapCost     = 3  // a gap, plus one per character skipped
	mentionStartBonus  = 8  // a character starting the path or a word
	mentionCamelBonus  = 6  // an upper case letter after a lower case one
	mentionCharWeight  = 16 // each query character
	mentionShortBonus  = 32 // a short path, less a quarter of its length
	mentionTopLevelCap = 100
)

// mentionIndex is a checkout's folders, each with a trailing slash, and files
// as the @ list offers them, with what the search looks up on each.
type mentionIndex struct {
	paths, lower []string
	// letters is the a-z a path holds, as bits; base is where its last
	// segment starts, 0 when lowering changed its length, and baseLetters
	// that segment's letters.
	letters, baseLetters []uint32
	base                 []int
	top                  []string
}

func newMentionIndex(files []string, dirs map[string]bool) *mentionIndex {
	folders := make([]string, 0, len(dirs))
	for d := range dirs {
		folders = append(folders, d+"/")
	}
	slices.Sort(folders)
	all := append(folders, files...)
	x := &mentionIndex{paths: all, lower: make([]string, len(all)), letters: make([]uint32, len(all)), baseLetters: make([]uint32, len(all)), base: make([]int, len(all))}
	for i, p := range all {
		low := strings.ToLower(p)
		x.lower[i] = low
		base := 0
		for j := 0; j < len(low); j++ {
			c := low[j]
			switch {
			case c >= 'a' && c <= 'z':
				x.letters[i] |= 1 << (c - 'a')
				x.baseLetters[i] |= 1 << (c - 'a')
			case (c == '/' || c == '\\') && j < len(low)-1:
				base, x.baseLetters[i] = j+1, 0
			}
		}
		if len(low) == len(p) {
			x.base[i] = base
		}
	}
	x.top = topLevel(all)
	return x
}

// topLevel is what a bare @ offers: the first segment of every path, a folder
// with its slash, the shortest first.
func topLevel(paths []string) []string {
	seen := map[string]bool{}
	var top []string
	for _, p := range paths {
		head := p
		if i := strings.IndexByte(p, '/'); i >= 0 {
			head = p[:i+1]
		}
		if !seen[head] {
			seen[head] = true
			top = append(top, head)
		}
		if len(top) >= mentionTopLevelCap {
			break
		}
	}
	slices.SortFunc(top, func(a, b string) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	})
	return top
}

// search is the paths query matches, best first, at most limit. Upper case in
// the query makes it match case.
func (x *mentionIndex) search(query string, limit int) []string {
	query = strings.TrimPrefix(query, "./")
	if query == "" {
		return x.top[:min(len(x.top), limit)]
	}
	q := []rune(query)
	q = q[:min(len(q), mentionQueryMax)]
	exact := query != strings.ToLower(query)
	var want uint32
	chars := make([]string, len(q))
	for i, r := range q {
		chars[i] = string(r)
		if r >= 'a' && r <= 'z' {
			want |= 1 << (r - 'a')
		}
	}
	type hit struct {
		i     int
		score int
	}
	var hits []hit
	at, inBase := make([]int, len(chars)), make([]int, len(chars))
	for i, p := range x.paths {
		if x.letters[i]&want != want {
			continue
		}
		s := x.lower[i]
		if exact {
			s = p
		}
		score, ok := x.score(i, s, chars, want, at, inBase)
		if ok {
			hits = append(hits, hit{i, score})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return b.score - a.score })
	out := make([]string, 0, min(len(hits), limit))
	for _, h := range hits[:min(len(hits), limit)] {
		out = append(out, x.paths[h.i])
	}
	return out
}

// score places chars in s at their first places in order, and again from the
// start of the last segment when that leaves gaps, and keeps the better; at
// and inBase are room for the places, reused across paths.
func (x *mentionIndex) score(i int, s string, chars []string, want uint32, at, inBase []int) (int, bool) {
	runs, gaps, ok := mentionPlace(s, chars, 0, at)
	if !ok {
		return 0, false
	}
	p := x.paths[i]
	best := runs - gaps + startBonuses(p, at)
	if b := x.base[i]; gaps > 0 && at[0] < b && x.baseLetters[i]&want == want {
		if r, g, ok := mentionPlace(s, chars, b, inBase); ok {
			best = max(best, r-g+startBonuses(p, inBase))
		}
	}
	return best + len(chars)*mentionCharWeight + max(0, mentionShortBonus-len(x.lower[i])>>2), true
}

// mentionPlace finds each of chars in s after the one before, from from, into
// at, and what the runs earn and the gaps cost between them.
func mentionPlace(s string, chars []string, from int, at []int) (runs, gaps int, ok bool) {
	prev := from - 1
	for k, c := range chars {
		j := strings.Index(s[prev+1:], c)
		if j < 0 {
			return 0, 0, false
		}
		j += prev + 1
		at[k] = j
		if k > 0 {
			if gap := j - prev - 1; gap == 0 {
				runs += mentionRunBonus
			} else {
				gaps += mentionGapCost + gap
			}
		}
		prev = j
	}
	return runs, gaps, true
}

func startBonuses(p string, at []int) int {
	n := 0
	for _, j := range at {
		n += startBonus(p, j)
	}
	return n
}

func startBonus(p string, j int) int {
	if j == 0 {
		return mentionStartBonus
	}
	before, _ := utf8.DecodeLastRuneInString(p[:j])
	if strings.ContainsRune(`/\-_. `, before) {
		return mentionStartBonus
	}
	now, _ := utf8.DecodeRuneInString(p[j:])
	if before >= 'a' && before <= 'z' && now >= 'A' && now <= 'Z' {
		return mentionCamelBonus
	}
	return 0
}

// mentionParts is a path's last segment, a folder's with its slash, and the
// folder it sits in.
func mentionParts(p string) (dir, name string) {
	folder := strings.HasSuffix(p, "/")
	dir, name = path.Split(strings.TrimSuffix(p, "/"))
	if folder {
		name += "/"
	}
	return dir, name
}
