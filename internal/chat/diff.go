package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// diffContext is the unchanged lines shown around a change, git's default.
const diffContext = 3

// lcsCells bounds the table comparing the changed stretch of two texts; a
// stretch past it is shown as every line removed and added.
const lcsCells = 1 << 20

// diffOp is one line of a diff: ' ' kept, '-' removed, '+' added.
type diffOp struct {
	kind byte
	text string
}

// unifiedDiff is old against next as git prints it: the lines that changed,
// diffContext kept lines around them, numbered from where old sits in path.
func unifiedDiff(path, old, next string) string {
	if old == next {
		return ""
	}
	from, to := "a/"+path, "b/"+path
	if old == "" {
		from = "/dev/null"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", from, to)
	writeHunks(&b, lineOps(diffLines(old), diffLines(next)), snippetLine(path, old, next))
	return b.String()
}

// snippetLine is the line old starts on in path as it is on disk, or next
// once the edit is made; 1 when the file holds neither.
//
// shortcut: the first place the text is found; an edit of a stretch the file
// repeats is numbered from its first copy.
func snippetLine(path, old, next string) int {
	if old == "" || !filepath.IsAbs(path) {
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 1
	}
	text := string(data)
	for _, s := range []string{old, next} {
		if i := strings.Index(text, s); s != "" && i >= 0 {
			return strings.Count(text[:i], "\n") + 1
		}
	}
	return 1
}

// lineOps is a over b line by line: the lines both start and end on kept, the
// stretch between compared by longest common subsequence.
func lineOps(a, b []string) []diffOp {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	ops := make([]diffOp, 0, len(a)+len(b))
	for _, l := range a[:pre] {
		ops = append(ops, diffOp{' ', l})
	}
	ops = append(ops, middleOps(a[pre:len(a)-suf], b[pre:len(b)-suf])...)
	for _, l := range a[len(a)-suf:] {
		ops = append(ops, diffOp{' ', l})
	}
	return ops
}

func middleOps(a, b []string) []diffOp {
	ops := make([]diffOp, 0, len(a)+len(b))
	if len(a)*len(b) > lcsCells {
		for _, l := range a {
			ops = append(ops, diffOp{'-', l})
		}
		for _, l := range b {
			ops = append(ops, diffOp{'+', l})
		}
		return ops
	}
	// common[i][j] is the longest run a[i:] and b[j:] share.
	common := make([][]int32, len(a)+1)
	for i := range common {
		common[i] = make([]int32, len(b)+1)
	}
	for i, v := range slices.Backward(a) {
		for j := len(b) - 1; j >= 0; j-- {
			if v == b[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}
	for i, j := 0, 0; i < len(a) || j < len(b); {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i, j = i+1, j+1
		case j == len(b) || i < len(a) && common[i+1][j] >= common[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	return ops
}

// writeHunks writes ops as hunks, the changes no more than twice diffContext
// kept lines apart sharing one, numbered from line at on both sides.
func writeHunks(b *strings.Builder, ops []diffOp, at int) {
	oldBefore, newBefore := make([]int, len(ops)+1), make([]int, len(ops)+1)
	for i, op := range ops {
		oldBefore[i+1], newBefore[i+1] = oldBefore[i], newBefore[i]
		if op.kind != '+' {
			oldBefore[i+1]++
		}
		if op.kind != '-' {
			newBefore[i+1]++
		}
	}
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start, stop := max(0, i-diffContext), min(len(ops), hunkEnd(ops, i)+diffContext)
		fmt.Fprintf(b, "@@ -%s +%s @@\n",
			hunkSide(at+oldBefore[start], oldBefore[stop]-oldBefore[start]),
			hunkSide(at+newBefore[start], newBefore[stop]-newBefore[start]))
		for _, op := range ops[start:stop] {
			b.WriteString(string(op.kind) + op.text + "\n")
		}
		i = stop
	}
}

// hunkEnd is where the hunk holding the change at i ends: past its last change
// before a run of kept lines too long to bridge.
func hunkEnd(ops []diffOp, i int) int {
	end := i
	for end < len(ops) {
		if ops[end].kind != ' ' {
			end++
			continue
		}
		run := end
		for run < len(ops) && ops[run].kind == ' ' {
			run++
		}
		if run == len(ops) || run-end > 2*diffContext {
			break
		}
		end = run
	}
	return end
}

// hunkSide is one side of a hunk header; an empty side names the line before
// it, as git does.
func hunkSide(start, count int) string {
	if count == 0 {
		start--
	}
	return fmt.Sprintf("%d,%d", start, count)
}
