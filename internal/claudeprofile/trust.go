package claudeprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/profilestore"
)

const trustKey = "hasTrustDialogAccepted"

// InheritTrust records in the global config of the profile whose directory is
// dir that the checkout top is trusted, when the person trusted it or origin,
// the main checkout top is a worktree of, in Personal. Personal ("") is the
// source and has nothing to inherit. Nothing is written when the profile
// already trusts either, has no global config yet, or Personal trusts neither:
// it never trusts a folder nobody trusted.
//
// A profile's global config is a copy of Personal's made once (layout.go), so
// every folder trusted in Personal after that asks again in the profile.
// Claude Code finds the answer for a subfolder in its checkout's top and for a
// linked worktree in its main checkout (measured on 2.1.302), which is why the
// two are all this asks about.
//
// Claude Code re-reads the file before each write of its own (measured on
// 2.1.302), so a running session keeps the entry. What one writes between this
// read and the rename is lost; the same window its own writes have.
func InheritTrust(dir, top, origin string) error {
	if dir == "" || top == "" {
		return nil
	}
	path := filepath.Join(dir, ".claude.json")
	doc, projects, err := readGlobalConfig(path)
	if doc == nil || err != nil {
		return err
	}
	key, originKey := configKey(top), configKey(origin)
	if trusted(projects, key) || trusted(projects, originKey) {
		return nil
	}
	personalPath, err := personalGlobalFile()
	if err != nil {
		return err
	}
	_, personal, err := readGlobalConfig(personalPath)
	if err != nil || !(trusted(personal, key) || trusted(personal, originKey)) {
		return err
	}
	if projects[key] == nil {
		projects[key] = map[string]json.RawMessage{}
	}
	projects[key][trustKey] = json.RawMessage("true")
	return writeGlobalConfig(path, doc, projects)
}

// readGlobalConfig is a global config and its projects table, both nil for a
// file that does not exist.
func readGlobalConfig(path string) (map[string]json.RawMessage, map[string]map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	doc, projects, err := parseGlobalConfig(data)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	return doc, projects, nil
}

// configKey is how Claude Code spells a folder in its projects table: forward
// slashes on Windows too.
func configKey(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(path)
}

func trusted(projects map[string]map[string]json.RawMessage, key string) bool {
	return key != "" && string(projects[key][trustKey]) == "true"
}

func parseGlobalConfig(data []byte) (map[string]json.RawMessage, map[string]map[string]json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, err
	}
	projects := map[string]map[string]json.RawMessage{}
	if raw, ok := doc["projects"]; ok {
		if err := json.Unmarshal(raw, &projects); err != nil {
			return nil, nil, err
		}
	}
	return doc, projects, nil
}

// writeGlobalConfig keeps the file's mode, whatever Claude Code gave it.
func writeGlobalConfig(path string, doc map[string]json.RawMessage, projects map[string]map[string]json.RawMessage) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	raw, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	doc["projects"] = raw
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return profilestore.WriteFileAtomic(path, out, info.Mode().Perm())
}
