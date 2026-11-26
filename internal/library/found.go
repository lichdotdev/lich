package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// What each provider has installed in its own config that is not in the
// library, and moving it in: a copy into the library, turned off where it was,
// never deleted. A skill is turned off by renaming its SKILL.md (or, for a
// link, the link) with a ".lich-moved" suffix, which no provider loads; an MCP
// server by moving its entry under a "lichMoved" key beside the provider's own,
// which no provider reads.

// lichNames are lich's own items, installed by the plugin, never offered.
func lichName(name string) bool {
	return strings.HasPrefix(name, "lich-") || name == relay.MCPServerName
}

// deliveredMarker marks a skill folder lich wrote into a provider's config
// (deliver.go), which is the library's own copy and not found there.
const deliveredMarker = ".lich-library"

// foundSkill is one skill in a provider's folder.
type foundSkill struct {
	skill
	provider string
	entry    string // the folder or link in the provider's folder
}

func (s *Service) foundSkills() []foundSkill {
	var out []foundSkill
	for _, p := range providers.Registry {
		for _, dir := range s.skillDirs(p.ID) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				entry := filepath.Join(dir, e.Name())
				if _, err := os.Stat(filepath.Join(entry, deliveredMarker)); err == nil {
					continue
				}
				data, err := os.ReadFile(filepath.Join(entry, skillFile))
				if err != nil {
					continue
				}
				name, description := frontmatter(string(data))
				if name == "" {
					name = e.Name()
				}
				if lichName(name) {
					continue
				}
				out = append(out, foundSkill{name: name, description: description, dir: entry, provider: p.ID, entry: entry})
			}
		}
	}
	return out
}

// foundServer is one MCP server in a provider's document.
type foundServer struct {
	name     string
	server   Server
	provider string
	doc      mcpDoc
}

func (s *Service) foundServers() []foundServer {
	var out []foundServer
	for _, p := range providers.Registry {
		for _, doc := range s.mcpDocs(p.ID) {
			config, err := readDoc(doc.path)
			if err != nil {
				continue
			}
			entries, _ := config[doc.key].(map[string]any)
			for name, raw := range entries {
				if lichName(name) {
					continue
				}
				if sv, ok := serverOf(raw, doc.shape); ok {
					out = append(out, foundServer{name: name, server: sv, provider: p.ID, doc: doc})
				}
			}
		}
	}
	return out
}

// readDoc reads a JSON document as an object; a missing one is empty.
func readDoc(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	config := map[string]any{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%s is not a JSON object lich can read: %w", path, err)
	}
	return config, nil
}

