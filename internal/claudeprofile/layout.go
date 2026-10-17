package claudeprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// emptyInstalledPlugins is installed_plugins.json with nothing in it, in the
// version Claude Code 2.1.296 writes. Plugins arrive through `claude plugin
// install` run in the profile, so the organization's policy decides what lands.
const emptyInstalledPlugins = `{"version":2,"plugins":{}}` + "\n"

// seed writes what a profile holds as files of its own beside the links: the
// global config minus the account, the known marketplaces moved under the
// profile's plugins directory and an empty installed set.
func seed(personal, dir string) error {
	if err := copyGlobalConfig(dir); err != nil {
		return err
	}
	if err := copyMarketplaces(personal, dir); err != nil {
		return err
	}
	installed := filepath.Join(dir, "plugins", "installed_plugins.json")
	return profilestore.WriteFileAtomic(installed, []byte(emptyInstalledPlugins), 0o600)
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
	return profilestore.WriteFileAtomic(filepath.Join(dir, ".claude.json"), out, 0o600)
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

// filterProjects keeps what a person decided about each project (the trust
// answer, the tools they allowed, the MCP servers) and drops what a session
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
	personalPlugins := personalPluginsDir(personal)
	src := filepath.Join(personalPlugins, "known_marketplaces.json")
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
		if json.Unmarshal(entry["installLocation"], &location) != nil || !isClone(location, personalPlugins) {
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
	return profilestore.WriteFileAtomic(filepath.Join(dir, "plugins", "known_marketplaces.json"), out, 0o600)
}

// isClone reports whether a marketplace location is a clone Claude Code made
// under some plugins/marketplaces directory, or under Personal's plugins
// directory wherever pluginsDirVar moved it, rather than a path the user gave.
func isClone(location, personalPlugins string) bool {
	parent := filepath.Dir(location)
	if parent == filepath.Join(personalPlugins, "marketplaces") {
		return true
	}
	return filepath.Base(parent) == "marketplaces" && filepath.Base(filepath.Dir(parent)) == "plugins"
}
