package i18n

import "strings"

// System is the locale the operating system's own language matches, English
// when it matches none: the interface language before anyone picks one.
func System() Locale { return Match(systemTag()) }

// tagFromEnv is the POSIX locale the environment names, in the order glibc
// reads it for messages (LC_ALL, LC_MESSAGES, LANG), as a BCP 47 tag:
// pt_BR.UTF-8@euro reads pt-BR. C and POSIX name no language.
func tagFromEnv(getenv func(string) string) string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := getenv(name)
		if v == "" {
			continue
		}
		v, _, _ = strings.Cut(v, ".")
		v, _, _ = strings.Cut(v, "@")
		if v == "C" || v == "POSIX" {
			return ""
		}
		return strings.ReplaceAll(v, "_", "-")
	}
	return ""
}
