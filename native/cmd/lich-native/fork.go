package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/lichclient"
)

// forkableKinds are providers.SupportsFork: the providers whose own CLI
// branches a conversation (`claude --fork-session`, `codex fork`,
// `opencode --fork`). The other five only resume, and lich forges no fork for
// them (docs/ceilings.md).
var forkableKinds = []string{"claude", "codex", "opencode"}

func canFork(kind string) bool { return slices.Contains(forkableKinds, kind) }

// forkLabel names a fork of the session called parent, among the labels
// already taken in its project: the parent's name without its trailing
// number, then the first number above the parent's own that no card holds.
// "auth-refactor" forks into "auth-refactor 2", and "auth-refactor 2" into
// "auth-refactor 3" unless a 3 is already there.
func forkLabel(parent string, taken []string) string {
	base, n := splitNumber(parent)
	for {
		n++
		label := fmt.Sprintf("%s %d", base, n)
		if !slices.Contains(taken, label) {
			return label
		}
	}
}

// splitNumber is label without a trailing " N", and N, 1 for a label with
// none: the bare name is the first of its line.
func splitNumber(label string) (string, int) {
	i := strings.LastIndexByte(label, ' ')
	if i < 0 {
		return label, 1
	}
	n, err := strconv.Atoi(label[i+1:])
	if err != nil || n < 1 {
		return label, 1
	}
	return label[:i], n
}

// forkSource is the session a fork branches: what the child is filed and
// named after, and the conversation its first start copies.
type forkSource struct {
	id, label, kind, conversation, folder string
	// path is the session's checkout as stored, "" for the project's own.
	path string
}

func forkSourceOf(s lichclient.Session) forkSource {
	return forkSource{id: s.ID, label: s.Label, kind: s.Kind, conversation: s.ProviderSessionID, folder: s.Folder, path: s.Path}
}

// labelsOf are the names a project's open sessions go by.
func labelsOf(p lichclient.Project) []string {
	labels := make([]string, len(p.Sessions))
	for i, s := range p.Sessions {
		labels[i] = s.Label
	}
	return labels
}

// placedAfter is ids with id moved to sit right after parent, so a fork
// lands under the card it came from rather than at the end; ids unchanged
// when parent is not among them.
func placedAfter(ids []string, id, parent string) []string {
	out := slices.DeleteFunc(slices.Clone(ids), func(x string) bool { return x == id })
	at := slices.Index(out, parent)
	if at < 0 {
		return append(out, id)
	}
	return slices.Insert(out, at+1, id)
}
