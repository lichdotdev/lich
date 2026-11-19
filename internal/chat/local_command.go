package chat

import (
	"regexp"
	"strings"
)

// A command Claude Code runs itself (/model, /clear, ...) is written into the
// transcript as two user lines of XML-tagged text, not as something the person
// wrote: the command, then what it printed (measured on 2.1.327).
var (
	commandTag = map[string]*regexp.Regexp{
		"name":    regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`),
		"message": regexp.MustCompile(`(?s)<command-message>(.*?)</command-message>`),
		"args":    regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`),
	}
	commandStdout = regexp.MustCompile(`(?s)^\s*<local-command-stdout>(.*?)</local-command-stdout>\s*$`)
)

// claudeNoResponse is the answer Claude Code writes after a local command, to
// have the turn end with a message: not something the agent said.
const claudeNoResponse = "No response requested."

// parseCommand reads a command line: its name with its arguments, "/model
// haiku". The text must be nothing but the command's tags, and name one; any
// other text, with a tag or without, is not a command.
func parseCommand(text string) (string, bool) {
	rest := text
	got := map[string]string{}
	for tag, re := range commandTag {
		m := re.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		got[tag] = strings.TrimSpace(m[1])
		rest = strings.Replace(rest, m[0], "", 1)
	}
	if got["name"] == "" || strings.TrimSpace(rest) != "" {
		return "", false
	}
	return strings.TrimSpace(got["name"] + " " + got["args"]), true
}

// parseStdout reads what a local command printed.
func parseStdout(text string) (string, bool) {
	m := commandStdout.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// localCommand turns a command line, or the output that follows one, into a
// notice of one muted line, "/model haiku · Set model to haiku", and reports
// whether text was one. Anything else is left to the caller.
func localCommand(c *chatSession, text string) bool {
	if cmd, ok := parseCommand(text); ok {
		c.conv.commandNotice(cmd)
		return true
	}
	if out, ok := parseStdout(text); ok {
		c.conv.commandOutput(out)
		return true
	}
	return false
}
