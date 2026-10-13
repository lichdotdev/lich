package codexprofile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/omartelo/lich/internal/profilestore"
)

// configFile is where Codex keeps its user config, the plugin set included, in
// its home. Codex reads no other per-home file: the system layers are under
// /etc/codex, and a managed_config.toml in CODEX_HOME is ignored (measured on
// 0.162.1).
const configFile = "config.toml"

// downloads are where a profile keeps its plugins and marketplace snapshots,
// its own since plugins are per profile. A profile made when they were linked
// to Personal's has the links removed (syncConfig), so a `plugin remove` or the
// lich plugin's pinned reinstall run there never deletes Personal's copy.
var downloads = []string{filepath.Join("plugins", "cache"), filepath.Join(".tmp", "marketplaces")}

// tableClass is who decides a table of a profile's config.toml.
type tableClass int

const (
	// fromPersonal tables are Personal's: model settings, features, MCP
	// servers, config hooks. The profile's own copy is replaced at every spawn.
	fromPersonal tableClass = iota
	// fromProfile tables are the plugin set: what is installed and the
	// marketplaces it came from. Personal's never reach the profile.
	fromProfile
	// fromBoth tables are answers given while working (a project's trust, a
	// hook's trust, a dismissed notice): Personal's are taken, and the
	// profile keeps the ones Personal has not given, so a profile is not asked
	// again at every spawn.
	fromBoth
)

func classify(path []string) tableClass {
	if len(path) == 0 {
		return fromPersonal
	}
	switch path[0] {
	case "plugins", "marketplaces":
		return fromProfile
	case "projects", "notice":
		return fromBoth
	case "hooks":
		if len(path) > 1 && path[1] == "state" {
			return fromBoth
		}
	}
	return fromPersonal
}

// syncConfig rebuilds a profile's config.toml from Personal's and its own
// (composeConfig), the Sync of the profile store. A config.toml that is still a
// link to Personal's, from a profile made when the whole file was shared, is
// taken as holding nothing of the profile's, and its download links go with it.
func syncConfig(personal, dir string) error {
	shared, err := readOptional(filepath.Join(personal, configFile))
	if err != nil {
		return err
	}
	path := filepath.Join(dir, configFile)
	var own []byte
	if profilestore.IsLink(path) {
		if err := unlinkDownloads(dir); err != nil {
			return err
		}
	} else if own, err = readOptional(path); err != nil {
		return err
	}
	out := composeConfig(shared, own)
	if bytes.Equal(out, own) && !profilestore.IsLink(path) {
		return nil
	}
	return profilestore.WriteFileAtomic(path, out, 0o600)
}

func unlinkDownloads(dir string) error {
	for _, name := range downloads {
		path := filepath.Join(dir, name)
		if !profilestore.IsLink(path) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("unlink %s: %w", path, err)
		}
	}
	return nil
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// composeConfig is a profile's config.toml: Personal's tables but its plugin
// set, then the profile's plugin set and the answers only the profile has given
// (classify). Tables are copied as they are written, comments included; the
// profile's top-level keys are dropped, since those are Personal's.
func composeConfig(personal, profile []byte) []byte {
	var out bytes.Buffer
	given := map[string]bool{}
	for _, t := range splitTables(personal) {
		if classify(t.path) == fromProfile {
			continue
		}
		given[t.key()] = true
		writeTable(&out, t.text)
	}
	for _, t := range splitTables(profile) {
		switch classify(t.path) {
		case fromProfile:
			writeTable(&out, t.text)
		case fromBoth:
			if !given[t.key()] {
				writeTable(&out, t.text)
			}
		}
	}
	return out.Bytes()
}

func writeTable(out *bytes.Buffer, text string) {
	if text == "" {
		return
	}
	if out.Len() > 0 && !bytes.HasSuffix(out.Bytes(), []byte("\n\n")) && strings.HasPrefix(text, "[") {
		out.WriteByte('\n')
	}
	out.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		out.WriteByte('\n')
	}
}

// table is one table of a TOML document as written: its header line and every
// line up to the next header. The keys above the first header are a table with
// no path.
type table struct {
	path  []string
	array bool
	text  string
}

func (t table) key() string {
	key := strings.Join(t.path, "\x00")
	if t.array {
		return "[[" + key
	}
	return key
}

