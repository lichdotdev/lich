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
	case RU:
		return russianForm(n)
	}
	return "other"
}

// russianForm is Russian's CLDR rule for integers: one is 1, 21, 31... but
// not 11; few is 2-4, 22-24... but not 12-14; many is every other integer.
// CLDR's other is left to fractions, which a count never is.
func russianForm(n int) string {
	switch ones, tens := n%10, n%100; {
	case ones == 1 && tens != 11:
		return "one"
	case ones >= 2 && ones <= 4 && (tens < 12 || tens > 14):
		return "few"
	}
	return "many"
}
