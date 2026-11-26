package codexprofile

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// EnablePlugin turns back on a plugin the config.toml in dir has turned off,
// the `enabled = false` Codex writes under `[plugins."<key>"]` (measured on
// 0.170.0). It reports whether it changed the file. A plugin with no table,
// or one not turned off, is left as written: this switches, it never installs.
func EnablePlugin(dir, key string) (bool, error) { return SetPlugin(dir, key, true) }

// SetPlugin turns the plugin key on or off in the config.toml in dir, which
// Codex has no CLI for (0.170.0 has `plugin add` and `remove` alone): the
// `enabled` line of its `[plugins."<key>"]` table, added under the header when
// turning off a table that has none, since a table with no line is on.
func SetPlugin(dir, key string, on bool) (bool, error) {
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
			text, changed = setEnabled(text, on)
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

// Plugins are the plugin@marketplace keys the config.toml in dir installs,
// each with whether it is on.
func Plugins(dir string) (map[string]bool, error) {
	data, err := readOptional(filepath.Join(dir, configFile))
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, t := range splitTables(data) {
		if len(t.path) == 2 && t.path[0] == "plugins" {
			value, _ := tableValue(t.text, "enabled")
			out[t.path[1]] = value != "false"
		}
	}
	return out, nil
}

// MCPServer is one `[mcp_servers.<name>]` table as the library screens show
// it: its command or URL. The rest of it comes from `codex mcp get --json`.
type MCPServer struct {
	Command, URL string
}

// MCPServers are the MCP servers the config.toml in dir declares.
func MCPServers(dir string) (map[string]MCPServer, error) {
	data, err := readOptional(filepath.Join(dir, configFile))
	if err != nil {
		return nil, err
	}
	out := map[string]MCPServer{}
	for _, t := range splitTables(data) {
		if len(t.path) == 2 && t.path[0] == "mcp_servers" {
			command, _ := tableValue(t.text, "command")
			url, _ := tableValue(t.text, "url")
			out[t.path[1]] = MCPServer{Command: unquote(command), URL: unquote(url)}
		}
	}
	return out, nil
}

// tableValue is the raw value of key's line in table, comment cut.
func tableValue(table, key string) (string, bool) {
	for line := range strings.SplitSeq(table, "\n") {
		k, rest, ok := parseKey(strings.TrimLeft(line, " \t"))
		if !ok || k != key {
			continue
		}
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		rest = strings.TrimSpace(rest[1:])
		if strings.HasPrefix(rest, `"`) || strings.HasPrefix(rest, "'") {
			return rest[:min(len(rest), skipString(rest, 0)+1)], true
		}
		value, _, _ := strings.Cut(rest, "#")
		return strings.TrimSpace(value), true
	}
	return "", false
}

// unquote is a one-line TOML string's text, "" for anything else.
func unquote(value string) string {
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) > 1 {
		return value[1 : len(value)-1]
	}
	text, err := strconv.Unquote(value)
	if err != nil {
		return ""
	}
	return text
}

// setEnabled rewrites a table's `enabled` line to on, or adds an `enabled =
// false` under its header when it has none; a table with no line is on.
func setEnabled(table string, on bool) (string, bool) {
	want := strconv.FormatBool(on)
	lines := strings.SplitAfter(table, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		key, _, ok := parseKey(trimmed)
		if !ok || key != "enabled" {
			continue
		}
		value, _ := tableValue(line, "enabled")
		if value == want {
			return table, false
		}
		indent := line[:len(line)-len(trimmed)]
		newline := line[len(strings.TrimRight(line, "\r\n")):]
		lines[i] = indent + "enabled = " + want + newline
		return strings.Join(lines, ""), true
	}
	if on || len(lines) == 0 {
		return table, false
	}
	head := lines[0]
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head + "enabled = false\n" + strings.Join(lines[1:], ""), true
}
