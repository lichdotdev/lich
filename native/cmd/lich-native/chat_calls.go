package main

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// callKind is what a call did, as a group's label counts it and a call's row
// names it. The order is the label's: what changed the checkout first.
type callKind int

const (
	callEdited callKind = iota
	callRan
	callRead
	callSearched
	callFetched
	callOther
)

// callKeys are each kind's words under chat.view: the row's verb, the verb
// while it runs, and the label's clause. callOther has no verb: its row
// names the tool.
var callKeys = [...]struct{ past, running, clause string }{
	callEdited:   {"chat.view.call.edited", "chat.view.call.editing", "chat.view.calls.edited"},
	callRan:      {"chat.view.call.ran", "chat.view.call.running", "chat.view.calls.ran"},
	callRead:     {"chat.view.call.read", "chat.view.call.reading", "chat.view.calls.read"},
	callSearched: {"chat.view.call.searched", "chat.view.call.searching", "chat.view.calls.searched"},
	callFetched:  {"chat.view.call.fetched", "chat.view.call.fetching", "chat.view.calls.fetched"},
	callOther:    {"", "", "chat.view.calls.others"},
}

// kindOf is what the tool called name does, read off the glyph toolIcons
// gives it, so a name the chat learns has one place to land.
func kindOf(name string) callKind {
	switch toolIcon(name) {
	case icToolEdit:
		return callEdited
	case icToolRun:
		return callRan
	case icToolRead:
		return callRead
	case icToolFind:
		return callSearched
	case icToolWeb:
		return callFetched
	}
	return callOther
}

// callVerb is a call's row verb: its kind's, in the running form while it
// runs; a tool of no known kind is its own name, an MCP tool's without the
// server it came from.
func callVerb(t *backend.ChatToolCall, running bool) string {
	k := kindOf(t.Name)
	if k == callOther {
		name := t.Name
		if rest, ok := strings.CutPrefix(name, "mcp__"); ok {
			if _, tool, ok := strings.Cut(rest, "__"); ok {
				name = tool
			}
		}
		return name
	}
	if running {
		return i18n.T(callKeys[k].running)
	}
	return i18n.T(callKeys[k].past)
}

// callSummary is a group's label: a clause per kind of call it made, in
// callKind's order, its first letter raised; and how many failed, "" for none.
// A call vetoed says it failed but was opened as a card: it is not counted.
func callSummary(tools []backend.ChatItem, vetoed func(backend.ChatItem) bool) (label, failed string) {
	var counts [callOther + 1]int
	fails := 0
	for _, it := range tools {
		if it.Tool == nil {
			continue
		}
		counts[kindOf(it.Tool.Name)]++
		if it.Tool.State == api.ToolFailed && !vetoed(it) {
			fails++
		}
	}
	var clauses []string
	for k, n := range counts {
		if n > 0 {
			clauses = append(clauses, i18n.T(callKeys[k].clause, i18n.Args{"count": n}))
		}
	}
	label = raiseFirst(strings.Join(clauses, i18n.T("chat.view.calls.sep")))
	if fails > 0 {
		failed = i18n.T("chat.view.calls.failed", i18n.Args{"count": fails})
	}
	return label, failed
}

// raiseFirst is s with its first letter in upper case; a script without case
// is left as it is.
func raiseFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// runningCall is the group's call running now, its last one to start.
func runningCall(tools []backend.ChatItem) (backend.ChatItem, bool) {
	for _, tool := range slices.Backward(tools) {
		if callRunning(tool) {
			return tool, true
		}
	}
	return backend.ChatItem{}, false
}
