package main

// What the Code tab's tree reads from the session's listing: the name filter,
// the line counts by path and the note a listing that left something out owes.

import (
	"strings"

	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui/filetree"
)

func filterPaths(paths []string, query string) []string {
	if strings.TrimSpace(query) == "" {
		return paths
	}
	var out []string
	for _, p := range paths {
		if matchesQuery(p, query) {
			out = append(out, p)
		}
	}
	return out
}

// diffStatsByPath keys each changed file's counts by its current path.
func diffStatsByPath(files []godemirror.FilePatch) map[string]filetree.Delta {
	out := make(map[string]filetree.Delta, len(files))
	for _, f := range files {
		out[f.Path] = filetree.Delta{Added: f.Added, Deleted: f.Deleted}
	}
	return out
}

// treeFootnote is what a listing that is not the whole folder owes the
// reader: the directories the walk stepped over, then whether it was cut.
func treeFootnote(cut bool, hidden []string) string {
	var parts []string
	if len(hidden) > 0 {
		parts = append(parts, i18n.T("git.fileTree.hidden", i18n.Args{"names": strings.Join(hidden, ", ")}))
	}
	if cut {
		parts = append(parts, i18n.T("git.fileTree.cut"))
	}
	return strings.Join(parts, " ")
}
