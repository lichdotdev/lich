package i18n

// pluralForm is the CLDR plural category of an integer count in l, what
// Intl.PluralRules(l).select(n) answers in the web window. Only the forms the
// catalogs use are told apart: Spanish's "many" (1000000 and up) has no
// message of its own and reads "other" either way.
func pluralForm(l Locale, n int) string {
	switch l {
	case PtBR:
		if n == 0 || n == 1 {
			return "one"
		}
	case EN, ES:
		if n == 1 {
			return "one"
		}
	}
	return "other"
}
