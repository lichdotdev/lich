// Package scope is what a confined session may ask of lich's loopback
// listener. A session running in the sandbox is handed a token minted for it
// alone (internal/terminal), never the launch-wide one, and every request made
// with it is held to this policy: an allowlist of RPC methods, and on the
// methods that name their caller, the rule that the caller is the session the
// token was minted for.
//
// Deny by default. The list is derived from what a confined agent legitimately
// does through lich-plugin's hooks, the `lich` CLI and `lich mcp`: talk to the
// sessions around it, open and steer sessions (which come out confined too,
// internal/spawn), drive its own browser, read the project's services and its
// cost. Everything that runs a command on the machine outside the sandbox
// (services.Start and the other service writes), moves a setting
// (store.SetSetting), types into another session's PTY (terminal.Write), hands
// the backend a host path, or belongs to the window alone is out.
package scope

import (
	"context"
	"fmt"
	"strings"
)

type callerKey struct{}

// With marks ctx as a request made with a scoped token minted for session.
func With(ctx context.Context, session string) context.Context {
	return context.WithValue(ctx, callerKey{}, session)
}

// Session is the confined session a request was made by, "" for the window,
// the command line outside a sandbox and every other holder of the launch
// token.
func Session(ctx context.Context) string {
	session, _ := ctx.Value(callerKey{}).(string)
	return session
}

// allowed is every RPC method a scoped token may call, with the JSON field of
// its options that names the calling session: "" when the method takes none.
// A field named here must equal the token's session, so a confined agent
// cannot speak as another session to the relay, open a browser on another
// session's checkout or file its workers under someone else.
var allowed = map[string]string{
	"relay.Peers":        "from",
	"relay.Self":         "from",
	"relay.Send":         "from",
	"relay.SendPrivate":  "from",
	"relay.SendSubagent": "from",
	"relay.Wait":         "",
	"relay.Collect":      "from",
	"relay.CollectNow":   "from",
	"relay.Reply":        "from",
	"relay.Insert":       "from",
	"relay.Focus":        "from",
	"relay.Focused":      "",
	"relay.Submit":       "",
	"relay.ScheduleSelf": "from",

	"spawn.Open":         "from",
	"spawn.OpenSubagent": "from",
	"spawn.Handoff":      "from",
	"spawn.Close":        "from",
	"spawn.CloseProject": "from",
	"spawn.Control":      "from",
	"spawn.Ask":          "from",
	"spawn.Worktrees":    "from",
	"spawn.Folders":      "from",
	"spawn.File":         "from",
	"spawn.RenameFolder": "from",
	"spawn.ColorFolder":  "from",
	"spawn.Rename":       "from",
	"spawn.Project":      "from",

	// The session's own browser. A local file it opens is served from its
	// own checkout only (internal/preview), which the sandbox mounts anyway.
	"browser.Open":         "id",
	"browser.Info":         "id",
	"browser.Click":        "id",
	"browser.Type":         "id",
	"browser.Press":        "id",
	"browser.Screenshot":   "id",
	"browser.Navigate":     "id",
	"browser.Reload":       "id",
	"browser.Back":         "id",
	"browser.Forward":      "id",
	"browser.Scroll":       "id",
	"browser.List":         "id",
	"browser.Close":        "id",
	"browser.Eval":         "id",
	"browser.Tabs":         "id",
	"browser.TabAction":    "id",
	"browser.AnswerDialog": "id",

	// Reads only: a service's command runs on the machine, outside the
	// sandbox, so starting, stopping or editing one stays with the window.
	"services.List":       "",
	"services.Output":     "",
	"services.LiveOutput": "",

	// A handoff waits for its successor's prompt, so the id is the successor's.
	"terminal.Ready":     "",
	"store.CostTotals":   "",
	"system.Diagnostics": "",
	"themes.List":        "",
	"themes.Selection":   "",
	"themes.Select":      "",
}

// Allows reports whether a scoped token may call method at all.
func Allows(method string) bool {
	_, ok := allowed[method]
	return ok
}

// CallerField is the options field that must name the calling session, "" when
// the method has none.
func CallerField(method string) string {
	return allowed[method]
}

// Refusal is the error a confined session reads when it calls a method its
// token does not cover.
func Refusal(method string) string {
	return fmt.Sprintf(
		"%s is not something a confined session may ask of lich: this session's token is "+
			"scoped to the sandbox it runs in, and the call would act on the machine outside it",
		method,
	)
}

// Impersonation is the error for an allowed method whose options name another
// session as the caller.
func Impersonation(method, claimed, session string) string {
	return fmt.Sprintf(
		"%s was called as session %q, and this token belongs to session %q: a confined "+
			"session speaks for itself only",
		method, claimed, session,
	)
}

// Method is the RPC method a request path names, "" for anything that is not
// an RPC.
func Method(path string) string {
	method, ok := strings.CutPrefix(path, "/rpc/")
	if !ok {
		return ""
	}
	return method
}
