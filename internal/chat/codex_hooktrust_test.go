package chat

import (
	"encoding/json"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// Only a lich-plugin PreToolUse hook Codex skips counts: the shape below is
// hooks/list's on 0.170.0, measured.
func TestLichHookTrustFindsTheSkippedHook(t *testing.T) {
	list := func(entries string) codexHookList {
		var l codexHookList
		if err := json.Unmarshal([]byte(`{"data":[{"hooks":[`+entries+`]}]}`), &l); err != nil {
			t.Fatal(err)
		}
		return l
	}
	lich := func(event, trust string) string {
		return `{"key":"lich@lich-plugin:hooks/codex-hooks.json:pre_tool_use:0:0","eventName":"` + event +
			`","source":"plugin","pluginId":"lich@lich-plugin","trustStatus":"` + trust + `","enabled":true}`
	}
	other := `{"key":"/home/u/.codex/hooks.json:pre_tool_use:0:0","eventName":"preToolUse","source":"user","trustStatus":"modified","enabled":true}`
	for name, c := range map[string]struct {
		entries, want string
	}{
		"trusted":                  {lich("preToolUse", "trusted"), ""},
		"modified":                 {lich("preToolUse", "modified"), api.HookModified},
		"untrusted":                {lich("preToolUse", "untrusted"), api.HookUntrusted},
		"another event":            {lich("stop", "modified"), ""},
		"someone else's hook":      {other, ""},
		"no lich-plugin installed": {"", ""},
	} {
		if got := lichHookTrust(list(c.entries)); got != c.want {
			t.Errorf("%s: lichHookTrust = %q, want %q", name, got, c.want)
		}
	}
}
