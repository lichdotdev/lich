package main

import (
	"slices"
	"sort"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui/icons"
)

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

// settingEntry is one row of a section, with the backend key and scope the
// row's states are read from. groupKey names the heading the row sits under,
// or the page that holds it in a split section; "" in a section without
// groups. A row names its catalog keys (titleKey, alsoKey,
// groupKey) and is read through titleText, alsoText and groupText.
type settingEntry struct {
	key   string
	scope settingScope

	titleKey, alsoKey, groupKey string

	// perProvider is a block of a provider's own screen: it is searched once
	// per enabled provider, only the ones in onlyFor when that is set, and a
	// result opens that provider's screen.
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

// settingsSection is one nav row, or one per page when it has pages. rows
// are the ported rows, drawn and searched; unported are the scoped keys of
// rows still to be ported, counted for the override badges until they are.
type settingsSection struct {
	id       string
	labelKey string
	icon     *icons.Icon
	// navGroupKey names the nav heading the section sits under, "" for the
	// footer's sections.
	navGroupKey string
	footer      bool
	// pages split the section into nav entries of their own, one per group
	// of its rows; a section without them is one entry.
	pages    []settingsPage
	rows     []settingEntry
	unported []settingEntry
}

func (s settingsSection) label() string { return i18n.T(s.labelKey) }

// The nav's headings over its sections.
const (
	navGroupGeneral    = "settings.settingsIndex.sectionGeneral"
	navGroupAppearance = "settings.settings.section.appearance"
	navGroupAgents     = "settings.settingsNav.groupAgents"
	navGroupWorkspace  = "settings.settingsNav.groupWorkspace"
)

// The groups General and Appearance split into, each a page.
const (
	groupLanguage      = "settings.language.group"
	groupWindow        = "settings.windowSetting.title"
	groupNotifications = "settings.settings.section.notifications"
	groupTerminal      = "settings.settingsGeneral.terminalGroup"
	groupInterface     = "settings.settingsIndex.interfaceGroup"
	groupThemes        = "settings.themesPage.title"
	groupFooter        = "settings.footerSettings.title"
)

// settingsSections is the nav in order.
var settingsSections = []settingsSection{
	{id: sectionGeneral, labelKey: "settings.settingsIndex.sectionGeneral", icon: icons.Lucide("settings-2"), navGroupKey: navGroupGeneral, pages: []settingsPage{
		{id: "language", groupKey: groupLanguage, icon: icons.Lucide("languages")},
		{id: "window", groupKey: groupWindow, icon: icons.Lucide("app-window")},
		{id: "notifications", groupKey: groupNotifications, icon: icons.Lucide("bell")},
	}, rows: []settingEntry{
		{titleKey: "settings.language.uiTitle", alsoKey: "settings.language.uiSearchWords", key: uiLanguageKey, groupKey: groupLanguage},
		{titleKey: "settings.language.promptTitle", alsoKey: "settings.language.promptSearchWords", key: promptLanguageKey, groupKey: groupLanguage},
		{titleKey: "settings.closeSetting.title", alsoKey: "settings.closeSetting.searchWords", key: closeActionKey, groupKey: groupWindow},
		{titleKey: "settings.windowButtonsSetting.title", alsoKey: "settings.windowButtonsSetting.searchWords", key: windowButtonsKey, groupKey: groupWindow},
		{titleKey: "settings.notificationsSettings.needsInputTitle", alsoKey: "settings.settingsGeneral.notifySearchWords", key: desktopNotifyKey, groupKey: groupNotifications},
		{titleKey: "settings.notificationsSettings.finishedTitle", alsoKey: "settings.settingsGeneral.notifySearchWords", key: finishedNotifyKey, groupKey: groupNotifications},
	}},
	{id: sectionHotkeys, labelKey: "settings.settings.section.hotkeys", icon: icons.Lucide("keyboard"), navGroupKey: navGroupGeneral, rows: hotkeyEntries()},
	{id: sectionAppearance, labelKey: "settings.settings.section.appearance", icon: icons.Lucide("palette"), navGroupKey: navGroupAppearance, pages: []settingsPage{
		{id: "interface", groupKey: groupInterface, icon: icons.Lucide("palette")},
		{id: themesPage, groupKey: groupThemes, icon: icThemesBrowse},
		{id: "terminal", groupKey: groupTerminal, icon: icons.Lucide("square-terminal")},
		{id: "footer", groupKey: groupFooter, icon: icons.Lucide("panel-bottom")},
	}, rows: []settingEntry{
		{titleKey: "settings.appearanceSettings.zoomTitle", key: zoomKey, groupKey: groupInterface},
		{titleKey: "settings.settingsAppearance.uiFontTitle", alsoKey: "settings.settingsAppearance.uiFontSearchWords", key: uiFontKey, groupKey: groupInterface},
		{titleKey: "settings.chatWidthSetting.title", alsoKey: "settings.chatWidthSetting.searchWords", key: chatWidthKey, groupKey: groupInterface},
		{titleKey: "settings.themesPage.catalogRow", alsoKey: "settings.themesPage.catalogSearchWords", key: themesCatalogKey, groupKey: groupThemes},
		{titleKey: "settings.themesPage.installedRow", alsoKey: "settings.themesPage.installedSearchWords", key: themesInstalledKey, groupKey: groupThemes},
		{titleKey: "settings.fontSetting.title", alsoKey: "settings.fontSetting.searchWords", key: terminalFontKey, groupKey: groupTerminal},
		{titleKey: "settings.appearanceSettings.textSizeTitle", key: terminalFontSizeKey, groupKey: groupTerminal},
		{titleKey: "settings.appearanceSettings.copyOnSelectTitle", alsoKey: "settings.appearanceSettings.copyOnSelectSearchWords", key: copyOnSelectKey, groupKey: groupTerminal},
		{titleKey: "settings.footerSettings.title", alsoKey: "settings.footerSettings.searchWords", key: footerLayoutKey, groupKey: groupFooter},
		{titleKey: "settings.footerSettings.ceilingTitle", alsoKey: "settings.footerSettings.ceilingSearchWords", key: costBudgetKey, groupKey: groupFooter},
	}},
	{id: sectionEditor, labelKey: "settings.settingsIndex.sectionEditor", icon: icons.Lucide("code"), navGroupKey: navGroupAppearance, rows: []settingEntry{
		{titleKey: "settings.settingsAppearance.editorFontTitle", alsoKey: "settings.settingsAppearance.editorFontSearchWords", key: editorFontKey},
		{titleKey: "settings.settingsAppearance.editorSizeTitle", alsoKey: "settings.settingsAppearance.editorSizeSearchWords", key: editorFontSizeKey},
		{titleKey: "settings.settingsIndex.hideWhitespaceTitle", alsoKey: "settings.settingsIndex.hideWhitespaceSearchWords", key: hideWhitespaceKey},
		{titleKey: "settings.diffLayoutSetting.title", alsoKey: "settings.diffLayoutSetting.searchWords", key: diffLayoutKey},
	}},
	{id: sectionProviders, labelKey: "settings.settings.section.providers", icon: icons.Lucide("blocks"), navGroupKey: navGroupAgents,
		rows: providerEntries(), unported: providerBinEntries()},
	{id: sectionSandbox, labelKey: "settings.settings.section.sandbox", icon: icons.Lucide("shield-check"), navGroupKey: navGroupAgents, rows: []settingEntry{
		{titleKey: "settings.sandboxSettings.confinedTitle", alsoKey: "settings.sandboxSettings.confinedSearchWords", key: sandboxLadderKey},
		{titleKey: "settings.sandboxSettings.sshTitle", alsoKey: "settings.sandboxSettings.sshSearchWords", key: sshAgentKey, scope: scopeOverridable, groupKey: "settings.sandboxSettings.carryTitle"},
		{titleKey: "settings.sandboxSettings.ghTitle", alsoKey: "settings.sandboxSettings.ghSearchWords", key: ghTokenKey, scope: scopeOverridable, groupKey: "settings.sandboxSettings.carryTitle"},
	}, unported: sandboxRungEntries()},
	{id: sectionVersionControl, labelKey: "settings.settings.section.versionControl", icon: icons.Lucide("git-branch"), navGroupKey: navGroupWorkspace, rows: []settingEntry{
		{titleKey: "settings.vcsToolsSetting.title", alsoKey: "settings.vcsToolsSetting.searchWords", key: vcsToolsKey},
		{titleKey: "settings.versionControlSettings.account", alsoKey: "settings.versionControlSettings.accountSearchWords", key: vcsAccountKey, scope: scopeProjectOnly},
	}},
	{id: sectionServices, labelKey: "settings.settings.section.services", icon: icons.Lucide("server"), navGroupKey: navGroupWorkspace, rows: []settingEntry{
		{titleKey: "settings.servicesSettings.title", alsoKey: "settings.servicesSettings.searchWords", key: servicesKey, scope: scopeProjectOnly},
	}},
	{id: sectionExtensions, labelKey: "settings.settings.section.extensions", icon: icons.Lucide("puzzle"), navGroupKey: navGroupWorkspace, rows: []settingEntry{
		{titleKey: "settings.extensionsSettings.installRow", alsoKey: "settings.extensionsSettings.installSearchWords", key: extensionsInstallKey},
		{titleKey: "settings.extensionsSettings.listRow", alsoKey: "settings.extensionsSettings.listSearchWords", key: extensionsListKey},
	}},
	{id: sectionUpdates, labelKey: "settings.settings.section.updates", icon: icons.Lucide("download"), footer: true, rows: []settingEntry{
		{titleKey: "settings.updatesSettings.applicationTitle", alsoKey: "settings.updatesSettings.applicationSearchWords", key: updatesAppKey},
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

// parseSettingsSection reads a section id, or the section of a "section/page"
// one: one this build has no section for opens the default.
func parseSettingsSection(raw string) string {
	id, _, _ := strings.Cut(raw, pageSeparator)
	if slices.ContainsFunc(settingsSections, func(s settingsSection) bool { return s.id == id }) {
		return id
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

// path is the line a result shows above its title: the section, then the
// group; "" when it would repeat the title.
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

// searchSettings searches the rows and the sections: a title that is the
// query beats one that starts with it, which beats one that contains it, and both beat a match on the also
// words, a row's group among them; ties keep the nav's order. providers are
// the enabled ones: a block of a provider's screen is a result once per
// provider, named after it, and none at all without any.
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
		case t == needle:
			found = append(found, scored{hit, -1})
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
