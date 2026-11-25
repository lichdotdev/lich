package main

import (
	"testing"

	"github.com/lichdotdev/lich/browserwire"
)

func TestARequestsTypeIsGuessedFromWhatItAsksFor(t *testing.T) {
	for _, tc := range []struct{ uri, accept, want string }{
		{"https://a.test/style", "text/css,*/*;q=0.1", browserwire.RequestStylesheet},
		{"https://a.test/pic", "image/webp,image/avif,*/*", browserwire.RequestImage},
		{"https://a.test/frame", "text/html,application/xhtml+xml", browserwire.RequestDocument},
		{"https://a.test/app.js?v=2", "*/*", browserwire.RequestScript},
		{"https://a.test/f.woff2", "*/*", browserwire.RequestFont},
		{"https://a.test/clip.webm", "*/*", browserwire.RequestMedia},
		{"https://a.test/api/items", "*/*", browserwire.RequestFetch},
		{"https://a.test/api/items", "application/json", browserwire.RequestFetch},
	} {
		if got := requestType(tc.uri, tc.accept); got != tc.want {
			t.Errorf("requestType(%q, %q) = %s, want %s", tc.uri, tc.accept, got, tc.want)
		}
	}
}
