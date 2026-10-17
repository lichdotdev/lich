package main

import (
	"time"

	"github.com/omartelo/lich/native/i18n"
)

// formatAge is lib/session/session-age.ts: how long a session has been in its
// status, in one floored unit (seconds, then minutes, then hours), so the
// readout never claims time that has not passed. A backwards clock reads as
// none.
//
// The unit is a catalog message so each language writes it as Intl's narrow
// unit does ("3 h" in pt-BR, "3小时" in zh-CN).
func formatAge(elapsed time.Duration) string {
	elapsed = max(elapsed, 0)
	switch {
	case elapsed < time.Minute:
		return i18n.T("session.age.second", i18n.Args{"count": int(elapsed / time.Second)})
	case elapsed < time.Hour:
		return i18n.T("session.age.minute", i18n.Args{"count": int(elapsed / time.Minute)})
	}
	return i18n.T("session.age.hour", i18n.Args{"count": int(elapsed / time.Hour)})
}

// nextAgeChange is how long until formatAge(elapsed) reads differently, so a
// frame is asked for when the readout moves rather than on a fixed tick.
func nextAgeChange(elapsed time.Duration) time.Duration {
	elapsed = max(elapsed, 0)
	switch {
	case elapsed < time.Minute:
		return time.Second - elapsed%time.Second
	case elapsed < time.Hour:
		return time.Minute - elapsed%time.Minute
	}
	return time.Hour - elapsed%time.Hour
}
