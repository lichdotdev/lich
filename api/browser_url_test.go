package api

import (
	"path/filepath"
	"testing"
)

func TestTypedURL(t *testing.T) {
	page := filepath.Join(t.TempDir(), "index.html")
	for in, want := range map[string]string{
		"":                       "about:blank",
		"localhost:3000/a":       "http://localhost:3000/a",
		"app.localhost":          "http://app.localhost",
		"127.0.0.1:8080":         "http://127.0.0.1:8080",
		"example.com":            "https://example.com",
		"http://example.com/x":   "http://example.com/x",
		"example.com:8443/login": "https://example.com:8443/login",
		"file:///tmp/a.html":     "file:///tmp/a.html",
		"1x://odd":               "https://1x://odd",
		page:                     page,
	} {
		if got := TypedURL(in); got != want {
			t.Errorf("TypedURL(%q) = %q, want %q", in, got, want)
		}
	}
}
