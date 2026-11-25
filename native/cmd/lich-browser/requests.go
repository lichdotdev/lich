package main

import (
	"net/url"
	"path"
	"strings"

	"github.com/lichdotdev/lich/browserwire"
)

// requestType guesses a subresource request's type from its URL and Accept
// header: WebKitGTK tells the UI process neither who made a request nor for
// what. Settle waits on fetch and XHR alone, so what the guess cannot place
// counts as a fetch: waited on, never skipped.
//
// shortcut: fetch and XHR are told apart by nothing WebKit hands over, so both
// are "fetch"; a page script tagging them would tell them apart.
func requestType(uri, accept string) string {
	switch {
	case strings.HasPrefix(accept, "text/html"):
		return browserwire.RequestDocument
	case strings.HasPrefix(accept, "text/css"):
		return browserwire.RequestStylesheet
	case strings.HasPrefix(accept, "image/"):
		return browserwire.RequestImage
	case strings.HasPrefix(accept, "video/"), strings.HasPrefix(accept, "audio/"):
		return browserwire.RequestMedia
	}
	u, err := url.Parse(uri)
	if err != nil {
		return browserwire.RequestFetch
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".js", ".mjs":
		return browserwire.RequestScript
	case ".css":
		return browserwire.RequestStylesheet
	case ".woff", ".woff2", ".ttf", ".otf":
		return browserwire.RequestFont
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg", ".ico":
		return browserwire.RequestImage
	case ".mp4", ".webm", ".mp3", ".ogg", ".m4a", ".wav":
		return browserwire.RequestMedia
	}
	return browserwire.RequestFetch
}
