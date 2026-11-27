package cli

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// The MCP instructions say a subagent becomes a card only where the session's
// provider opens cards, as lich's store says; with lich out of reach they read
// as before the setting existed.
func TestMCPInstructionsFollowTheCardSetting(t *testing.T) {
	t.Parallel()
	cards := prompt.For(prompt.English).MCPSubagentCards
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"cards on", 200, `{"cards":true}`, true},
		{"cards off", 200, `{"cards":false}`, false},
		{"lich out of reach", 500, `"boom"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLich(t, `null`)
			f.answers = map[string]answer{api.SubagentCardsOn.Name: {status: tc.status, body: tc.body}}
			c := &client{env: f.env, version: testVersion, running: noInstance}
			got := mcpInstructions(prompt.English, c.mcpCards(prompt.English))
			if strings.Contains(got, cards) != tc.want {
				t.Errorf("card sentences present = %v, want %v", !tc.want, tc.want)
			}
			if call := f.only(t); call.method != api.SubagentCardsOn.Name || optionsOf[api.SessionIDOptions](t, call).ID != "s1" {
				t.Errorf("called %s %v, want subagent.CardsOn for s1", call.method, call.args)
			}
		})
	}
}
