package main

import (
	"context"
	"hash/fnv"
	"image/color"

	"github.com/omartelo/lich/native/lichclient"
)

// settingsProject is an open project as the scope selector lists it.
type settingsProject struct {
	id, name string
}

// projectSwatch is a project's color in the scope selector. Projects have no
// color of their own, so it is a stable pick from the card palette by id.
func projectSwatch(id string) color.NRGBA {
	h := fnv.New32a()
	h.Write([]byte(id))
	return cardColors[cardColorNames[h.Sum32()%uint32(len(cardColorNames))]]
}

// scopedValues is what the open projects set: key, then project id, to a
// value that is never "" (an empty value is an unset one, as the backend's
// projectSetting reads it).
type scopedValues map[string]map[string]string

func (v scopedValues) set(key, projectID string) (string, bool) {
	value, ok := v[key][projectID]
	return value, ok
}

// rowKind is the scope state a setting row is drawn in.
type rowKind uint8

const (
	rowPlain rowKind = iota
	// rowOverridden is a global row some open project sets.
	rowOverridden
	// rowFollows is a project row the project leaves to Global.
	rowFollows
	// rowChanged is a project row the project sets.
	rowChanged
	// rowGlobalOnly is a global-only row seen from a project.
	rowGlobalOnly
	// rowProjectOnly is a project-only row seen from its project.
	rowProjectOnly
	// rowHidden is a project-only row seen from Global.
	rowHidden
)

// rowKindOf is the state of e seen from scope, a project id or "" for Global.
func rowKindOf(e settingEntry, scope string, projects []settingsProject, values scopedValues) rowKind {
	if scope == "" {
		switch {
		case e.scope == scopeProjectOnly:
			return rowHidden
		case e.scope == scopeOverridable && len(overriders(e, projects, values)) > 0:
			return rowOverridden
		}
		return rowPlain
	}
	switch e.scope {
	case scopeGlobalOnly:
		return rowGlobalOnly
	case scopeProjectOnly:
		return rowProjectOnly
	}
	if _, ok := values.set(e.key, scope); ok {
		return rowChanged
	}
	return rowFollows
}

// overriders are the names of the open projects that set e, in tab order.
func overriders(e settingEntry, projects []settingsProject, values scopedValues) []string {
	if e.scope != scopeOverridable {
		return nil
	}
	var names []string
	for _, p := range projects {
		if _, ok := values.set(e.key, p.id); ok {
			names = append(names, p.name)
		}
	}
	return names
}

// sectionBadge is the count on a nav row: from Global, the section's rows
// some open project overrides; from a project, the rows it changed.
func sectionBadge(s settingsSection, scope string, projects []settingsProject, values scopedValues) int {
	n := 0
	for _, e := range s.entries() {
		k := rowKindOf(e, scope, projects, values)
		if k == rowOverridden || k == rowChanged {
			n++
		}
	}
	return n
}

// projectChanges is the count the scope menu shows beside a project: the
// overridable settings it changed, across every section.
func projectChanges(projectID string, values scopedValues) int {
	n := 0
	for _, s := range settingsSections {
		n += sectionBadge(s, projectID, nil, values)
	}
	return n
}

// resolveScope is the remembered scope if its project is still open, else
// Global.
func resolveScope(scope string, projects []settingsProject) string {
	for _, p := range projects {
		if p.id == scope {
			return scope
		}
	}
	return ""
}

// scopedKeys are the keys a project can set, read for each open project.
func scopedKeys() []string {
	var keys []string
	for _, s := range settingsSections {
		for _, e := range s.entries() {
			if e.scope != scopeGlobalOnly {
				keys = append(keys, e.key)
			}
		}
	}
	return keys
}

// loadScopedValues reads every scoped key of every open project. There is no
// RPC that lists a project's settings, so it is one GetSetting per pair; the
// keys are a handful and the projects are the open tabs.
func loadScopedValues(ctx context.Context, c *lichclient.Client, projects []settingsProject) (scopedValues, error) {
	values := scopedValues{}
	for _, key := range scopedKeys() {
		for _, p := range projects {
			v, err := c.GetSetting(ctx, key, p.id)
			if err != nil {
				return nil, err
			}
			if v == "" {
				continue
			}
			if values[key] == nil {
				values[key] = map[string]string{}
			}
			values[key][p.id] = v
		}
	}
	return values, nil
}
