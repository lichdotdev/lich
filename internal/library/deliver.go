package library

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// Handing the items a provider's session has on to it, by the route phase 1
// measured for each (docs/ceilings.md): per session where the provider takes
// it (a plugin folder for Claude Code and Cursor CLI, opencode's inline config,
// an omp --config overlay, a composed CRUSH_SKILLS_DIR for Crush, MCP servers
// on Claude Code's, Codex's and opencode's own spawn routes), and written into
// the provider's config where it takes none (skills for Codex, Antigravity and
// Kiro CLI, MCP servers for oh-my-pi and Cursor CLI), where lich marks what it
// wrote and removes only that.

// Delivery is what one spawn hands its provider.
type Delivery struct {
	// PluginDir is a Claude Code plugin holding the skills, for --plugin-dir.
	PluginDir string
	// SkillsDir is the folder of skills, for opencode's skills.paths.
	SkillsDir string
	// OMPConfig is an omp --config overlay naming SkillsDir.
	OMPConfig string
	// CrushSkills is the folder for CRUSH_SKILLS_DIR: Crush's own global
	// skills and the library's, since the variable replaces Crush's search.
	CrushSkills string
	// MCP are the servers for the providers that take them at spawn.
	MCP map[string]Server
}

const (
	buildDir        = "build"
	pluginName      = "lich-library"
	writtenMCPKey   = "library.mcp.written."
	deliveredMarkTx = "lich writes this folder from its library and removes it when the skill is turned off\n"
)

// Deliver works out provider's items for a session in profile (its slug, ""
// for Personal), whose own config directory is profileDir ("" for
// Personal's), and writes what the provider can only read from its config. A
// failure costs the session those items, never its spawn.
func (s *Service) Deliver(provider, profile, profileDir string) Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	var d Delivery
	skills := s.onSkills(provider, profile)
	switch provider {
	case providers.Claude, providers.Cursor, providers.OpenCode, providers.OMP:
		if len(skills) > 0 {
			if dir, err := s.build(skills); err == nil {
				d.SkillsDir = filepath.Join(dir, skillsDir)
				d.PluginDir = dir
				d.OMPConfig = filepath.Join(dir, "omp.yml")
			} else {
				slog.Warn("library: build the skills", "provider", provider, "err", err)
			}
		}
	case providers.Crush:
		if len(skills) > 0 {
			if dir, err := s.buildCrush(skills); err == nil {
				d.CrushSkills = dir
			} else {
				slog.Warn("library: compose Crush's skills", "err", err)
			}
		}
	case providers.Codex, providers.Antigravity, providers.Kiro:
		if err := s.writeSkills(s.deliveryDir(provider, profileDir), provider, profile); err != nil {
			slog.Warn("library: write the skills", "provider", provider, "err", err)
		}
	}
	switch provider {
	case providers.Claude, providers.Codex, providers.OpenCode:
		d.MCP = s.onServers(provider, profile)
	case providers.OMP, providers.Cursor:
		for _, doc := range s.mcpDocs(provider) {
			if err := s.writeServersInto(doc, provider, s.onServers(provider, "")); err != nil {
				slog.Warn("library: write the MCP servers", "provider", provider, "err", err)
			}
		}
	}
	return d
}

// onSkills are the library skills on for provider in profile.
func (s *Service) onSkills(provider, profile string) []skill {
	all, err := s.librarySkills()
	if err != nil {
		return nil
	}
	return slices.DeleteFunc(all, func(sk skill) bool { return !s.On(api.LibrarySkill, sk.name, provider, profile) })
}

// onServers are the library servers on for provider in profile.
func (s *Service) onServers(provider, profile string) map[string]Server {
	all, err := s.libraryServers()
	if err != nil {
		return nil
	}
	out := map[string]Server{}
	for name, sv := range all {
		if s.On(api.LibraryMCP, name, provider, profile) {
			out[name] = sv
		}
	}
	return out
}

// build lays the skills out once per set of names, as a Claude Code plugin
// whose skills folder opencode and omp read too, and answers its folder. A
// folder is never rewritten once built, so a session keeps the one it started
// with.
func (s *Service) build(skills []skill) (string, error) {
	lib, err := s.libraryDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(lib, buildDir, setKey(skills, nil))
	if _, err := os.Stat(filepath.Join(dir, "omp.yml")); err == nil {
		return dir, nil
	}
	tmp := dir + ".tmp"
	_ = os.RemoveAll(tmp)
	for _, sk := range skills {
		if err := copyDir(sk.dir, filepath.Join(tmp, skillsDir, filepath.Base(sk.dir))); err != nil {
			return "", err
		}
	}
	manifest := `{"name":"` + pluginName + `","description":"Skills from lich's library"}` + "\n"
	if err := writeFile(filepath.Join(tmp, ".claude-plugin", "plugin.json"), []byte(manifest)); err != nil {
		return "", err
	}
	quoted, _ := json.Marshal(filepath.Join(dir, skillsDir))
	if err := writeFile(filepath.Join(tmp, "omp.yml"), []byte("skills:\n  customDirectories:\n    - "+string(quoted)+"\n")); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		if _, statErr := os.Stat(dir); statErr == nil {
			return dir, nil
		}
		return "", err
	}
	return dir, nil
}

