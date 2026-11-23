package markdown

import (
	"bytes"
	"strings"
)

// htmlElements is the HTML a Markdown body is written with: GitHub's PR
// bodies, READMEs. A tag of any other name is text.
var htmlElements = map[string]bool{
	"a": true, "abbr": true, "b": true, "blockquote": true, "body": true, "br": true, "code": true,
	"dd": true, "del": true, "details": true, "div": true, "dl": true, "dt": true, "em": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "head": true, "hr": true,
	"html": true, "i": true, "img": true, "input": true, "ins": true, "kbd": true, "li": true,
	"mark": true, "ol": true, "p": true, "picture": true, "pre": true, "q": true, "s": true,
	"samp": true, "small": true, "source": true, "span": true, "strike": true, "strong": true,
	"sub": true, "summary": true, "sup": true, "table": true, "tbody": true, "td": true,
	"tfoot": true, "th": true, "thead": true, "tr": true, "tt": true, "u": true, "ul": true,
	"var": true, "video": true, "wbr": true,
}

// isHTMLTag is whether raw opens on HTML: a comment, a declaration or a tag
// HTML has.
func isHTMLTag(raw []byte) bool {
	raw = bytes.TrimLeft(raw, " \t")
	if !bytes.HasPrefix(raw, []byte("<")) {
		return false
	}
	rest := strings.TrimPrefix(string(raw[1:]), "/")
	if strings.HasPrefix(rest, "!") || strings.HasPrefix(rest, "?") {
		return true
	}
	end := strings.IndexFunc(rest, func(r rune) bool { return r == ' ' || r == '>' || r == '/' || r == '\t' || r == '\n' })
	if end < 0 {
		end = len(rest)
	}
	return htmlElements[strings.ToLower(rest[:end])]
}
