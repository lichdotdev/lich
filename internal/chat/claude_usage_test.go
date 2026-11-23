package chat

import "testing"

func TestContextWindowIsTheSessionsModel(t *testing.T) {
	both := map[string]claudeModelUsage{"claude-opus-5-5": {ContextWindow: 1_000_000}, "claude-haiku-5-5": {ContextWindow: 200_000}}
	for _, tc := range []struct {
		name   string
		models map[string]claudeModelUsage
		model  string
		want   int
	}{
		{"by its name", both, "claude-haiku-5-5", 200_000},
		{"a 1M variant listed without its suffix", both, "claude-opus-5-5[1m]", 1_000_000},
		{"the one model that ran", map[string]claudeModelUsage{"claude-opus-5-5": {ContextWindow: 1_000_000}}, "opus", 1_000_000},
		{"none of several", both, "opus", 0},
	} {
		if got := contextWindow(tc.models, tc.model); got != tc.want {
			t.Errorf("%s: window = %d, want %d", tc.name, got, tc.want)
		}
	}
}
