package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
)

// The keys below are built at runtime or sit in tables, which the source
// scan of TestEveryKeyTheSourceNamesIsInTheCatalog does not see. The other
// locales hold every key English does (TestEveryLocaleMatchesEnglish).
func TestEverySettingsKeyBuiltAtRuntimeIsInEveryLocale(t *testing.T) {
	var keys []string
	add := func(k ...string) { keys = append(keys, k...) }
	for _, s := range settingsSections {
		add(s.labelKey)
		for _, e := range s.entries() {
			for _, k := range []string{e.titleKey, e.alsoKey, e.groupKey} {
				if k != "" {
					add(k)
				}
			}
		}
	}
	for _, c := range closeChoices {
		add(c.labelKey, c.consequenceKey)
	}
	for _, r := range append(append([]ladderRung{}, skipRungs...), restoreRungs...) {
		add(r.labelKey, r.consequenceKey)
	}
	add(vcsGit.without, vcsGh.without)
	for _, k := range pluginHints {
		add(k)
	}
	for _, rung := range sandboxRungs {
		add("settings.sandboxSettings.rung." + rung)
		if rung != rungEverywhere {
			add("settings.sandboxSettings.exposes." + rung)
		}
	}
	for _, k := range keys {
		if !i18n.Has(k) {
			t.Errorf("no message %q", k)
		}
	}
}
