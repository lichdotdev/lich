package i18n

import format "github.com/lichdotdev/lich/extension/i18n"

// System is the locale the operating system's own language matches, English
// when it matches none: the interface language before anyone picks one.
func System() Locale { return Match(systemTag()) }

func tagFromEnv(getenv func(string) string) string { return format.TagFromEnv(getenv) }
