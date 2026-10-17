package main

import (
	"slices"
	"sort"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The section ids, Settings.tsx's plus General and Editor, which only the
// native window has: General holds what the web keeps under Notifications.
const (
	sectionGeneral        = "general"
	sectionAppearance     = "appearance"
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
// the backend key and scope the row's states are read from. groupKey names the
// heading the row sits under, "" in a section without groups. A row names
// its catalog keys (titleKey, alsoKey, groupKey), as settings-index.ts does,
// and is read through titleText, alsoText and groupText.
type settingEntry struct {
	key   string
	scope settingScope

	titleKey, alsoKey, groupKey string

	// perProvider is a block of a provider's own screen: it is searched once
	// per enabled provider, only the ones in onlyFor when that is set, and a
	// result opens that provider's screen (settings-index.ts perProvider).
	perProvider bool
	onlyFor     []string
}

func (e settingEntry) titleText() string { return keyText(e.titleKey) }
func (e settingEntry) alsoText() string  { return keyText(e.alsoKey) }
func (e settingEntry) groupText() string { return keyText(e.groupKey) }

// keyText is key's message in the interface language, "" for a row without
// that part.
func keyText(key string) string {
	if key == "" {
		return ""
	}
	return i18n.T(key)
}

// settingsSection is one nav row. rows are the ported rows, drawn and
// searched; unported are the scoped keys of rows still to be ported, counted
// for the override badges until they are.
type settingsSection struct {
	id       string
	labelKey string
	icon     *icons.Icon
	footer   bool
	rows     []settingEntry
	unported []settingEntry
}

func (s settingsSection) label() string { return i18n.T(s.labelKey) }

// settingsSections is the nav in order.
var settingsSections = []settingsSection{
	{id: sectionGeneral, labelKey: "settings.settingsIndex.sectionGeneral", icon: icons.Lucide("settings-2"), rows: []settingEntry{
		{titleKey: "settings.language.uiTitle", alsoKey: "settings.language.uiSearchWords", key: uiLanguageKey, groupKey: "settings.language.group"},
		{titleKey: "settings.language.promptTitle", alsoKey: "settings.language.promptSearchWords", key: promptLanguageKey, groupKey: "settings.language.group"},
		{titleKey: "settings.windowSetting.title", alsoKey: "settings.windowSetting.searchWords", key: windowKindKey, groupKey: "settings.windowSetting.title"},
		{titleKey: "settings.closeSetting.title", alsoKey: "settings.closeSetting.searchWords", key: closeActionKey, groupKey: "settings.windowSetting.title"},
		{titleKey: "settings.notificationsSettings.needsInputTitle", alsoKey: "settings.settingsGeneral.notifySearchWords", key: desktopNotifyKey, groupKey: "settings.settings.section.notifications"},
		{titleKey: "settings.notificationsSettings.finishedTitle", alsoKey: "settings.settingsGeneral.notifySearchWords", key: finishedNotifyKey, groupKey: "settings.settings.section.notifications"},
	}},
	{id: sectionAppearance, labelKey: "settings.settings.section.appearance", icon: icons.Lucide("palette"), rows: []settingEntry{
		{titleKey: "settings.themePicker.title", alsoKey: "settings.themePicker.searchWords", key: themeSettingKey},
		{titleKey: "settings.appearanceSettings.zoomTitle", key: zoomKey},
		{titleKey: "settings.settingsAppearance.uiFontTitle", alsoKey: "settings.settingsAppearance.uiFontSearchWords", key: uiFontKey},
		{titleKey: "settings.fontSetting.title", alsoKey: "settings.fontSetting.searchWords", key: terminalFontKey},
		{titleKey: "settings.appearanceSettings.textSizeTitle", key: terminalFontSizeKey},
		{titleKey: "settings.footerSettings.title", alsoKey: "settings.footerSettings.searchWords", key: footerLayoutKey},
		{titleKey: "settings.footerSettings.ceilingTitle", alsoKey: "settings.footerSettings.ceilingSearchWords", key: costBudgetKey},
	}},
	{id: sectionHotkeys, labelKey: "settings.settings.section.hotkeys", icon: icons.Lucide("keyboard"), rows: hotkeyEntries()},
	{id: sectionProviders, labelKey: "settings.settings.section.providers", icon: icons.Lucide("blocks"),
		rows: providerEntries(), unported: providerBinEntries()},
	{id: sectionSandbox, labelKey: "settings.settings.section.sandbox", icon: icons.Lucide("shield-check"), rows: []settingEntry{
		{titleKey: "settings.sandboxSettings.confinedTitle", alsoKey: "settings.sandboxSettings.confinedSearchWords", key: sandboxLadderKey},
		{titleKey: "settings.sandboxSettings.sshTitle", alsoKey: "settings.sandboxSettings.sshSearchWords", key: sshAgentKey, scope: scopeOverridable, groupKey: "settings.sandboxSettings.carryTitle"},
		{titleKey: "settings.sandboxSettings.ghTitle", alsoKey: "settings.sandboxSettings.ghSearchWords", key: ghTokenKey, scope: scopeOverridable, groupKey: "settings.sandboxSettings.carryTitle"},
	}, unported: sandboxRungEntries()},
	{id: sectionVersionControl, labelKey: "settings.settings.section.versionControl", icon: icons.Lucide("git-branch"), rows: []settingEntry{
		{titleKey: "settings.vcsToolsSetting.title", alsoKey: "settings.vcsToolsSetting.searchWords", key: vcsToolsKey},
		{titleKey: "settings.versionControlSettings.account", alsoKey: "settings.versionControlSettings.accountSearchWords", key: vcsAccountKey, scope: scopeProjectOnly},
	}},
	{id: sectionEditor, labelKey: "settings.settingsIndex.sectionEditor", icon: icons.Lucide("code"), rows: []settingEntry{
		{titleKey: "settings.settingsAppearance.editorFontTitle", alsoKey: "settings.settingsAppearance.editorFontSearchWords", key: editorFontKey},
		{titleKey: "settings.settingsAppearance.editorSizeTitle", alsoKey: "settings.settingsAppearance.editorSizeSearchWords", key: editorFontSizeKey},
		{titleKey: "settings.settingsIndex.hideWhitespaceTitle", alsoKey: "settings.settingsIndex.hideWhitespaceSearchWords", key: hideWhitespaceKey},
		{titleKey: "settings.diffLayoutSetting.title", alsoKey: "settings.diffLayoutSetting.searchWords", key: diffLayoutKey},
	}},
	{id: sectionUpdates, labelKey: "settings.settings.section.updates", icon: icons.Lucide("download"), footer: true, rows: []settingEntry{
		{titleKey: "settings.updatesSettings.applicationTitle", alsoKey: "settings.updatesSettings.applicationSearchWords", key: updatesAppKey},
		{titleKey: "settings.updatesSettings.whatsNewTitle", alsoKey: "settings.updatesSettings.whatsNewSearchWords", key: updatesNotesKey},
		{titleKey: "settings.pluginSetting.title", alsoKey: "settings.pluginSetting.searchWords", key: updatesPluginKey},
	}},
	{id: sectionHelp, labelKey: "settings.settings.section.help", icon: icons.Lucide("circle-question-mark"), footer: true, rows: []settingEntry{
		{titleKey: "settings.helpSettings.bugTitle", alsoKey: "settings.helpSettings.bugSearchWords", key: helpBugKey},
		{titleKey: "settings.helpSettings.logTitle", alsoKey: "settings.settingsHelp.logSearchWords", key: helpLogKey},
		{titleKey: "settings.helpSettings.aboutTitle", alsoKey: "settings.helpSettings.aboutSearchWords", key: helpAboutKey},
	}},
}

// defaultSection is the pane a first open lands on.
const defaultSection = sectionProviders

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
	group   string
	title   string
	// providerID is the provider whose screen holds the row, "" for a row
	// that is not on one.
	providerID string
}

// path is settings-index.ts hitPath, the line a result shows above its
// title: the section, then the group; "" when it would repeat the title.
func (h settingHit) path() string {
	label := sectionByID(h.section).label()
	switch {
	case h.group != "":
		return label + " › " + h.group
	case label == h.title:
		return ""
	}
	return label
}

// searchSettings is settings-index.ts searchSettings over the ported rows and
// the sections: a title that starts with the query beats one that contains
// it, and both beat a match on the also words, a row's group among them;
// ties keep the nav's order. providers are the enabled ones: a block of a
// provider's screen is a result once per provider, named after it, and none
// at all without any.
func searchSettings(query string, providers ...launchProvider) []settingHit {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}
	type scored struct {
		hit  settingHit
		rank int
	}
	var found []scored
	consider := func(hit settingHit, also string) {
		t := strings.ToLower(hit.title)
		switch {
		case strings.HasPrefix(t, needle):
			found = append(found, scored{hit, 0})
		case strings.Contains(t, needle):
			found = append(found, scored{hit, 1})
		case strings.Contains(strings.ToLower(also), needle):
			found = append(found, scored{hit, 2})
		}
	}
	for _, s := range settingsSections {
		for _, e := range s.rows {
			if !e.perProvider {
				consider(settingHit{section: s.id, group: e.groupText(), title: e.titleText()}, e.alsoText()+" "+e.groupText())
				continue
			}
			for _, p := range providers {
				if len(e.onlyFor) == 0 || slices.Contains(e.onlyFor, p.id) {
					consider(settingHit{section: s.id, group: p.name, title: e.titleText(), providerID: p.id}, e.alsoText()+" "+p.name)
				}
			}
		}
	}
	for _, s := range settingsSections {
		consider(settingHit{section: s.id, title: s.label()}, "")
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].rank < found[j].rank })
	hits := make([]settingHit, len(found))
	for i, f := range found {
		hits[i] = f.hit
	}
	return hits
}
