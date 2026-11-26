package main

import (
	"slices"
	"strings"
)

// The DevTools console runs a script in REPL mode: the value of its last
// expression, top-level await, and a let or const that a later script may
// declare again. WebKitGTK runs a script only as a program or as an async
// function's body, so the script is rewritten: its top-level let, const and
// class become var, which a script may declare again, and its last expression
// is found by JSC's own syntax check (parses) rather than a parser of ours.

// consoleRun is how a script runs: as a program, or with top-level await as
// an async function's body.
type consoleRun struct {
	// program answers [0, value], or [1] once it left a thenable in
	// replValue for body to await. Its value comes back in the same call: a
	// second call to read it ran after a timer the script set, and a dialog
	// that timer opened held the read until someone answered it.
	program, body string
	async         bool
}

// replValue is where a program leaves a thenable, for body to await.
const replValue = "globalThis.__lichConsoleValue"

// awaitValue is the body that answers what a program left in replValue.
const awaitValue = "try { return await " + replValue + "; } finally { delete " + replValue + "; }"

// settled is the program's last expression, expr, answered as program says,
// its value as consoleValue has it.
func settled(expr string) string {
	return "((v) => v !== null && (typeof v === 'object' || typeof v === 'function') && typeof v.then === 'function' ? (" +
		replValue + " = v, [1]) : [0, " + consoleValue + "])((" + expr + "\n))"
}

// consoleForm is script as the console runs it, parses saying whether code
// is a valid script; ok is false for a script that does not parse either way,
// which runs as written so its SyntaxError is the answer.
func consoleForm(script string, parses func(string) bool) (consoleRun, bool) {
	s := redeclarable(script)
	async := !parses(s)
	if async && !parses(asyncBody(s)) {
		return consoleRun{}, false
	}
	prefix, tail, found := lastExpression(s, async, parses)
	switch {
	case async && found:
		return consoleRun{body: prefix + "\n;return (" + tail + "\n);", async: true}, true
	case async:
		return consoleRun{body: s, async: true}, true
	case found:
		return consoleRun{program: prefix + "\n;" + settled(tail), body: awaitValue}, true
	}
	return consoleRun{program: s + "\n;[0]", body: awaitValue}, true
}

func asyncBody(s string) string { return "(async () => {" + s + "\n})" }

// lastExpression splits s at the first statement boundary after which the
// rest is one expression: the first, so "a\n+ 1" stays one expression.
func lastExpression(s string, async bool, parses func(string) bool) (prefix, tail string, ok bool) {
	for _, at := range boundaries(s) {
		tail = strings.TrimSpace(s[at:])
		if tail == "" {
			continue
		}
		expr := "(" + tail + "\n)"
		head := s[:at]
		if async {
			expr, head = "(async () => ("+tail+"\n))", asyncBody(head)
		}
		if parses(expr) && parses(head) {
			return s[:at], tail, true
		}
	}
	return "", "", false
}

// boundaries are where a top-level statement may start in s: its start, past
// each top-level semicolon, and past each top-level newline, where JavaScript
// may insert one.
func boundaries(s string) []int {
	at := []int{0}
	walk(s, 0, false, func(e token) {
		if e.kind == ';' || e.kind == '\n' {
			at = append(at, e.end)
		}
	})
	return at
}

// redeclarable is s with its top-level let and const declarations made var,
// and its class declarations var bindings of the class.
func redeclarable(s string) string {
	var out strings.Builder
	last := 0
	walk(s, 0, false, func(e token) {
		if e.kind != 'w' {
			return
		}
		word, rest := s[e.at:e.end], strings.TrimLeft(s[e.end:], " \t")
		switch {
		case (word == "let" || word == "const") && rest != "" && (isIdent(rest[0]) || rest[0] == '[' || rest[0] == '{'):
			out.WriteString(s[last:e.at])
			out.WriteString("var")
			last = e.end
		case word == "class" && rest != "" && isIdent(rest[0]):
			name := rest[:identEnd(rest, 0)]
			if slices.Contains([]string{"extends", "implements"}, name) {
				return
			}
			out.WriteString(s[last:e.at])
			out.WriteString("var " + name + " = class")
			last = e.end
		}
	})
	out.WriteString(s[last:])
	return out.String()
}

