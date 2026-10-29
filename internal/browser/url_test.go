package browser

import "testing"

func TestAllowURL(t *testing.T) {
	cases := []struct {
		in, want, err string
	}{
		{"", "about:blank", ""},
		{"about:blank", "about:blank", ""},
		{"  ABOUT:BLANK  ", "about:blank", ""},
		{"https://example.com/path", "https://example.com/path", ""},
		{"http://localhost:3000", "http://localhost:3000", ""},
		{"file:///etc/passwd", "", `blocked url scheme "file"`},
		{"javascript:alert(1)", "", `blocked url scheme "javascript"`},
		{"data:text/html,hi", "", `blocked url scheme "data"`},
		{"chrome://settings", "", `blocked url scheme "chrome"`},
		{"example.com", "", `url "example.com" is missing a scheme`},
		{"https://", "", `url "https://" has no host`},
	}
	for _, c := range cases {
		got, err := allowURL(c.in)
		if c.err != "" {
			if err == nil || err.Error() != c.err {
				t.Errorf("allowURL(%q) err = %v, want %s", c.in, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("allowURL(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("allowURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveURLHandsALocalFileToTheOwnersCheckout(t *testing.T) {
	s := &Service{}
	if _, err := s.resolveURL("s1", "file:///srv/index.html"); err == nil || err.Error() != `blocked url scheme "file"` {
		t.Fatalf("unset local: err = %v, want the file scheme blocked", err)
	}
	var gotOwner, gotRaw string
	s.SetLocal(func(raw string) bool { return raw == "file:///srv/index.html" }, func(owner, raw string) (string, error) {
		gotOwner, gotRaw = owner, raw
		return "http://127.0.0.1:25001/index.html", nil
	})
	u, err := s.resolveURL("s1", "file:///srv/index.html")
	if err != nil || u != "http://127.0.0.1:25001/index.html" || gotOwner != "s1" || gotRaw != "file:///srv/index.html" {
		t.Fatalf("resolveURL = %q, %v; local saw %q %q", u, err, gotOwner, gotRaw)
	}
	if u, err := s.resolveURL("s1", "https://example.com"); err != nil || u != "https://example.com" {
		t.Fatalf("resolveURL(https) = %q, %v", u, err)
	}
}

func TestNavigateRefusesABlockedURLBeforeLookingForTheBrowser(t *testing.T) {
	s := &Service{}
	if _, err := s.Navigate("nobody", "javascript:alert(1)"); err == nil || err.Error() != `blocked url scheme "javascript"` {
		t.Fatalf("Navigate err = %v, want the scheme blocked", err)
	}
	if err := s.Nav("nobody", "go", "data:text/html,hi"); err == nil || err.Error() != `blocked url scheme "data"` {
		t.Fatalf("Nav err = %v, want the scheme blocked", err)
	}
}
