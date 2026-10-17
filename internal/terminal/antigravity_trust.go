package terminal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/omartelo/lich/internal/profilestore"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
)

const antigravityTrustKey = "trustedWorkspaces"

// inheritAntigravityTrust adds the session's folder to Antigravity's trusted
// list when the person trusted its checkout's top or the main checkout of the
// worktree it is, and for every confined session, so neither opens on the
// "trust this folder?" question. Antigravity trusts a folder only by its exact
// path, so every worktree and subfolder of a trusted repository asked again
// (measured on 1.3.5). The entry stays: the folder opens without the question
// outside lich too. A failure costs the session the question, never its spawn.
func inheritAntigravityTrust(kind, cwd string, confined bool) {
	if kind != providers.Antigravity || cwd == "" {
		return
	}
	if err := trustAntigravityFolder(userHome(), cwd, confined); err != nil {
		slog.Warn("carry folder trust into Antigravity", "cwd", cwd, "err", err)
	}
}

func trustAntigravityFolder(home, cwd string, confined bool) error {
	if home == "" {
		return errors.New("no home directory to find Antigravity's settings under")
	}
	folder, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return err
	}
	path := antigravitySettingsFile(home)
	doc, trusted, err := readAntigravitySettings(path)
	if err != nil || slices.Contains(trusted, folder) {
		return err
	}
	top, origin := project.CheckoutRoots(folder)
	inherited := slices.ContainsFunc([]string{top, origin}, func(dir string) bool {
		return dir != "" && slices.Contains(trusted, filepath.Clean(dir))
	})
	if !confined && !inherited {
		return nil
	}
	raw, err := json.Marshal(append(trusted, folder))
	if err != nil {
		return err
	}
	doc[antigravityTrustKey] = raw
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	return profilestore.WriteFileAtomic(path, out, 0o644)
}

// antigravitySettingsFile is where Antigravity keeps its trusted folders (not
// ~/.gemini/trustedFolders.json, which is the Gemini CLI's). No environment
// variable moves it.
func antigravitySettingsFile(home string) string {
	return filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
}

// readAntigravitySettings is the settings file and its trusted folders, an
// empty document for a file that is not there yet. Every other key is kept as
// it was: Antigravity merges what it finds on disk (measured on 1.3.5).
func readAntigravitySettings(path string) (map[string]json.RawMessage, []string, error) {
	doc := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var trusted []string
	if raw, ok := doc[antigravityTrustKey]; ok {
		if err := json.Unmarshal(raw, &trusted); err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, err)
		}
	}
	return doc, trusted, nil
}