// token is what walk tells of the top level: a ';' or a '\n' (end just past
// it), or as 'w' a word where a statement may start.
type token struct {
	kind    byte
	at, end int
}

// walk reads JavaScript from i, telling the top level's tokens to emit; nested
// reads a template's ${…} and returns past its closing brace, telling none.
// It knows strings, templates, comments and regular expressions well enough
// that a ; or a newline inside one is not the top level's.
func walk(s string, i int, nested bool, emit func(token)) int {
	depth, prev, word, fresh := 0, byte(0), "", false
	top := func() bool { return depth == 0 && !nested }
	for i < len(s) {
		c := s[i]
		next := i + 1
		switch {
		case c == '\n':
			if top() {
				emit(token{kind: '\n', at: i, end: i + 1})
			}
			fresh = true
			i = next
			continue
		case c == ' ' || c == '\t' || c == '\r':
			i = next
			continue
		case strings.HasPrefix(s[i:], "//"):
			i = skipTo(s, i, "\n", false)
			continue
		case strings.HasPrefix(s[i:], "/*"):
			i = skipTo(s, i+2, "*/", true)
			continue
		case c == '"' || c == '\'':
			next, c, word = skipString(s, i), 'a', ""
		case c == '`':
			next, c, word = skipTemplate(s, i), 'a', ""
		case c == '/' && regexMayStart(prev, word):
			next, c, word = skipRegex(s, i), 'a', ""
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 && nested && c == '}' {
				return next
			}
			depth = max(0, depth-1)
		case c == ';':
			if top() {
				emit(token{kind: ';', at: i, end: next})
			}
		case isIdent(c):
			next = identEnd(s, i)
			if top() && (prev == 0 || prev == ';' || prev == '}' || fresh) {
				emit(token{kind: 'w', at: i, end: next})
			}
			word, c = s[i:next], 'a'
		}
		if c != 'a' {
			word = ""
		}
		prev, fresh, i = c, false, next
	}
	return i
}

func isIdent(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func identEnd(s string, i int) int {
	for i < len(s) && isIdent(s[i]) {
		i++
	}
	return i
}

// skipTo is the index of end in s from i, past it when past is set; the end
// of s when it is not there.
func skipTo(s string, i int, end string, past bool) int {
	j := strings.Index(s[i:], end)
	if j < 0 {
		return len(s)
	}
	if past {
		return i + j + len(end)
	}
	return i + j
}

func skipString(s string, i int) int {
	quote := s[i]
	for i++; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote, '\n':
			return i + 1
		}
	}
	return i
}

func skipTemplate(s string, i int) int {
	for i++; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == '`':
			return i + 1
		case strings.HasPrefix(s[i:], "${"):
			i = walk(s, i+2, true, func(token) {}) - 1
		}
	}
	return i
}

func skipRegex(s string, i int) int {
	class := false
	for i++; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\':
			i++
		case c == '[':
			class = true
		case c == ']':
			class = false
		case c == '/' && !class:
			return identEnd(s, i+1)
		case c == '\n':
			return i
		}
	}
	return i
}

// regexKeywords are the words after which a / starts a regular expression.
var regexKeywords = []string{"return", "typeof", "instanceof", "in", "of", "new", "delete", "void", "throw", "case", "do", "else", "yield", "await"}

// regexMayStart is whether a / after prev, the last significant character
// ('a' for a word, a number, a string or a regular expression, word the
// word), starts a regular expression rather than dividing.
func regexMayStart(prev byte, word string) bool {
	switch prev {
	case 'a':
		return slices.Contains(regexKeywords, word)
	case ')', ']', '}':
		return false
	}
	return true
}
