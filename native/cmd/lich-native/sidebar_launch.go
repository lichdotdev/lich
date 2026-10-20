package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/i18n"
)

// providerDefaultKey is store.providerDefaultKey in Go: the provider an
// implicit new session spawns, globally or per project.
const providerDefaultKey = "provider.default"

// launchProvider is a provider's state, trimmed to what the launch menu and
// the implicit kind read.
type launchProvider struct {
	id        string
	name      string
	installed bool
	enabled   bool
}

func providerEnabledKey(id string) string { return "provider." + id + ".enabled" }

// readEnabled: Claude is on unless switched off (it was always offered
// before providers could be chosen); every other provider is opt-in.
func readEnabled(id, value string) bool {
	switch value {
	case "1":
		return true
	case "0":
		return false
	}
	return id == "claude"
}

// enabledProviders are the ones a launch menu offers, installed or not: a
// binary set in one project's scope is invisible to detection, and a truly
// missing one surfaces as a PTY error.
func enabledProviders(list []launchProvider) []launchProvider {
	return slices.DeleteFunc(slices.Clone(list), func(p launchProvider) bool { return !p.enabled })
}

// resolveProjectDefaultProvider: an enabled project default, else an enabled
// global default, else the first enabled provider, else Claude.
func resolveProjectDefaultProvider(list []launchProvider, globalDefault, projectDefault string) string {
	enabled := enabledProviders(list)
	for _, id := range []string{projectDefault, globalDefault} {
		if slices.ContainsFunc(enabled, func(p launchProvider) bool { return p.id == id }) {
			return id
		}
	}
	if len(enabled) > 0 {
		return enabled[0].id
	}
	return "claude"
}

// implicitSessionKind is what a session nobody picked a kind for spawns. On a
// machine where detection found no binary at all it is the shell, where the
// install command gets pasted, rather than a card failing on `claude: command
// not found`.
// ponytail: skips the per-project binary overrides (provider.<id>.bin in
// project scope), so a bare machine whose project points at a binary still
// opens a shell; read them here when that case shows up.
func implicitSessionKind(list []launchProvider, globalDefault, projectDefault string) string {
	if len(list) > 0 && !slices.ContainsFunc(list, func(p launchProvider) bool { return p.installed }) {
		return "shell"
	}
	return resolveProjectDefaultProvider(list, globalDefault, projectDefault)
}

// launchCheckout is a checkout a folder's + can open a session in, titled like
// its block.
type launchCheckout struct {
	path  string
	label string
}

// folderLaunchHeading is the menu label: a folder of one
// checkout opens there and says so; with more, each checkout is a submenu.
func folderLaunchHeading(folder string, checkouts []launchCheckout) string {
	if len(checkouts) == 1 {
		return i18n.T("sidebar.folderLaunchMenuItems.newSessionInOn", i18n.Args{"folder": folder, "checkout": checkouts[0].label})
	}
	return i18n.T("sidebar.folderLaunchMenuItems.newSessionIn", i18n.Args{"folder": folder})
}
