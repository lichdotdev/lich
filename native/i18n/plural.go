package i18n

import format "github.com/lichdotdev/lich/extension/i18n"

// pluralForm is the CLDR plural category of an integer count in l.
func pluralForm(l Locale, n int) string { return format.PluralForm(string(l), n) }
