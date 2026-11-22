package main

import (
	"html"
	"regexp"
	"slices"
	"strings"
)

// A notification envelope lich handed the chat's agent as its user message
// (docs/hooks/notification.md). Its element text is written unescaped, so it is
// not XML: the root's attributes are read off its first line and each known
// child by its tags, its text running to the last closing tag of its name.

const (
	envelopeOpen  = "<lich-notification "
	envelopeClose = "</lich-notification>"
)

// envelopeTags is every child an envelope of version 1 carries.
var envelopeTags = []string{"status", "result", "summary", "error", "by", "request", "prompt"}

var envelopeAttr = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)

type envelope struct {
	kind, topic, from, session, ticket string
	// attrs is every attribute of the root, unescaped, the ones above included.
	attrs map[string]string
	text  map[string]string
	items []envelopeItem
}

// envelopeItem is one <item> of a request: its attributes, unescaped, and its
// text, empty for a self-closing one.
type envelopeItem struct {
	attrs map[string]string
	text  string
}

// parseEnvelopes reads a message that is one envelope or several one blank line
// apart, and is false for any other message, which is drawn as the person's.
func parseEnvelopes(text string) ([]envelope, bool) {
	if !strings.HasPrefix(text, envelopeOpen) || !strings.HasSuffix(text, envelopeClose) {
		return nil, false
	}
	var envs []envelope
	for chunk := range strings.SplitSeq(text, envelopeClose+"\n\n"+envelopeOpen) {
		if !strings.HasPrefix(chunk, envelopeOpen) {
			chunk = envelopeOpen + chunk
		}
		env, ok := parseEnvelope(strings.TrimSuffix(chunk, envelopeClose))
		if !ok {
			return nil, false
		}
		envs = append(envs, env)
	}
	return envs, true
}

func parseEnvelope(chunk string) (envelope, bool) {
	head, body, ok := strings.Cut(chunk, ">\n")
	if !ok {
		return envelope{}, false
	}
	env := envelope{text: map[string]string{}, attrs: attrsOf(head)}
	for _, m := range envelopeAttr.FindAllStringSubmatch(head, -1) {
		value := html.UnescapeString(m[2])
		switch m[1] {
		case "kind":
			env.kind = value
		case "topic":
			env.topic = value
		case "from":
			env.from = value
		case "session":
			env.session = value
		case "ticket":
			env.ticket = value
		}
	}
	body = "\n" + body
	env.items = parseItems(body)
	type child struct {
		tag   string
		start int
	}
	var children []child
	for _, tag := range envelopeTags {
		if start := strings.Index(body, "\n<"+tag+">"); start >= 0 {
			children = append(children, child{tag, start})
		}
	}
	slices.SortFunc(children, func(a, b child) int { return a.start - b.start })
	for i, c := range children {
		stop := len(body)
		if i+1 < len(children) {
			stop = children[i+1].start
		}
		from := c.start + len(c.tag) + 3
		if end := strings.LastIndex(body[from:stop], "</"+c.tag+">"); end >= 0 {
			env.text[c.tag] = body[from : from+end]
		}
	}
	return env, env.kind != ""
}

func attrsOf(head string) map[string]string {
	attrs := map[string]string{}
	for _, m := range envelopeAttr.FindAllStringSubmatch(head, -1) {
		attrs[m[1]] = html.UnescapeString(m[2])
	}
	return attrs
}

// parseItems reads the <item> rows between a request's summary and its
// prompt. An attribute value is escaped, so the first > ends an item's tag;
// its text runs to the last </item> before the next row.
func parseItems(body string) []envelopeItem {
	start := strings.Index(body, "</summary>")
	if start < 0 {
		return nil
	}
	end := strings.Index(body[start:], "\n<prompt>")
	if end < 0 {
		end = len(body) - start
	}
	var items []envelopeItem
	for i, chunk := range strings.Split(body[start:start+end], "\n<item") {
		head, rest, ok := strings.Cut(chunk, ">")
		if i == 0 || !ok {
			continue
		}
		it := envelopeItem{attrs: attrsOf(head)}
		if !strings.HasSuffix(head, "/") {
			if cut := strings.LastIndex(rest, "</item>"); cut >= 0 {
				it.text = rest[:cut]
			}
		}
		items = append(items, it)
	}
	return items
}
