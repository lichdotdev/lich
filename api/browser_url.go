package api

import (
	"path/filepath"
	"strings"
)

// NewTabPrefix is the path every session's new-tab page is served under, on a
// loopback listener of its own (internal/preview), followed by the session's
// id.
const NewTabPrefix = "/new-tab/"

// TypedURL is what a browser's address field loads for text: a URL or an
// absolute path as typed, and a bare host (localhost:3000, example.com) over
// http for a local one and https for any other. The backend serves a path from
// the session's checkout (internal/preview).
func TypedURL(text string) string {
	switch {
	case text == "":
		return "about:blank"
	case hasScheme(text), filepath.IsAbs(text):
		return text
	case isLocalHost(text):
		return "http://" + text
	}
	return "https://" + text
}

func hasScheme(text string) bool {
	for i, c := range text {
		switch {
		case c == ':':
			return i > 0 && len(text) > i+2 && text[i+1:i+3] == "//"
		case !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.')):
			return false
		}
	}
	return false
}

func isLocalHost(text string) bool {
	host := text
	for i, c := range text {
		if c == ':' || c == '/' {
			host = text[:i]
			break
		}
	}
	return host == "localhost" || host == "127.0.0.1" || host == "[::1]" || strings.HasSuffix(host, ".localhost")
}
