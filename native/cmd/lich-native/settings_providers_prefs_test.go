package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// TestChatPrefOptions pins the list under the trigger: the provider's default
// first, a stored value the last chat no longer offered kept, and the search
// matching the label or the id.
func TestChatPrefOptions(t *testing.T) {
	choices := []api.ChatChoice{{ID: "opus", Label: "Opus 5.5"}, {ID: "sonnet", Label: "Sonnet 5.5"}}
	ids := func(opts []api.ChatChoice) []string {
		var out []string
		for _, o := range opts {
			out = append(out, o.ID)
		}
		return out
	}
	for _, tc := range []struct {
		current, query string
		want           []string
	}{
		{"", "", []string{"", "opus", "sonnet"}},
		{"haiku", "", []string{"", "opus", "sonnet", "haiku"}},
		{"", "SON", []string{"", "sonnet"}},
		{"", "opus", []string{"", "opus"}},
	} {
		if got := ids(chatPrefOptions(choices, tc.current, tc.query)); !slices.Equal(got, tc.want) {
			t.Errorf("chatPrefOptions(%q, %q) = %q, want %q", tc.current, tc.query, got, tc.want)
		}
	}
	if len(choices) != 2 {
		t.Errorf("chatPrefOptions grew the caller's list: %v", choices)
	}
}

func TestChatPrefLabelAndShown(t *testing.T) {
	choices := chatPrefChoices(`[{"id":"opus","label":"Opus 5.5"}]`)
	if got := chatPrefLabel(choices, "opus", "Claude Code"); got != "Opus 5.5" {
		t.Errorf("label of a listed choice = %q", got)
	}
	if got := chatPrefLabel(choices, "gone", "Claude Code"); got != "gone" {
		t.Errorf("label of an unlisted value = %q, want the id", got)
	}
	if chatPrefChoices("not json") != nil {
		t.Error("an unreadable list lists something")
	}
	if !chatPrefShown(api.OptionModel, nil, "") || chatPrefShown(api.OptionEffort, nil, "") || !chatPrefShown(api.OptionEffort, nil, "high") {
		t.Error("the model row always shows; the effort row only with a choice or a value")
	}
}