// serverOf normalizes one entry of a provider's document.
func serverOf(raw any, shape docShape) (Server, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return Server{}, false
	}
	str := func(k string) string { v, _ := m[k].(string); return v }
	strs := func(v any) []string {
		var out []string
		list, _ := v.([]any)
		for _, x := range list {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	env := func(k string) map[string]string {
		obj, _ := m[k].(map[string]any)
		if len(obj) == 0 {
			return nil
		}
		out := map[string]string{}
		for key, v := range obj {
			if s, ok := v.(string); ok {
				out[key] = s
			}
		}
		return out
	}
	if shape == shapeOpencode {
		cmd := strs(m["command"])
		sv := Server{URL: str("url"), Env: env("environment")}
		if len(cmd) > 0 {
			sv.Command, sv.Args = cmd[0], cmd[1:]
		}
		if str("type") == "remote" {
			sv.Type = "http"
		}
		return sv, sv.Command != "" || sv.URL != ""
	}
	sv := Server{Type: str("type"), Command: str("command"), Args: strs(m["args"]), Env: env("env"), URL: str("url")}
	if sv.URL == "" {
		sv.URL = str("serverUrl")
	}
	return sv, sv.Command != "" || sv.URL != ""
}

// found is everything the providers hold that the library does not, one entry
// per kind and name.
func (s *Service) found() []api.LibraryFound {
	have := map[string]bool{}
	if skills, err := s.librarySkills(); err == nil {
		for _, sk := range skills {
			have[api.LibrarySkill+"/"+sk.name] = true
		}
	}
	if servers, err := s.libraryServers(); err == nil {
		for name := range servers {
			have[api.LibraryMCP+"/"+name] = true
		}
	}
	byKey := map[string]*api.LibraryFound{}
	add := func(kind, name, description, transport, provider, path string) {
		key := kind + "/" + name
		if have[key] {
			return
		}
		f := byKey[key]
		if f == nil {
			f = &api.LibraryFound{Kind: kind, Name: name, Description: description, Transport: transport}
			byKey[key] = f
		}
		if !slices.Contains(f.Providers, provider) {
			f.Providers = append(f.Providers, provider)
		}
		f.Paths = append(f.Paths, path)
	}
	for _, f := range s.foundSkills() {
		add(api.LibrarySkill, f.name, f.description, "", f.provider, f.entry)
	}
	for _, f := range s.foundServers() {
		add(api.LibraryMCP, f.name, f.server.commandLine(), f.server.transport(), f.provider, f.doc.path)
	}
	seen := map[string]map[string]bool{}
	for _, kind := range []string{api.LibrarySkill, api.LibraryMCP} {
		seen[kind] = map[string]bool{}
		for _, n := range s.seen(kind) {
			seen[kind][n] = true
		}
	}
	out := make([]api.LibraryFound, 0, len(byKey))
	for _, f := range byKey {
		f.New = !seen[f.Kind][f.Name]
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind > out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// MoveIn copies a found item into the library, turns it off in every provider
// it was found in, and turns it on in lich for those providers, over RPC.
func (s *Service) MoveIn(_ context.Context, opts api.LibraryItemOptions) (api.None, error) {
	s.mu.Lock()
	var origins []string
	var err error
	switch opts.Kind {
	case api.LibrarySkill:
		origins, err = s.moveSkill(opts.Name)
	case api.LibraryMCP:
		origins, err = s.moveServer(opts.Name)
	default:
		err = fmt.Errorf("no library item of kind %q", opts.Kind)
	}
	s.mu.Unlock()
	if err != nil {
		return api.None{}, err
	}
	for _, p := range origins {
		if err := s.set(switchKey(opts.Kind, opts.Name, p, ""), switchOn); err != nil {
			return api.None{}, err
		}
	}
	s.changed()
	return api.None{}, nil
}

// moveSkill copies the first copy found into the library, then turns every
// copy off where it was.
func (s *Service) moveSkill(name string) ([]string, error) {
	var copies []foundSkill
	for _, f := range s.foundSkills() {
		if f.name == name {
			copies = append(copies, f)
		}
	}
	if len(copies) == 0 {
		return nil, fmt.Errorf("no provider has a skill named %q", name)
	}
	lib, err := s.libraryDir()
	if err != nil {
		return nil, err
	}
	dest := filepath.Join(lib, skillsDir, name)
	if _, err := os.Stat(dest); err == nil {
		return nil, fmt.Errorf("the library already has a skill named %q", name)
	}
	if err := copyDir(copies[0].entry, dest); err != nil {
		_ = os.RemoveAll(dest)
		return nil, fmt.Errorf("copy %s into the library: %w", copies[0].entry, err)
	}
	var origins []string
	done := map[string]bool{}
	for _, f := range copies {
		// One folder read by two providers (~/.agents/skills, by Codex and
		// Crush) is set aside once.
		if !done[f.entry] {
			done[f.entry] = true
			if err := turnOffSkill(f.entry); err != nil {
				return origins, fmt.Errorf("turn %s off where it was: %w", f.entry, err)
			}
		}
		if !slices.Contains(origins, f.provider) {
			origins = append(origins, f.provider)
		}
	}
	return origins, nil
}

// turnOffSkill makes a provider stop loading the skill at entry without
// deleting it: a link is renamed, a folder keeps everything but the name of its
// SKILL.md.
func turnOffSkill(entry string) error {
	info, err := os.Lstat(entry)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Rename(entry, entry+movedSuffix)
	}
	return os.Rename(filepath.Join(entry, skillFile), filepath.Join(entry, skillFile+movedSuffix))
}

// moveServer copies the first entry found into the library, then moves every
// entry of that name out of the key its provider reads.
func (s *Service) moveServer(name string) ([]string, error) {
	var copies []foundServer
	for _, f := range s.foundServers() {
		if f.name == name {
			copies = append(copies, f)
		}
	}
	if len(copies) == 0 {
		return nil, fmt.Errorf("no provider has an MCP server named %q", name)
	}
	servers, err := s.libraryServers()
	if err != nil {
		return nil, err
	}
	if _, ok := servers[name]; ok {
		return nil, fmt.Errorf("the library already has an MCP server named %q", name)
	}
	servers[name] = copies[0].server
	if err := s.writeServers(servers); err != nil {
		return nil, err
	}
	var origins []string
	for _, f := range copies {
		// turnOffServer leaves a document already set aside as it is.
		if err := turnOffServer(f.doc, name); err != nil {
			return origins, fmt.Errorf("turn %s off in %s: %w", name, f.doc.path, err)
		}
		if !slices.Contains(origins, f.provider) {
			origins = append(origins, f.provider)
		}
	}
	return origins, nil
}

// movedKey is where a moved entry goes, beside the key the provider reads.
func movedKey(key string) string { return "lichMoved_" + key }

// turnOffServer moves name's entry from doc's key to movedKey.
func turnOffServer(doc mcpDoc, name string) error {
	config, err := readDoc(doc.path)
	if err != nil {
		return err
	}
	entries, _ := config[doc.key].(map[string]any)
	entry, ok := entries[name]
	if !ok {
		return nil
	}
	moved, _ := config[movedKey(doc.key)].(map[string]any)
	if moved == nil {
		moved = map[string]any{}
	}
	moved[name] = entry
	delete(entries, name)
	config[doc.key], config[movedKey(doc.key)] = entries, moved
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(doc.path, append(body, '\n'))
}

// copyDir copies from into to, following links so a skill that links a file
// from elsewhere arrives whole; .git is left out.
func copyDir(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ".git" || e.Name() == deliveredMarker {
			continue
		}
		src, dst := filepath.Join(from, e.Name()), filepath.Join(to, e.Name())
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
