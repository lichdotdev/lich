package main

import (
	"context"
	"log"
	"maps"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
)

type providerInfo struct {
	launchProvider
	binary string // the executable a session spawns, which an id is not
	source string // "path", "setting" or "" when nothing was found
	docs   string
}

// providersMachine is what the backend answered: the detected providers with
// their enabled flags, the global values of every key their screens read, and
// each known project's own default provider and profiles.
type providersMachine struct {
	loaded          bool
	list            []providerInfo
	globals         map[string]string
	projectDefaults map[string]string
	// projectProfiles is each project's own profile, by the provider's
	// profile key; a project absent follows Global.
	projectProfiles scopedValues
}

func (m providersMachine) enabled() []providerInfo {
	return slices.DeleteFunc(slices.Clone(m.list), func(p providerInfo) bool { return !p.enabled })
}

func (m providersMachine) find(id string) (providerInfo, bool) {
	i := slices.IndexFunc(m.list, func(p providerInfo) bool { return p.id == id })
	if i < 0 {
		return providerInfo{}, false
	}
	return m.list[i], true
}

func (m providersMachine) launchList() []launchProvider {
	list := make([]launchProvider, len(m.list))
	for i, p := range m.list {
		list[i] = p.launchProvider
	}
	return list
}

// launchEnabled are the enabled providers as the search and the launch menus
// read them.
func (m providersMachine) launchEnabled() []launchProvider {
	return enabledProviders(m.launchList())
}

func (m providersMachine) nameOf(id string) string {
	if p, ok := m.find(id); ok {
		return p.name
	}
	return id
}

func (m providersMachine) skipLevel(kind string) string {
	return skipLevelOf(m.globals[providerSkipKey(kind, false)] == "true", m.globals[providerSkipKey(kind, true)] == "true")
}

func providerFrom(d backend.DetectedProvider, enabled bool) providerInfo {
	return providerInfo{
		launchProvider: launchProvider{id: d.ID, name: d.Name, installed: d.Installed, enabled: enabled},
		binary:         d.Binary, source: d.Source, docs: d.Docs,
	}
}

// readProvidersMachine is the one read of the providers: detection, each
// provider's enabled flag and the global values of the keys its screen reads,
// and the default and profiles of each project in projectIDs.
func readProvidersMachine(ctx context.Context, c *backend.Client, projectIDs []string) (providersMachine, error) {
	detected, err := c.DetectProviders(ctx)
	if err != nil {
		return providersMachine{}, err
	}
	m := providersMachine{loaded: true, globals: map[string]string{}, projectDefaults: map[string]string{}, projectProfiles: scopedValues{}}
	read := func(key string) error {
		v, err := c.GetSetting(ctx, key, "")
		m.globals[key] = v
		return err
	}
	if err := read(providerDefaultKey); err != nil {
		return m, err
	}
	for _, d := range detected {
		for _, key := range providerGlobalKeys(d.ID) {
			if err := read(key); err != nil {
				return m, err
			}
		}
		m.list = append(m.list, providerFrom(d, readEnabled(d.ID, m.globals[providerEnabledKey(d.ID)])))
	}
	for _, id := range projectIDs {
		v, err := c.GetSetting(ctx, providerDefaultKey, id)
		if err != nil {
			return m, err
		}
		if v != "" {
			m.projectDefaults[id] = v
		}
	}
	return m, m.readProfiles(ctx, c, projectIDs)
}

// readProfiles reads each project's profile of every detected provider with
// profiles.
func (m *providersMachine) readProfiles(ctx context.Context, c *backend.Client, projectIDs []string) error {
	var profileKeys []string
	for _, p := range m.list {
		if prov, ok := profileProviderOf(p.id); ok {
			profileKeys = append(profileKeys, prov.key)
		}
	}
	for _, id := range projectIDs {
		for _, key := range profileKeys {
			v, err := c.GetSetting(ctx, key, id)
			if err != nil {
				return err
			}
			m.projectProfiles.put(key, id, v)
		}
	}
	return nil
}

// projectProfile is the profile a project's new sessions of one provider are
// born in: shown, the project's own when it sets one (own), else global.
type projectProfile struct {
	shown, global string
	own           bool
}

func (m providersMachine) projectProfile(kind, projectID string) projectProfile {
	prov, ok := profileProviderOf(kind)
	if !ok {
		return projectProfile{shown: personalProfile, global: personalProfile}
	}
	project, own := m.projectProfiles.set(prov.key, projectID)
	return projectProfile{
		shown:  effectiveScopeProfile(m.globals[prov.key], project, own),
		global: effectiveProfile(m.globals[prov.key]),
		own:    own,
	}
}

// providerBook is the window's one store of the providers: what detection
// found, which are enabled, the default and the binary of each, and every
// open project's default. The launch menus, the implicit session kind,
// Settings › Providers and Sandbox's rows all read it. Whoever writes
// something it reads says so with changed, once the backend has the write.
type providerBook struct {
	client     *backend.Client
	invalidate func()

	mu      sync.Mutex
	machine providersMachine
	// ids are the open projects, and known the ones whose default the book
	// holds, read or taken from the state.
	ids, known []string
	// reading is a read in flight, and again a change that came in during
	// it, which that read may have missed.
	reading, again bool
}

func newProviderBook(c *backend.Client, invalidate func()) *providerBook {
	return &providerBook{client: c, invalidate: invalidate}
}

