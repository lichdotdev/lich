package main

import (
	"slices"
	"sort"
	"strings"

	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (settings.settings.*,
// diffLayoutSetting, and the scope copy of the approved mockup).
const (
	settingsSearchHint = "Search settings"
	settingsNoMatch    = "Nothing matches %s. The search reads the name of every setting, not their values."
	settingsResultOne  = "%d setting"
	settingsResultMany = "%d settings"

	hideWhitespaceTitle = "Hide whitespace in diffs"
	hideWhitespaceDesc  = "Leave out changes that only touch spacing in the Review tab."
	hideWhitespaceAlso  = "space indentation review ignore"
	diffLayoutTitle     = "Diff layout"
	diffLayoutDesc      = "How changes are drawn in the Review tab and in pull requests. Side-by-side shows unified while the panel is too narrow for two columns."
	diffLayoutAlso      = "side by side split unified review"
)

// The section ids, Settings.tsx's plus Editor, which only the native window
// has.
const (
	sectionAppearance     = "appearance"
	sectionNotifications  = "notifications"
	sectionHotkeys        = "hotkeys"
	sectionProviders      = "providers"
	sectionSandbox        = "sandbox"
	sectionVersionControl = "version-control"
	sectionEditor         = "editor"
	sectionUpdates        = "updates"
	sectionHelp           = "help"
)

// settingScope is where a setting can be set.
type settingScope uint8

const (
	scopeGlobalOnly settingScope = iota
	// scopeOverridable has a global value a project may replace.
	scopeOverridable
	// scopeProjectOnly exists per project and has no global value.
	scopeProjectOnly
)

// settingEntry is one row of a section: settings-index.ts's SettingEntry plus
// the backend key and scope the row's states are read from.
type settingEntry struct {
	title, also string
	key         string
	scope       settingScope
}

// settingsSection is one nav row. rows are the ported rows, drawn and
// searched; unported are the scoped keys of rows still to be ported, counted
// for the override badges until they are.
type settingsSection struct {
	id, label string
	icon      *icons.Icon
	footer    bool
	rows      []settingEntry
	unported  []settingEntry
}

// settingsSections is the nav in order. ponytail: the unported keys are the
// fixed ones; Sandbox's per-provider rung (provider.<id>.sandbox) joins the
// count when Sandbox is ported against the provider list.
var settingsSections = []settingsSection{
	{id: sectionAppearance, label: "Appearance", icon: icons.Lucide("palette")},
	{id: sectionNotifications, label: "Notifications", icon: icons.Lucide("bell")},
	{id: sectionHotkeys, label: "Hotkeys", icon: icons.Lucide("keyboard")},
	{id: sectionProviders, label: "Providers", icon: icons.Lucide("blocks"),
		unported: []settingEntry{{key: "provider.default", scope: scopeOverridable}}},
	{id: sectionSandbox, label: "Sandbox", icon: icons.Lucide("shield-check"), unported: []settingEntry{
		{key: "sandbox.ssh-agent", scope: scopeOverridable},
		{key: "sandbox.gh-token", scope: scopeOverridable},
	}},
	{id: sectionVersionControl, label: "Version Control", icon: icons.Lucide("git-branch"),
		unported: []settingEntry{{key: "vcs.account", scope: scopeProjectOnly}}},
	{id: sectionEditor, label: "Editor", icon: icons.Lucide("code"), rows: []settingEntry{
		{title: hideWhitespaceTitle, also: hideWhitespaceAlso, key: hideWhitespaceKey},
		{title: diffLayoutTitle, also: diffLayoutAlso, key: diffLayoutKey},
	}},
	{id: sectionUpdates, label: "Updates", icon: icons.Lucide("download"), footer: true},
	{id: sectionHelp, label: "Help", icon: icons.Lucide("circle-question-mark"), footer: true},
}

// defaultSection is the pane a first open lands on: the web's is Providers,
// which the native window has not ported yet.
const defaultSection = sectionEditor

func sectionByID(id string) settingsSection {
	for _, s := range settingsSections {
		if s.id == id {
			return s
		}
	}
	return settingsSections[0]
}

// parseSettingsSection is settings-prefs.ts readSettingsSection: an id this
// build has no section for opens the default.
func parseSettingsSection(raw string) string {
	if slices.ContainsFunc(settingsSections, func(s settingsSection) bool { return s.id == raw }) {
		return raw
	}
	return defaultSection
}

// entries are every entry of the section, ported or not.
func (s settingsSection) entries() []settingEntry { return append(slices.Clone(s.rows), s.unported...) }

// projectScoped reports whether a project can change anything in the
// section, which is what keeps it lit in a project's scope.
func (s settingsSection) projectScoped() bool {
	return slices.ContainsFunc(s.entries(), func(e settingEntry) bool { return e.scope != scopeGlobalOnly })
}

// settingHit is one search result: a row, or a section itself, whose title
// is its label.
type settingHit struct {
	section string
	title   string
}

// path is the line a result shows above its title, "" when it would repeat it.
func (h settingHit) path() string {
	label := sectionByID(h.section).label
	if label == h.title {
		return ""
	}
	return label
}

// searchSettings is settings-index.ts searchSettings over the ported rows and
// the sections: a title that starts with the query beats one that contains
// it, and both beat a match on the also words; ties keep the nav's order.
func searchSettings(query string) []settingHit {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}
	type scored struct {
		hit  settingHit
		rank int
	}
	var found []scored
	consider := func(section, title, also string) {
		t := strings.ToLower(title)
		switch {
		case strings.HasPrefix(t, needle):
			found = append(found, scored{settingHit{section, title}, 0})
		case strings.Contains(t, needle):
			found = append(found, scored{settingHit{section, title}, 1})
		case strings.Contains(strings.ToLower(also), needle):
			found = append(found, scored{settingHit{section, title}, 2})
		}
	}
	for _, s := range settingsSections {
		for _, e := range s.rows {
			consider(s.id, e.title, e.also)
		}
	}
	for _, s := range settingsSections {
		consider(s.id, s.label, "")
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].rank < found[j].rank })
	hits := make([]settingHit, len(found))
	for i, f := range found {
		hits[i] = f.hit
	}
	return hits
}
