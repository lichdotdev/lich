package library

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// The providers' own plugins, as the Plugins screen lists them: what each
// loads in the screen's scope and a switch through the provider's own CLI.
// lich lists and switches them but never holds or moves one. Crush and Kiro
// CLI take no plugins, and Cursor CLI loads Claude Code's (docs/ceilings.md).

// lichPlugin is whether id names lich's own plugin, which the screen shows
// first on its own.
func lichPlugin(id string) bool {
	name, _, _ := strings.Cut(id, "@")
	return name == "lich"
}

// pluginSet gathers plugins by id across providers.
type pluginSet map[string]*api.LibraryItem

func (set pluginSet) add(id, version string, sw api.LibrarySwitch) {
	if lichPlugin(id) {
		return
	}
	it := set[id]
	if it == nil {
		it = &api.LibraryItem{Kind: api.LibraryPluginKind, Name: id}
		set[id] = it
	}
	if it.Description == "" {
		it.Description = version
	}
	it.Switches = append(it.Switches, sw)
}

func (set pluginSet) list() []api.LibraryItem {
	out := make([]api.LibraryItem, 0, len(set))
	for _, it := range set {
		out = append(out, *it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// plugins are the providers' plugins loaded in project's scope, the user's
// alone for "".
func (s *Service) plugins(project string) []api.LibraryItem {
	set := pluginSet{}
	s.claudePlugins(project, set)
	s.codexPlugins(set)
	s.antigravityPlugins(set)
	s.ompPlugins(set)
	s.opencodePlugins(project, set)
	return set.list()
}

// profileSlugs are provider's profiles that each get a switch, or one "" for
// Personal alone.
func (s *Service) profileSlugs(provider string) []string {
	list := s.profilesOf(provider)
	if len(list) == 0 {
		return []string{""}
	}
	slugs := make([]string, len(list))
	for i, p := range list {
		slugs[i] = p.Slug
	}
	return slugs
}

// claudePlugins are Claude Code's: in the user's scope each profile's
// enabledPlugins; in a project's, Personal's as the project's
// .claude/settings.json and then settings.local.json override it, which is
// how Claude Code resolves them (measured on 2.1.332), behind one switch.
func (s *Service) claudePlugins(project string, set pluginSet) {
	if project == "" {
		for _, slug := range s.profileSlugs(providers.Claude) {
			dir, err := claudeprofile.ConfigDir(slug)
			if err != nil {
				continue
			}
			installed := claudeInstalled(claudeprofile.PluginsDir(slug, dir))
			for id, on := range enabledPlugins(filepath.Join(dir, "settings.json")) {
				if version, ok := installed[id]; ok {
					set.add(id, version, api.LibrarySwitch{Provider: providers.Claude, Profile: slug, On: on})
				}
			}
		}
		return
	}
	dir, err := claudeprofile.PersonalDir()
	if err != nil {
		return
	}
	installed := claudeInstalled(claudeprofile.PluginsDir("", dir))
	on := enabledPlugins(filepath.Join(dir, "settings.json"))
	for _, name := range []string{"settings.json", "settings.local.json"} {
		maps.Copy(on, enabledPlugins(filepath.Join(project, ".claude", name)))
	}
	reason := ""
	if s.committed(project, filepath.Join(".claude", "settings.local.json")) {
		reason = api.LibraryCommitted
	}
	for id, v := range on {
		if version, ok := installed[id]; ok {
			set.add(id, version, api.LibrarySwitch{Provider: providers.Claude, On: v, Reason: reason})
		}
	}
}

// claudeInstalled are the plugins installed_plugins.json in a plugins
// directory records, at any scope, by key, with a version.
func claudeInstalled(dir string) map[string]string {
	var doc struct {
		Plugins map[string][]struct {
			Version string `json:"version"`
		} `json:"plugins"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "installed_plugins.json"))
	if err != nil || json.Unmarshal(data, &doc) != nil {
		return nil
	}
	out := map[string]string{}
	for key, installs := range doc.Plugins {
		if len(installs) > 0 {
			out[key] = installs[0].Version
		}
	}
	return out
}

// enabledPlugins is a Claude Code settings file's enabledPlugins.
func enabledPlugins(path string) map[string]bool {
	var doc struct {
		Enabled map[string]bool `json:"enabledPlugins"`
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &doc) != nil || doc.Enabled == nil {
		return map[string]bool{}
	}
	return doc.Enabled
}

// codexPlugins are each profile's [plugins] tables; Codex installs plugins
// for the user alone (0.170.0).
func (s *Service) codexPlugins(set pluginSet) {
	for _, slug := range s.profileSlugs(providers.Codex) {
		dir, err := codexDir(slug)
		if err != nil {
			continue
		}
		list, _ := codexprofile.Plugins(dir)
		for id, on := range list {
			set.add(id, "", api.LibrarySwitch{Provider: providers.Codex, Profile: slug, On: on})
		}
	}
}

func codexDir(slug string) (string, error) {
	if codexprofile.IsPersonal(slug) {
		return codexprofile.PersonalDir()
	}
	return codexprofile.ConfigDir(slug)
}

// antigravityPlugins are the folders under ~/.gemini/config/plugins, each on
// unless config.json beside them turns it off (measured on 1.5.15).
func (s *Service) antigravityPlugins(set pluginSet) {
	home, err := s.home()
	if err != nil {
		return
	}
	root := filepath.Join(home, ".gemini", "config")
	entries, err := os.ReadDir(filepath.Join(root, "plugins"))
	if err != nil {
		return
	}
	var config struct {
		Plugins map[string]struct {
			Enabled *bool `json:"enabled"`
		} `json:"plugins"`
	}
	if data, err := os.ReadFile(filepath.Join(root, "config.json")); err == nil {
		_ = json.Unmarshal(data, &config)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var manifest struct {
			Version string `json:"version"`
		}
		if data, err := os.ReadFile(filepath.Join(root, "plugins", e.Name(), "plugin.json")); err == nil {
			_ = json.Unmarshal(data, &manifest)
		}
		enabled := config.Plugins[e.Name()].Enabled
		set.add(e.Name(), manifest.Version, api.LibrarySwitch{Provider: providers.Antigravity, On: enabled == nil || *enabled})
	}
}

// ompPlugins are the ones oh-my-pi's lock records and node_modules holds, the
// two `omp plugin list` reads (19.1.1); every profile shares them.
func (s *Service) ompPlugins(set pluginSet) {
	home, err := s.home()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".omp", "plugins")
	var lock struct {
		Plugins map[string]struct {
			Version string `json:"version"`
			Enabled bool   `json:"enabled"`
		} `json:"plugins"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "omp-plugins.lock.json"))
	if err != nil || json.Unmarshal(data, &lock) != nil {
		return
	}
	for name, p := range lock.Plugins {
		if _, err := os.Stat(filepath.Join(dir, "node_modules", filepath.FromSlash(name))); err == nil {
			set.add(name, p.Version, api.LibrarySwitch{Provider: providers.OMP, On: p.Enabled})
		}
	}
}

// opencodePlugins are the `plugin` lists of opencode's config files and the
// files of its plugin folders, the user's and project's; opencode has no
// switch short of removing one (1.19.4).
func (s *Service) opencodePlugins(project string, set pluginSet) {
	home, err := s.home()
	if err != nil {
		return
	}
	dirs := []string{filepath.Join(xdgConfig(home), "opencode")}
	if project != "" {
		dirs = append(dirs, project, filepath.Join(project, ".opencode"))
	}
	for _, dir := range dirs {
		for _, doc := range opencodeDocs(dir) {
			config, err := readDoc(doc.path)
			if err != nil {
				continue
			}
			list, _ := config["plugin"].([]any)
			for _, v := range list {
				if spec, ok := v.(string); ok {
					set.add(spec, "", api.LibrarySwitch{Provider: providers.OpenCode, On: true, Reason: api.LibraryNoSwitch})
				}
			}
		}
		for _, folder := range []string{"plugin", "plugins"} {
			entries, _ := os.ReadDir(filepath.Join(dir, folder))
			for _, e := range entries {
				if ext := filepath.Ext(e.Name()); !e.IsDir() && (ext == ".js" || ext == ".ts") {
					set.add(e.Name(), "", api.LibrarySwitch{Provider: providers.OpenCode, On: true, Reason: api.LibraryNoSwitch})
				}
			}
		}
	}
}

// setPlugin turns plugin opts.Name on or off through its provider's own CLI.
func (s *Service) setPlugin(opts api.LibrarySwitchOptions) error {
	verb := "disable"
	if opts.On {
		verb = "enable"
	}
	switch opts.Provider {
	case providers.Claude:
		return s.setClaudePlugin(opts, verb)
	case providers.Codex:
		dir, err := codexDir(opts.Profile)
		if err != nil {
			return err
		}
		_, err = codexprofile.SetPlugin(dir, opts.Name, opts.On)
		return err
	case providers.Antigravity, providers.OMP:
		_, err := s.cli(command{provider: opts.Provider, args: []string{"plugin", verb, opts.Name}})
		return err
	}
	return fmt.Errorf("%s has no switch for its plugins", opts.Provider)
}

// setClaudePlugin runs `claude plugin enable|disable`: at user scope in the
// profile's config, or in a project at local scope, which writes the
// project's .claude/settings.local.json, never the settings.json it commits.
func (s *Service) setClaudePlugin(opts api.LibrarySwitchOptions, verb string) error {
	c := command{provider: providers.Claude, args: []string{"plugin", verb, opts.Name, "--scope", "user"}}
	if opts.Project != "" {
		if s.committed(opts.Project, filepath.Join(".claude", "settings.local.json")) {
			return fmt.Errorf("%s commits .claude/settings.local.json, which lich does not write", opts.Project)
		}
		c.dir, c.args[4] = opts.Project, "local"
	} else if !claudeprofile.IsPersonal(opts.Profile) {
		dir, err := claudeprofile.ConfigDir(opts.Profile)
		if err != nil {
			return err
		}
		c.env = []string{"CLAUDE_CONFIG_DIR=" + dir}
	}
	_, err := s.cli(c)
	return err
}

// committed is whether the repository at project tracks path, which lich
// never writes; a folder git does not know commits nothing.
func (s *Service) committed(project, path string) bool {
	out, err := s.cli(command{provider: gitTool, dir: project, args: []string{"ls-files", "--", filepath.ToSlash(path)}})
	return err == nil && strings.TrimSpace(string(out)) != ""
}