// snapshot is the book as of now, for a screen to draw from.
func (b *providerBook) snapshot() providersMachine {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := b.machine
	m.list = slices.Clone(m.list)
	m.globals = maps.Clone(m.globals)
	m.projectDefaults = maps.Clone(m.projectDefaults)
	m.projectProfiles = m.projectProfiles.clone()
	return m
}

// launchProviders are the providers a launch menu lists, in detection order.
func (b *providerBook) launchProviders() []launchProvider {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.machine.launchEnabled()
}

// projectProfile is the profile projectID's new sessions of kind are born in.
func (b *providerBook) projectProfile(kind, projectID string) projectProfile {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.machine.projectProfile(kind, projectID)
}

// implicitKind is what a session nobody picked a kind for spawns in project.
func (b *providerBook) implicitKind(projectID string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return implicitSessionKind(b.machine.launchList(), b.machine.globals[providerDefaultKey], b.machine.projectDefaults[projectID])
}

func (b *providerBook) find(id string) (providerInfo, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.machine.find(id)
}

// reload reads the providers and replaces the book with what it found.
func (b *providerBook) reload(ctx context.Context) error {
	b.mu.Lock()
	ids := slices.Clone(b.ids)
	b.mu.Unlock()
	m, err := readProvidersMachine(ctx, b.client, ids)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.machine, b.known = m, ids
	b.mu.Unlock()
	b.invalidate()
	return nil
}

// changed is the invalidation: what the book reads was written, so it reads
// again off the caller. Changes that come in while a read runs fold into one
// more read.
func (b *providerBook) changed(ctx context.Context) {
	b.mu.Lock()
	if b.reading {
		b.again = true
		b.mu.Unlock()
		return
	}
	b.reading = true
	b.mu.Unlock()
	go b.readUntilQuiet(ctx)
}

func (b *providerBook) readUntilQuiet(ctx context.Context) {
	for {
		if err := b.reload(ctx); err != nil {
			log.Printf("providers: %v", err)
		}
		b.mu.Lock()
		if !b.again {
			b.reading = false
			b.mu.Unlock()
			return
		}
		b.again = false
		b.mu.Unlock()
	}
}

// noteProjects is the open projects, which the book takes a default for. The
// default of one it has not read yet is what the state it came in with says,
// until the next read. Its profiles are not in the state: it returns the
// projects it had not seen, for readProfiles.
func (b *providerBook) noteProjects(projects []backend.Project) (fresh []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ids = b.ids[:0]
	for _, p := range projects {
		b.ids = append(b.ids, p.ID)
		if slices.Contains(b.known, p.ID) {
			continue
		}
		fresh = append(fresh, p.ID)
		b.known = append(b.known, p.ID)
		if p.DefaultProvider != "" {
			if b.machine.projectDefaults == nil {
				b.machine.projectDefaults = map[string]string{}
			}
			b.machine.projectDefaults[p.ID] = p.DefaultProvider
		}
	}
	return fresh
}

// setGlobal shows a global write before the backend has it.
func (b *providerBook) setGlobal(key, value string) {
	b.mu.Lock()
	if b.machine.globals == nil {
		b.machine.globals = map[string]string{}
	}
	b.machine.globals[key] = value
	b.mu.Unlock()
}

// setEnabled shows a provider switched on or off before the backend has it.
func (b *providerBook) setEnabled(id string, on bool) {
	b.mu.Lock()
	for i := range b.machine.list {
		if b.machine.list[i].id == id {
			b.machine.list[i].enabled = on
		}
	}
	b.mu.Unlock()
}

// setProjectDefault shows a project's default before the backend has it; ""
// is the project following Global again.
func (b *providerBook) setProjectDefault(projectID, id string) {
	b.mu.Lock()
	if b.machine.projectDefaults == nil {
		b.machine.projectDefaults = map[string]string{}
	}
	if id == "" {
		delete(b.machine.projectDefaults, projectID)
	} else {
		b.machine.projectDefaults[projectID] = id
	}
	b.mu.Unlock()
}

// readProfiles reads the profiles of projects the book learned of from the
// state, without detecting the providers again. Before the first read there is
// no provider to read them for, and that read takes them.
func (b *providerBook) readProfiles(ctx context.Context, projectIDs []string) {
	b.mu.Lock()
	m := providersMachine{list: slices.Clone(b.machine.list), projectProfiles: scopedValues{}}
	b.mu.Unlock()
	if err := m.readProfiles(ctx, b.client, projectIDs); err != nil {
		log.Printf("providers: project profiles: %v", err)
		return
	}
	b.mu.Lock()
	if b.machine.projectProfiles == nil {
		b.machine.projectProfiles = scopedValues{}
	}
	for key, byProject := range m.projectProfiles {
		for id, v := range byProject {
			b.machine.projectProfiles.put(key, id, v)
		}
	}
	b.mu.Unlock()
	b.invalidate()
}

// setProjectProfile shows a project's profile for the provider of key before
// the backend has it; "" is the project following Global again.
func (b *providerBook) setProjectProfile(projectID, key, value string) {
	b.mu.Lock()
	if b.machine.projectProfiles == nil {
		b.machine.projectProfiles = scopedValues{}
	}
	b.machine.projectProfiles.put(key, projectID, value)
	b.mu.Unlock()
}
