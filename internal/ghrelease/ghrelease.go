// Package ghrelease reads the published releases of a GitHub repository —
// the one piece lich's two update checks (the app in internal/appupdate and the
// lich plugin in internal/agentplugin) share. It reports release tags as bare
// versions and never errors: a failed lookup yields nothing, which every caller
// treats as "no release known" so a network blip never blocks or breaks startup.
package ghrelease

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// BodyLimit caps a small metadata read; a release's checksums.txt is a few
// KiB, so anything larger is malformed or hostile.
const BodyLimit = 1 << 20

// ListLimit caps a read of the releases list. Each entry carries its notes and
// the metadata of every asset: a page of 100 lich releases measured 2.2 MiB in
// November 2026, so the cap leaves room for the notes to grow.
const ListLimit = 8 << 20

// Release is one published release: its tag as a bare version and its notes.
type Release struct {
	Tag  string
	Body string
}

// Get issues a GET carrying lich's identifying headers. Every GitHub read lich
// makes goes through here so the request shape is stated once.
func Get(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lich")
	return client.Do(req)
}

// Releases GETs a GitHub releases list URL and returns its published releases,
// newest first as GitHub lists them. Drafts and pre-releases are left out:
// neither is something lich installs. Any failure yields nil.
func Releases(client *http.Client, url string) []Release {
	data := read(client, url, ListLimit)
	if data == nil {
		return nil
	}
	var docs []struct {
		TagName    string `json:"tag_name"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil
	}
	releases := make([]Release, 0, len(docs))
	for _, d := range docs {
		if !d.Draft && !d.Prerelease && d.TagName != "" {
			releases = append(releases, Release{Tag: strings.TrimPrefix(d.TagName, "v"), Body: d.Body})
		}
	}
	return releases
}

// ReleaseTags is the tags of Releases, nil on any failure.
func ReleaseTags(client *http.Client, url string) []string {
	releases := Releases(client, url)
	if releases == nil {
		return nil
	}
	tags := make([]string, len(releases))
	for i, r := range releases {
		tags[i] = r.Tag
	}
	return tags
}

// read GETs url and returns its body, capped at limit, or nil on any failure
// or non-200 answer.
func read(client *http.Client, url string, limit int64) []byte {
	resp, err := Get(client, url)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil
	}
	return data
}
