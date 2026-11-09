package scope

import (
	"context"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// The denials this policy exists for, pinned by name: each is a way out of the
// sandbox through lich's own loopback.
func TestDeniesTheEscapes(t *testing.T) {
	for _, method := range []string{
		"services.Start", "services.Save", "services.StartAll", "services.Restart",
		"store.SetSetting", "terminal.Write", "terminal.Start", "terminal.ContinueIn",
		"project.ReadFile", "project.Open", "drop.Attach", "system.OpenExternal",
		"system.Quit", "appupdate.Apply", "agentplugin.Install", "browser.Show",
		"relay.SetShown", "relay.SetSchedule", "relay.AnnounceMerge",
	} {
		if Allows(method) {
			t.Errorf("%s is allowed to a confined session", method)
		}
	}
	if Allows("") {
		t.Error("an empty method is allowed")
	}
}

// What a confined agent does through lich-plugin and `lich`: the relay, opening
// sessions, its own browser, reading services.
func TestAllowsWhatAConfinedSessionUses(t *testing.T) {
	for _, method := range []string{
		"relay.Send", "relay.Reply", "relay.Collect", "spawn.Open", "spawn.OpenSubagent",
		"spawn.Handoff", "browser.Open", "services.List", "terminal.Ready", "store.CostTotals",
	} {
		if !Allows(method) {
			t.Errorf("%s is refused to a confined session", method)
		}
	}
}

// Every entry names a method api declares, so a renamed method cannot leave a
// dead entry behind that looks like a grant.
func TestAllowedMethodsAreDeclared(t *testing.T) {
	declared := api.Methods()
	for method := range allowed {
		if _, ok := declared[method]; !ok {
			t.Errorf("%s is allowed and not declared in api", method)
		}
	}
}

func TestCallerFieldNamesTheSession(t *testing.T) {
	if got := CallerField("relay.Send"); got != "from" {
		t.Errorf("relay.Send caller field = %q, want from", got)
	}
	if got := CallerField("browser.Open"); got != "id" {
		t.Errorf("browser.Open caller field = %q, want id", got)
	}
	// A handoff waits on its successor, so terminal.Ready's id is another
	// session's by design.
	if got := CallerField("terminal.Ready"); got != "" {
		t.Errorf("terminal.Ready caller field = %q, want none", got)
	}
}

func TestSessionRidesTheContext(t *testing.T) {
	if got := Session(context.Background()); got != "" {
		t.Errorf("a bare context names session %q", got)
	}
	if got := Session(With(context.Background(), "s1")); got != "s1" {
		t.Errorf("Session = %q, want s1", got)
	}
}

func TestMethodReadsTheRPCPath(t *testing.T) {
	if got := Method("/rpc/relay.Send"); got != "relay.Send" {
		t.Errorf("Method = %q", got)
	}
	for _, path := range []string{"/events", "/ws", "/blob", "/drop", "/rpc"} {
		if got := Method(path); got != "" {
			t.Errorf("Method(%q) = %q, want none", path, got)
		}
	}
}

func TestRefusalsNameTheCall(t *testing.T) {
	if msg := Refusal("services.Start"); !strings.Contains(msg, "services.Start") {
		t.Errorf("refusal does not name the method: %s", msg)
	}
	msg := Impersonation("relay.Send", "other", "mine")
	if !strings.Contains(msg, `"other"`) || !strings.Contains(msg, `"mine"`) {
		t.Errorf("impersonation names neither session: %s", msg)
	}
}
