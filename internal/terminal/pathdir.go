package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// WithPathDir is env with dir ahead on its PATH, unless PATH already holds it:
// the folder an installer wrote a provider into, which lich resolves binaries
// through and hands its sessions from then on.
func WithPathDir(env []string, dir string) []string {
	out := slices.Clone(env)
	for i, kv := range out {
		v, ok := strings.CutPrefix(kv, "PATH=")
		if !ok {
			continue
		}
		if slices.Contains(filepath.SplitList(v), dir) {
			return out
		}
		out[i] = "PATH=" + dir + string(os.PathListSeparator) + v
		return out
	}
	return append(out, "PATH="+dir)
}
