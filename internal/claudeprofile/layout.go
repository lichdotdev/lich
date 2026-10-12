package claudeprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sharedEntries are the user's own Claude Code content, linked from every
// profile to Personal so an edit made under one account is there under all of
// them. Claude Code writes through a link to its target (measured on 2.1.296).
// Only entries Personal has are linked; EnsureShared links one created later.
var sharedEntries = []string{"settings.json", "CLAUDE.md", "agents", "commands", "skills", "hooks"}

// sharedPluginEntries are Personal's plugin downloads, linked so a profile does
// not clone every marketplace and plugin again. The two state files beside them
// stay per profile: an organization account may forbid plugins a personal one
// runs, so the installed set differs per account.
var sharedPluginEntries = []string{
	filepath.Join("plugins", "cache"),
	filepath.Join("plugins", "marketplaces"),
}

// sharedFiles are the entries that are files rather than directories, the ones
// Windows can only link with Developer Mode and otherwise copies.
var sharedFiles = map[string]bool{"settings.json": true, "CLAUDE.md": true}

// emptyInstalledPlugins is installed_plugins.json with nothing in it, in the
// version Claude Code 2.1.296 writes. Plugins arrive through `claude plugin
// install` run in the profile, so the organization's policy decides what lands.
const emptyInstalledPlugins = `{"version":2,"plugins":{}}` + "\n"

// build lays a new profile out at dir against Personal. It reports the shared
// entries it had to copy instead of link.
func build(dir string) ([]string, error) {
	personal, err := PersonalDir()
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o700); err != nil {
		return nil, err
	}
	for _, name := range sharedPluginEntries {
		if err := os.MkdirAll(filepath.Join(personal, name), 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", filepath.Join(personal, name), err)
		}
	}
	copied, err := linkShared(personal, dir)
	if err != nil {
		return nil, err
	}
	if err := copyGlobalConfig(dir); err != nil {
		return nil, err
	}
	if err := copyMarketplaces(personal, dir); err != nil {
		return nil, err
	}
	installed := filepath.Join(dir, "plugins", "installed_plugins.json")
	if err := writeFileAtomic(installed, []byte(emptyInstalledPlugins), 0o600); err != nil {
		return nil, err
	}
	return copied, nil
}

// linkShared links every shared entry Personal has and the profile lacks. An
// entry the profile already holds is left alone, link or not: Claude Code may
// have written a settings.json of its own there before Personal had one, and
// that file is the profile's now.
func linkShared(personal, dir string) ([]string, error) {
	var copied []string
	for _, name := range append(sharedEntries, sharedPluginEntries...) {
		target := filepath.Join(personal, name)
		path := filepath.Join(dir, name)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			continue
		}
		if !sharedFiles[name] {
			if err := linkDir(target, path); err != nil {
				return copied, fmt.Errorf("link %s: %w", path, err)
			}
			continue
		}
		wasCopied, err := linkFile(target, path)
		if err != nil {
			return copied, fmt.Errorf("link %s: %w", path, err)
		}
		if wasCopied {
			copied = append(copied, name)
		}
	}
	return copied, nil
}

// EnsureShared links into a profile the shared entries Personal gained since
// the profile was made — a CLAUDE.md written after it, say — and records any it
// had to copy. It runs before every spawn into the profile, so an entry appears
// on the next session. Personal needs nothing.
func EnsureShared(slug string) error {
	if IsPersonal(slug) {
		return nil
	}
	dir, err := ConfigDir(slug)
	if err != nil {
		return err
	}
	personal, err := PersonalDir()
	if err != nil {
		return err
	}
	copied, err := linkShared(personal, dir)
	if err != nil || len(copied) == 0 {
		return err
	}
	m, err := readMeta(dir)
	if err != nil {
		return err
	}
	m.Copied = append(m.Copied, copied...)
	return writeMeta(dir, m)
}

// globalDroppedKeys are the top-level keys of Claude Code's global config that
// describe an account or cache one, matched case-insensitively as substrings.
// Copying them would hand the new profile Personal's identity until its own
// login overwrote them.
var globalDroppedKeys = []string{
	"cache", "oauthaccount", "userid", "anonymousid", "additionalmodeloptions", "promo",
	"passes", "hasvisited", "hasavailablesubscription", "subscriptionnoticecount",
	"feedbacksurveystate", "penguinmodeorgenabled", "claudecodefirsttokendate",
	"claudeaimcpeverconnected",
}

// copyGlobalConfig writes the profile's global config: Personal's, minus the
// account and its caches, and minus each project's own history. A copy because
// Claude Code replaces the file through a temporary and a rename, which would
// turn a link into a file of its own on the first write anyway.
func copyGlobalConfig(dir string) error {
	src, err := personalGlobalFile()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	out, err := filterGlobalConfig(data)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	return writeFileAtomic(filepath.Join(dir, ".claude.json"), out, 0o600)
}

func filterGlobalConfig(data []byte) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for key := range doc {
		if containsAny(strings.ToLower(key), globalDroppedKeys) {
			delete(doc, key)
		}
	}
	if raw, ok := doc["projects"]; ok {
		projects, err := filterProjects(raw)
		if err != nil {
			return nil, err
		}
		doc["projects"] = projects
	}
	return json.MarshalIndent(doc, "", "  ")
}

// filterProjects keeps what a person decided about each project — the trust
// answer, the tools they allowed, the MCP servers — and drops what a session
// left behind there (last cost, last session id, example files).
func filterProjects(raw json.RawMessage) (json.RawMessage, error) {
	var projects map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &projects); err != nil {
		return nil, err
	}
	for _, project := range projects {
		for key := range project {
			lower := strings.ToLower(key)
			if key == "hasTrustDialogAccepted" || key == "allowedTools" || strings.Contains(lower, "mcp") {
				continue
			}
			delete(project, key)
		}
	}
	return json.Marshal(projects)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// copyMarketplaces writes the profile's known_marketplaces.json: Personal's,
// with each clone's location moved under the profile's own plugins directory,
// which links to the same clones. A marketplace read from a local directory
// keeps its path.
func copyMarketplaces(personal, dir string) error {
	src := filepath.Join(personal, "plugins", "known_marketplaces.json")
	data, err := os.ReadFile(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	var doc map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	clones := filepath.Join(dir, "plugins", "marketplaces")
	for name, entry := range doc {
		var location string
		if json.Unmarshal(entry["installLocation"], &location) != nil || !isClone(location) {
			continue
		}
		moved, err := json.Marshal(filepath.Join(clones, name))
		if err != nil {
			return err
		}
		entry["installLocation"] = moved
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "plugins", "known_marketplaces.json"), out, 0o600)
}

// isClone reports whether a marketplace location is a clone Claude Code made
// under some plugins/marketplaces directory, rather than a path the user gave.
func isClone(location string) bool {
	parent := filepath.Dir(location)
	return filepath.Base(parent) == "marketplaces" && filepath.Base(filepath.Dir(parent)) == "plugins"
}
