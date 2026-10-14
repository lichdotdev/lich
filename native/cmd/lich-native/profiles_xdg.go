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
		profileDesc:  "Which opencode login new sessions use. A session keeps the profile it was born in, resumes included.",
		profilesDesc: "Each profile is its own opencode login with its own history. opencode's config, plugins, agents and MCP servers are the same in all of them.",
		searchAlso:   "opencode account login personal work",
		profilesAlso: "opencode accounts logins personal work log in",
		newDesc:      "An opencode login of its own, with its own history. You log in from a terminal once it is made.",
		renameDesc:   "Only its name changes: its login and history stay as they are.",
		deleteDesc:   "Its login and conversation history are removed, and sessions started in it can no longer be resumed.",
	})
	// opencode keeps a conversation in the database that holds the login.
	p.noContinue = true
	return p
}()

var cursorProfileProvider = xdgProfileProvider(cursorKind, "Cursor CLI", profileWords{
	profileDesc:  "Which Cursor login new sessions use. A session keeps the profile it was born in, resumes included.",
	profilesDesc: "Each profile is its own Cursor login with its own chats and CLI settings. mcp.json, hooks and skills in ~/.cursor are the same in all of them.",
	searchAlso:   "cursor cursor-agent account login personal work",
	profilesAlso: "cursor cursor-agent accounts logins personal work log in",
	newDesc:      "A Cursor login of its own, with its own chats. You log in from a terminal once it is made.",
	renameDesc:   "Only its name changes: its login and chats stay as they are.",
	deleteDesc:   "Its login and chats are removed, and sessions started in it can no longer be resumed.",
})

var kiroProfileProvider = xdgProfileProvider(kiroKind, "Kiro CLI", profileWords{
	profileDesc:  "Which Kiro login new sessions use. A session keeps the profile it was born in, resumes included.",
	profilesDesc: "Each profile is its own Kiro login. Conversations, agents, steering and settings in ~/.kiro are the same in all of them.",
	searchAlso:   "kiro kiro-cli account login personal work aws builder",
	profilesAlso: "kiro kiro-cli accounts logins personal work log in",
	newDesc:      "A Kiro login of its own. You log in from a terminal once it is made.",
	renameDesc:   "Only its name changes: its login stays as it is.",
	deleteDesc:   "Its login is removed and sessions started in it can no longer be resumed. Conversations stay: every profile shares them.",
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
