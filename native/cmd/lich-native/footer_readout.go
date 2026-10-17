package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lichdotdev/lich/native/i18n"
)

// usageTone is usageColor (ContextRing.tsx): one 0–100 scale for every
// bounded share, so a budget half spent and a context half full read alike.
type usageTone int

const (
	toneMuted usageTone = iota
	toneWait
	toneDestructive
)

func toneOf(percent float64) usageTone {
	switch {
	case percent >= 95:
		return toneDestructive
	case percent >= 80:
		return toneWait
	}
	return toneMuted
}

// footerReadout is what FooterReadout draws: Text in the strip, and a tooltip
// headed by Title (Label when empty) over the Detail lines. Label is also the
// accessible name.
type footerReadout struct {
	Label  string
	Title  string
	Text   string
	Detail []string
	Tone   usageTone
}

// contextReadout is FooterSession's SessionContext. A zero window is a
// provider that reports spend and no window: no ring then.
func contextReadout(u sessionUsage) (footerReadout, bool) {
	if u.Window <= 0 {
		return footerReadout{}, false
	}
	return footerReadout{
		Label:  i18n.T("shell.footerSession.contextWindow"),
		Text:   fmt.Sprintf("%d%%", u.Percent),
		Detail: []string{i18n.T("shell.footerSession.contextTokens", i18n.Args{"tokens": groupThousands(u.Tokens), "window": groupThousands(u.Window)})},
		Tone:   toneOf(float64(u.Percent)),
	}, true
}

// groupThousands is toLocaleString in English: 128000 → "128,000".
func groupThousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// sessionModelText is SessionModel's slot: the short model name and the
// full id for its hint. False for a report with no model — the cost-only
// rung — so the slot stays empty rather than drawing a glyph beside nothing.
func sessionModelText(u sessionUsage) (text, hint string, ok bool) {
	if u.Model == "" {
		return "", "", false
	}
	text, hint = formatModel(u.Model), u.Model
	if u.Effort != "" {
		text += " · " + u.Effort
		hint += " · " + u.Effort
	}
	return text, hint, true
}

var modelSnapshot = regexp.MustCompile(`-\d{8}$`)

// formatModel shortens a Claude model id: "claude-opus-4-8" → "opus 4.8",
// "claude-haiku-4-5-20251001" → "haiku 4.5". Other ids pass through.
func formatModel(id string) string {
	stripped, ok := strings.CutPrefix(id, "claude-")
	if !ok {
		return id
	}
	stripped = modelSnapshot.ReplaceAllString(stripped, "")
	family, version, found := strings.Cut(stripped, "-")
	if !found {
		return stripped
	}
	return family + " " + strings.ReplaceAll(version, "-", ".")
}

// clockText is FooterClock in en-US: "Sun, Oct 11, 2026 · 02:30 PM".
func clockText(now time.Time) string {
	return now.Format("Mon, Jan 02, 2006 · 03:04 PM")
}

// untilNextMinute is how long until the wall clock turns over, the cadence
// every minute-resolution readout re-reads on (use-now.ts).
func untilNextMinute(now time.Time) time.Duration {
	return now.Truncate(time.Minute).Add(time.Minute).Sub(now)
}
