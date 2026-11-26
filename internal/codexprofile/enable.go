package codexprofile

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// EnablePlugin turns back on a plugin the config.toml in dir has turned off,
// the `enabled = false` Codex writes under `[plugins."<key>"]` (measured on
// 0.170.0). It reports whether it changed the file. A plugin with no table,
// or one not turned off, is left as written: this switches, it never installs.
func EnablePlugin(dir, key string) (bool, error) {
	path := filepath.Join(dir, configFile)
	data, err := readOptional(path)
	if err != nil || data == nil {
		return false, err
	}
	var out strings.Builder
	changed := false
	for _, t := range splitTables(data) {
		text := t.text
		if len(t.path) == 2 && t.path[0] == "plugins" && t.path[1] == key {
			text, changed = enableLine(text)
		}
		out.WriteString(text)
	}
	if !changed {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return true, profilestore.WriteFileAtomic(path, []byte(out.String()), info.Mode().Perm())
}

// enableLine rewrites a table's `enabled = false` line to true.
func enableLine(table string) (string, bool) {
	lines := strings.SplitAfter(table, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		key, rest, ok := parseKey(trimmed)
		if !ok || key != "enabled" {
			continue
		}
		value, _, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest), "=")), "#")
		if strings.TrimSpace(value) != "false" {
			return table, false
		}
		indent := line[:len(line)-len(trimmed)]
		newline := line[len(strings.TrimRight(line, "\r\n")):]
		lines[i] = indent + "enabled = true" + newline
		return strings.Join(lines, ""), true
	}
	return table, false
}