// buildCrush composes the folder CRUSH_SKILLS_DIR names: a link to each skill
// of Crush's own global folders, the first of a name winning as in Crush's own
// order, and the library's copies, which take their names over.
func (s *Service) buildCrush(skills []skill) (string, error) {
	home, err := s.home()
	if err != nil {
		return "", err
	}
	theirs := map[string]string{}
	var names []string
	for _, dir := range crushSkillDirs(home) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if _, ok := theirs[e.Name()]; !ok {
				theirs[e.Name()] = filepath.Join(dir, e.Name())
				names = append(names, e.Name())
			}
		}
	}
	lib, err := s.libraryDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(lib, buildDir, "crush-"+setKey(skills, names))
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}
	tmp := dir + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}
	ours := map[string]bool{}
	for _, sk := range skills {
		ours[filepath.Base(sk.dir)] = true
		if err := copyDir(sk.dir, filepath.Join(tmp, filepath.Base(sk.dir))); err != nil {
			return "", err
		}
	}
	for _, name := range names {
		if ours[name] {
			continue
		}
		// shortcut: needs symlinks, which Windows grants only to some accounts;
		// there the session goes without the library's skills until a copy is wanted.
		if err := os.Symlink(theirs[name], filepath.Join(tmp, name)); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		if _, statErr := os.Stat(dir); statErr == nil {
			return dir, nil
		}
		return "", err
	}
	return dir, nil
}

// setKey names a built folder by what is in it.
func setKey(skills []skill, extra []string) string {
	h := sha256.New()
	for _, sk := range skills {
		info, err := os.Stat(filepath.Join(sk.dir, skillFile))
		stamp := ""
		if err == nil {
			stamp = info.ModTime().String()
		}
		fmt.Fprintf(h, "%s|%s|%s\n", sk.name, filepath.Base(sk.dir), stamp)
	}
	fmt.Fprintf(h, "--\n%s\n", strings.Join(extra, "\n"))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// deliveryDir is the skills folder lich writes into for provider, in the
// profile whose config directory is profileDir.
func (s *Service) deliveryDir(provider, profileDir string) string {
	home, err := s.home()
	if err != nil {
		return ""
	}
	switch provider {
	case providers.Codex:
		if profileDir == "" {
			dir, err := codexprofile.PersonalDir()
			if err != nil {
				return ""
			}
			profileDir = dir
		}
		return filepath.Join(profileDir, skillsDir)
	case providers.Antigravity:
		return filepath.Join(home, ".gemini", skillsDir)
	case providers.Kiro:
		return filepath.Join(providers.KiroHome(home), skillsDir)
	}
	return ""
}

// writeSkills brings dir to the library skills on for provider: each one a
// copy carrying deliveredMarker, the ones turned off removed. A folder of that
// name the person made themselves is never touched.
func (s *Service) writeSkills(dir, provider, profile string) error {
	if dir == "" {
		return nil
	}
	all, err := s.librarySkills()
	if err != nil {
		return err
	}
	var failed []string
	for _, sk := range all {
		target := filepath.Join(dir, filepath.Base(sk.dir))
		_, statErr := os.Lstat(target)
		_, markErr := os.Stat(filepath.Join(target, deliveredMarker))
		ours := markErr == nil
		if statErr == nil && !ours {
			continue
		}
		if ours {
			if err := os.RemoveAll(target); err != nil {
				failed = append(failed, err.Error())
				continue
			}
		}
		if !s.On(api.LibrarySkill, sk.name, provider, profile) {
			continue
		}
		if err := copyDir(sk.dir, target); err != nil {
			failed = append(failed, err.Error())
			continue
		}
		if err := os.WriteFile(filepath.Join(target, deliveredMarker), []byte(deliveredMarkTx), 0o644); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%s", strings.Join(failed, "; "))
	}
	return nil
}

// writeServersInto brings doc to servers for the entries lich wrote there
// before (kept under writtenMCPKey), adding, replacing or removing those alone;
// an entry of the same name the person wrote is left as it is.
func (s *Service) writeServersInto(doc mcpDoc, provider string, servers map[string]Server) error {
	var written []string
	_ = json.Unmarshal([]byte(s.get(writtenMCPKey+provider)), &written)
	config, err := readDoc(doc.path)
	if err != nil {
		return err
	}
	entries, _ := config[doc.key].(map[string]any)
	if entries == nil {
		entries = map[string]any{}
	}
	var now []string
	for _, name := range written {
		if _, on := servers[name]; !on {
			delete(entries, name)
		}
	}
	for name, sv := range servers {
		if _, theirs := entries[name]; theirs && !slices.Contains(written, name) {
			continue
		}
		entries[name] = sv
		now = append(now, name)
	}
	slices.Sort(now)
	if slices.Equal(now, sortedCopy(written)) && len(now) == 0 {
		return nil
	}
	config[doc.key] = entries
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(doc.path, append(body, '\n')); err != nil {
		return err
	}
	data, _ := json.Marshal(now)
	return s.set(writtenMCPKey+provider, string(data))
}

func sortedCopy(list []string) []string {
	out := slices.Clone(list)
	slices.Sort(out)
	return out
}
