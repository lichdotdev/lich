package main

import (
	"reflect"
	"testing"
)

func TestModelRuns(t *testing.T) {
	same := func(text string) []modelRun { return []modelRun{{text: text}} }
	changed := func(requested, running string) []modelRun {
		return []modelRun{{text: "Opened on "}, {text: requested, name: true}, {text: ", running "}, {text: running, name: true}}
	}
	cases := []struct {
		name, requested, running string
		want                     []modelRun
	}{
		{"default birth", "", "claude-opus-5-5", same("opus 5.5")},
		{"alias", "opus", "claude-opus-5-5", same("opus 5.5")},
		{"alias with context suffix", "sonnet[1m]", "claude-sonnet-5-5", same("sonnet 5.5")},
		{"snapshot suffix", "claude-haiku-4-5", "claude-haiku-4-5-20251001", same("haiku 4.5")},
		{"full id", "gpt-5.5", "gpt-5.5", same("gpt-5.5")},
		{"alias of another family", "sonnet", "claude-opus-5-5", changed("sonnet", "opus 5.5")},
		{"another version", "claude-opus-4-8", "claude-opus-5-5", changed("opus 4.8", "opus 5.5")},
		{"codex switch", "gpt-5.2", "gpt-5.5", changed("gpt-5.2", "gpt-5.5")},
	}
	for _, c := range cases {
		if got := modelRuns(c.requested, c.running); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: modelRuns(%q, %q) = %+v, want %+v", c.name, c.requested, c.running, got, c.want)
		}
	}
}