// installedPlugins are the plugin@marketplace keys a config.toml has a
// [plugins."<key>"] table for, which is how Codex records an install.
func installedPlugins(config []byte) []string {
	var keys []string
	for _, t := range splitTables(config) {
		if len(t.path) == 2 && t.path[0] == "plugins" {
			keys = append(keys, t.path[1])
		}
	}
	return keys
}

// marketplaceTables are the tables declaring the marketplace name, as written.
func marketplaceTables(config []byte, name string) string {
	var out strings.Builder
	for _, t := range splitTables(config) {
		if len(t.path) >= 2 && t.path[0] == "marketplaces" && t.path[1] == name {
			out.WriteString(t.text)
			if !strings.HasSuffix(t.text, "\n") {
				out.WriteByte('\n')
			}
		}
	}
	return out.String()
}

// splitTables cuts a TOML document into its tables at every header line. A
// line is a header only outside a multi-line string or array, which is all the
// parsing this needs: the tables themselves are copied, never read.
func splitTables(doc []byte) []table {
	var tables []table
	current := table{}
	var text strings.Builder
	var scan scanState
	for _, line := range strings.SplitAfter(string(doc), "\n") {
		if scan.atTopLevel() {
			if path, array, ok := parseHeader(line); ok {
				current.text = text.String()
				tables = append(tables, current)
				current, text = table{path: path, array: array}, strings.Builder{}
			}
		}
		text.WriteString(line)
		scan.feed(line)
	}
	current.text = text.String()
	tables = append(tables, current)
	return tables
}

// scanState is where a TOML scan is between lines: inside a multi-line string
// (its closing quotes), or inside how many open arrays and inline tables.
type scanState struct {
	multi string
	depth int
}

func (s *scanState) atTopLevel() bool { return s.multi == "" && s.depth == 0 }

func (s *scanState) feed(line string) {
	for i := 0; i < len(line); i++ {
		if s.multi != "" {
			i = s.skipMulti(line, i)
			continue
		}
		switch c := line[i]; c {
		case '#':
			return
		case '"', '\'':
			quotes := string([]byte{c, c, c})
			if strings.HasPrefix(line[i:], quotes) {
				s.multi = quotes
				i += 2
				continue
			}
			i = skipString(line, i)
		case '[', '{':
			s.depth++
		case ']', '}':
			s.depth--
		}
	}
}

// skipMulti moves past line[i] inside a multi-line string, and past the
// closing quotes when they start there. It returns the last index consumed.
func (s *scanState) skipMulti(line string, i int) int {
	if s.multi == `"""` && line[i] == '\\' {
		return i + 1
	}
	if strings.HasPrefix(line[i:], s.multi) {
		s.multi = ""
		return i + 2
	}
	return i
}

// skipString returns the index of the quote closing the one-line string that
// opens at line[i], or the line's end when it does not close.
func skipString(line string, i int) int {
	quote := line[i]
	for j := i + 1; j < len(line); j++ {
		switch line[j] {
		case '\\':
			if quote == '"' {
				j++
			}
		case quote:
			return j
		}
	}
	return len(line)
}

// parseHeader reads a [table] or [[array]] header line into its key path,
// quoted keys unquoted.
func parseHeader(line string) ([]string, bool, bool) {
	rest := strings.TrimSpace(line)
	if !strings.HasPrefix(rest, "[") {
		return nil, false, false
	}
	array := strings.HasPrefix(rest, "[[")
	rest = rest[1:]
	if array {
		rest = rest[1:]
	}
	var path []string
	for {
		rest = strings.TrimLeft(rest, " \t")
		key, after, ok := parseKey(rest)
		if !ok {
			return nil, false, false
		}
		path = append(path, key)
		rest = strings.TrimLeft(after, " \t")
		switch {
		case strings.HasPrefix(rest, "."):
			rest = rest[1:]
		case strings.HasPrefix(rest, "]"):
			return path, array, true
		default:
			return nil, false, false
		}
	}
}

// parseKey reads one key, bare, "basic" or 'literal', off the front of s.
func parseKey(s string) (key, rest string, ok bool) {
	if s == "" {
		return "", "", false
	}
	switch s[0] {
	case '"':
		end := skipString(s, 0)
		if end >= len(s) {
			return "", "", false
		}
		key, err := strconv.Unquote(s[:end+1])
		return key, s[end+1:], err == nil
	case '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return "", "", false
		}
		return s[1 : end+1], s[end+2:], true
	}
	end := strings.IndexFunc(s, func(r rune) bool {
		return !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if end <= 0 {
		return "", "", false
	}
	return s[:end], s[end:], true
}
