package api

import (
	"testing"
	"time"
)

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"45m", now.Add(45 * time.Minute), true},
		{"2h", now.Add(2 * time.Hour), true},
		{"2h30", now.Add(150 * time.Minute), true},
		{"2H30M", now.Add(150 * time.Minute), true},
		{" 18:30 ", time.Date(2026, 11, 8, 18, 30, 0, 0, time.UTC), true},
		{"9:05", time.Date(2026, 11, 9, 9, 5, 0, 0, time.UTC), true},
		{"16:00", time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC), true},
		{"2026-11-12 14:30", time.Date(2026, 11, 12, 14, 30, 0, 0, time.UTC), true},
		{"2026-11-08 15:00", time.Time{}, false},
		{"0m", time.Time{}, false},
		{"0h", time.Time{}, false},
		{"25:00", time.Time{}, false},
		{"12:60", time.Time{}, false},
		{"2026-13-01 10:00", time.Time{}, false},
		{"tomorrow", time.Time{}, false},
		{"", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := ParseWhen(c.in, now)
		if ok != c.ok {
			t.Errorf("ParseWhen(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && !got.Equal(c.want) {
			t.Errorf("ParseWhen(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
