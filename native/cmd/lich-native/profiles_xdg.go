package main

import "github.com/omartelo/lich/native/lichclient"

// opencode, Cursor CLI and Kiro CLI have profiles only where the backend keeps
// them (internal/xdgprofile, Linux): xdgProfileProviders is empty elsewhere.
// Each logs in from a terminal card and has no plan gauge per profile.
const (
	opencodeKind = "opencode"
	cursorKind   = "cursor"
	kiroKind     = "kiro"
)

// xdgProfileProvider is the profile provider of kind, whose service is
// "<kind>profile" and whose setting is provider.<kind>.profile.
func xdgProfileProvider(kind, label string, words profileWords) profileProvider {
	return profileProvider{
		kind: kind, key: "provider." + kind + ".profile", label: label,
		api:           func(c *lichclient.Client) lichclient.ProfileAPI { return c.XDGProfiles(kind) },
		terminalLogin: true, words: words,
	}
}

var opencodeProfileProvider = func() profileProvider {
	p := xdgProfileProvider(opencodeKind, "opencode", profileWords{
		profileDescKey:  "settings.profiles.profileDesc",
		profilesDescKey: "settings.profilesXdg.opencode.profilesDesc",
		searchAlsoKey:   "settings.profilesXdg.opencode.searchAlso",
		profilesAlsoKey: "settings.profilesXdg.opencode.profilesAlso",
		newDescKey:      "settings.profilesXdg.opencode.newDesc",
		renameDescKey:   "settings.profilesXdg.opencode.renameDesc",
		deleteDescKey:   "settings.profilesXdg.opencode.deleteDesc",
	})
	// opencode keeps a conversation in the database that holds the login.
	p.noContinue = true
	return p
}()

var cursorProfileProvider = xdgProfileProvider(cursorKind, "Cursor CLI", profileWords{
	profileDescKey:  "settings.profiles.profileDesc",
	profilesDescKey: "settings.profilesXdg.cursor.profilesDesc",
	searchAlsoKey:   "settings.profilesXdg.cursor.searchAlso",
	profilesAlsoKey: "settings.profilesXdg.cursor.profilesAlso",
	newDescKey:      "settings.profilesXdg.cursor.newDesc",
	renameDescKey:   "settings.profilesXdg.cursor.renameDesc",
	deleteDescKey:   "settings.profilesXdg.cursor.deleteDesc",
})

var kiroProfileProvider = xdgProfileProvider(kiroKind, "Kiro CLI", profileWords{
	profileDescKey:  "settings.profiles.profileDesc",
	profilesDescKey: "settings.profilesXdg.kiro.profilesDesc",
	searchAlsoKey:   "settings.profilesXdg.kiro.searchAlso",
	profilesAlsoKey: "settings.profilesXdg.kiro.profilesAlso",
	newDescKey:      "settings.profilesXdg.kiro.newDesc",
	renameDescKey:   "settings.profilesXdg.kiro.renameDesc",
	deleteDescKey:   "settings.profilesXdg.kiro.deleteDesc",
})

// xdgProfileProviderOf is the link-home provider of kind on this platform.
func xdgProfileProviderOf(kind string) (profileProvider, bool) {
	for _, p := range xdgProfileProviders {
		if p.kind == kind {
			return p, true
		}
	}
	return profileProvider{}, false
}

func xdgProfileBooks(c *lichclient.Client, invalidate func(), quota *planQuota) map[string]*profileBook {
	books := map[string]*profileBook{}
	for _, p := range xdgProfileProviders {
		books[p.kind] = newProfileBook(p, c, invalidate, quota)
	}
	return books
}

// xdgProfileKinds are the kinds of xdgProfileProviders.
func xdgProfileKinds() []string {
	kinds := make([]string, len(xdgProfileProviders))
	for i, p := range xdgProfileProviders {
		kinds[i] = p.kind
	}
	return kinds
}
