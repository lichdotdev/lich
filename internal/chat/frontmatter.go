package chat

import (
	"strconv"
	"strings"
)

// frontmatter is the top-level keys of a Markdown file's YAML frontmatter,
// each with the lines under it: what a subagent's file is read for, a
// one-line scalar or a block list. shortcut: lich has no YAML library, so a
// folded or multi-line scalar reads as empty; a parser earns its place if a
// panel ever needs one.
type frontmatter map[string][]string

const frontmatterFence = "---"

func parseFrontmatter(text string) frontmatter {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontmatterFence {
		return nil
	}
	out, key := frontmatter{}, ""
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == frontmatterFence {
			return out
		}
		if line != "" && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				key = ""
				continue
			}
			key = strings.TrimSpace(k)
			out[key] = []string{strings.TrimSpace(v)}
			continue
		}
		if key != "" {
			out[key] = append(out[key], strings.TrimSpace(line))
		}
	}
	return nil
}

// scalar is key's one-line value, unquoted; "" when it has none or spans
// lines.
func (f frontmatter) scalar(key string) string {
	v := f[key]
	if len(v) == 0 || v[0] == "|" || v[0] == ">" || strings.HasPrefix(v[0], "|") || strings.HasPrefix(v[0], ">") {
		return ""
	}
	return unquote(v[0])
}

// unquote reads a YAML double-quoted scalar's escapes the way Go reads its
// own, which agree on the ones a one-line description holds.
func unquote(s string) string {
	if u, err := strconv.Unquote(s); err == nil && strings.HasPrefix(s, `"`) {
		return u
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// agentTools is a subagent file's tools: a comma list, a flow list or a
// block list, as Claude Code's docs write them; nil when there are none.
func agentTools(f frontmatter) []string {
	v := f["tools"]
	if len(v) == 0 {
		return nil
	}
	var items []string
	if first := strings.Trim(unquote(v[0]), "[]"); first != "" {
		items = strings.Split(first, ",")
	}
	for _, line := range v[1:] {
		if item, ok := strings.CutPrefix(line, "- "); ok {
			items = append(items, item)
		}
	}
	var tools []string
	for _, t := range items {
		if t = unquote(strings.TrimSpace(t)); t != "" {
			tools = append(tools, t)
		}
	}
	return tools
}
