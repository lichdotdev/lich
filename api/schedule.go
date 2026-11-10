package api

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The conditions a session's watch waits for, read from the pull request of
// the session's branch.
const (
	WatchChecksFailed = "checks-failed"
	WatchChecksPassed = "checks-passed"
	WatchReview       = "review"
)

// Who set a scheduled prompt or a watch, when not the person at the window.
const (
	ScheduledByAgent = "agent"
	ScheduledByLich  = "lich"
)

// WatchRunLimit is how many times a watch fires on one pull request before it
// stops. An agent answering failing checks with commits is a loop nobody is
// watching, and three tries is where a person should look.
const WatchRunLimit = 3

var (
	RelaySetSchedule  = define[SetScheduleOptions, None]("relay.SetSchedule")
	RelayScheduleSelf = define[ScheduleSelfOptions, None]("relay.ScheduleSelf")
)

// SessionWatch is a session's condition and the prompt typed when it is met.
// On is "" for a session watching nothing. Runs, PR and Seen outlive a disarm
// so an agent re-arming its own watch cannot reset its run limit.
type SessionWatch struct {
	On     string `json:"on"`
	Prompt string `json:"prompt"`
	By     string `json:"by"`
	Runs   int    `json:"runs"`
	PR     int    `json:"pr"`
	Seen   string `json:"seen"`
}

// SetScheduleOptions is the person's trigger for a session. At and On are
// exclusive; neither clears what the session holds.
type SetScheduleOptions struct {
	SessionID string `json:"sessionId"`
	At        int64  `json:"at"`
	On        string `json:"on"`
	Prompt    string `json:"prompt"`
}

// ScheduleSelfOptions is an agent's trigger for the session it runs in.
type ScheduleSelfOptions struct {
	From   string `json:"from"`
	At     int64  `json:"at"`
	On     string `json:"on"`
	Prompt string `json:"prompt"`
}

// PRRead is one read of an armed checkout's pull request. Head, Checks and
// Review are only filled while it is open. Review is the latest submitted
// review's id, "" for none.
type PRRead struct {
	Path   string
	Number int
	Open   bool
	Head   string
	Checks ChecksRollup
	Review string
}

var (
	whenMinutes  = regexp.MustCompile(`^(\d+)m$`)
	whenHours    = regexp.MustCompile(`^(\d+)h(?:(\d+)m?)?$`)
	whenClock    = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
	whenDateTime = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})\s+(\d{1,2}:\d{2})$`)
)

// ParseWhen reads the one form a scheduled time is written in, by the window's
// field and by `lich schedule --when`: 45m, 2h30, 14:30 or 2026-11-12 14:30.
// Digits only, so it reads the same in every interface language. A clock that
// has passed today means tomorrow; a full date in the past is refused.
func ParseWhen(raw string, now time.Time) (time.Time, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if m := whenMinutes.FindStringSubmatch(s); m != nil {
		return after(now, digits(m[1])*time.Minute)
	}
	if m := whenHours.FindStringSubmatch(s); m != nil {
		return after(now, digits(m[1])*time.Hour+digits(m[2])*time.Minute)
	}
	if whenClock.MatchString(s) {
		at, ok := atClock(now, s)
		if ok && !at.After(now) {
			at = at.AddDate(0, 0, 1)
		}
		return at, ok
	}
	if m := whenDateTime.FindStringSubmatch(s); m != nil {
		at, err := time.ParseInLocation("2006-01-02 15:04", m[1]+"-"+m[2]+"-"+m[3]+" "+zeroPadded(m[4]), now.Location())
		return at, err == nil && at.After(now)
	}
	return time.Time{}, false
}

func after(now time.Time, d time.Duration) (time.Time, bool) {
	return now.Add(d), d > 0
}

func digits(s string) time.Duration {
	n, _ := strconv.Atoi(s) // empty for 2h, and the patterns allow only digits
	return time.Duration(n)
}

func atClock(now time.Time, clock string) (time.Time, bool) {
	t, err := time.Parse("15:04", zeroPadded(clock))
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), true
}

func zeroPadded(clock string) string {
	if len(clock) == len("9:30") {
		return "0" + clock
	}
	return clock
}
