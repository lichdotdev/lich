package ghrelease

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// GitHub serves a different JSON shape without the Accept header and rejects
// an unidentified client outright, so every read lich makes carries both.
func TestGetCarriesTheGitHubHeaders(t *testing.T) {
	var accept, agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept, agent = r.Header.Get("Accept"), r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	resp, err := Get(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if accept != "application/vnd.github+json" {
		t.Errorf("Accept = %q", accept)
	}
	if agent != "lich" {
		t.Errorf("User-Agent = %q", agent)
	}
}

// serveBody starts a test server returning status/body.
func serveBody(t *testing.T, status int, body string) (*http.Client, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.Client(), srv.URL
}

func TestReleasesCarryTheNotes(t *testing.T) {
	body := `[
		{"tag_name":"v0.14.0-rc.1","prerelease":true,"body":"rc"},
		{"tag_name":"v0.13.1","body":"> [!CAUTION]\n> **Everyone: update.**"},
		{"tag_name":"v0.13.0","draft":true},
		{"tag_name":"0.12.0"}
	]`
	client, url := serveBody(t, http.StatusOK, body)
	got := Releases(client, url)
	want := []Release{{Tag: "0.13.1", Body: "> [!CAUTION]\n> **Everyone: update.**"}, {Tag: "0.12.0"}}
	if !slices.Equal(got, want) {
		t.Fatalf("Releases() = %+v, want %+v", got, want)
	}
}

func TestReleasesNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := srv.Client()
	url := srv.URL
	srv.Close()

	if got := Releases(client, url); got != nil {
		t.Fatalf("Releases() = %v, want nil for an unreachable server", got)
	}
}

func TestReleasesCapBody(t *testing.T) {
	// The list is valid but padded past ListLimit: the cap truncates the body
	// mid-JSON, so a parse failure here proves the read is bounded.
	body := `[{"tag_name":"v9.9.9","body":"` + strings.Repeat("x", ListLimit+1) + `"}]`
	client, url := serveBody(t, http.StatusOK, body)
	if got := Releases(client, url); got != nil {
		t.Fatalf("Releases() = %v, want nil: body read past ListLimit", got)
	}
}

// A page of 100 real releases is over two MiB, past the cap a small metadata
// read is held to.
func TestReleasesReadAListLargerThanBodyLimit(t *testing.T) {
	body := `[{"tag_name":"v9.9.9","body":"` + strings.Repeat("x", BodyLimit*2) + `"}]`
	client, url := serveBody(t, http.StatusOK, body)
	if got := Releases(client, url); len(got) != 1 || got[0].Tag != "9.9.9" {
		t.Fatalf("Releases() = %d releases, want the one in a list past BodyLimit", len(got))
	}
}

func TestReleasesBadURL(t *testing.T) {
	if got := Releases(&http.Client{}, "://not a url"); got != nil {
		t.Fatalf("Releases() = %v, want nil for an unbuildable request", got)
	}
}

func TestReleaseTags(t *testing.T) {
	body := `[
		{"tag_name":"v0.14.0-rc.1","prerelease":true},
		{"tag_name":"v0.13.1"},
		{"tag_name":"v0.13.0","draft":true},
		{"tag_name":""},
		{"tag_name":"0.12.0"}
	]`
	client, url := serveBody(t, http.StatusOK, body)
	got := strings.Join(ReleaseTags(client, url), ",")
	if got != "0.13.1,0.12.0" {
		t.Fatalf("ReleaseTags() = %q, want published releases only, v stripped", got)
	}
}

func TestReleaseTagsFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"not a list", http.StatusOK, `{"tag_name":"v0.2.0"}`},
		{"not found", http.StatusNotFound, `[]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, url := serveBody(t, tc.status, tc.body)
			if got := ReleaseTags(client, url); got != nil {
				t.Fatalf("ReleaseTags() = %v, want nil", got)
			}
		})
	}
}
