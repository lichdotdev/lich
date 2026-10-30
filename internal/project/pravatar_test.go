package project

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestPullRequestAvatar(t *testing.T) {
	t.Parallel()
	const avatar = "https://avatars.githubusercontent.com/u/1?s=96&v=4"

	t.Run("reads the picture through gh, in the checkout", func(t *testing.T) {
		gh := &fakeGH{out: []byte("\x89PNG")}
		got, err := withGH(gh).PullRequestAvatar(t.Context(), api.AvatarOptions{Path: "/repo", URL: avatar})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("\x89PNG")) {
			t.Errorf("bytes = %q, want gh's answer", got)
		}
		if gh.dir != "/repo" || !slices.Equal(gh.args, []string{"api", avatar}) {
			t.Errorf("gh ran %v in %q", gh.args, gh.dir)
		}
	})

	t.Run("an address that is not an avatar never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		for _, raw := range []string{
			"repos/{owner}/{repo}/pulls",
			"https://api.github.com/user",
			"http://avatars.githubusercontent.com/u/1",
			"https://x@avatars.githubusercontent.com/u/1",
		} {
			if _, err := withGH(gh).PullRequestAvatar(t.Context(), api.AvatarOptions{URL: raw}); err == nil {
				t.Errorf("%q: want a refusal", raw)
			}
		}
		if gh.calls != 0 {
			t.Errorf("gh ran %d times, want none", gh.calls)
		}
	})

	t.Run("gh's failure is returned", func(t *testing.T) {
		gh := &fakeGH{err: testError("HTTP 404")}
		if _, err := withGH(gh).PullRequestAvatar(t.Context(), api.AvatarOptions{URL: avatar}); !errors.Is(err, testError("HTTP 404")) {
			t.Errorf("err = %v, want gh's", err)
		}
	})

	t.Run("an answer past the cap is refused", func(t *testing.T) {
		gh := &fakeGH{out: make([]byte, maxAvatarBytes+1)}
		if _, err := withGH(gh).PullRequestAvatar(t.Context(), api.AvatarOptions{URL: avatar}); err == nil {
			t.Error("want a refusal")
		}
	})
}

func TestIsAvatarURL(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"https://avatars.githubusercontent.com/u/1?s=96": true,
		"https://avatars.ghe.example.com/u/1?s=96":       true,
		"https://ghe.example.com/avatars/u/1?s=96":       true,
		"https://ghe.example.com/api/v3/user":            false,
		"https://github.com/omartelo.png":                false,
		"file:///etc/passwd":                             false,
		"":                                               false,
	} {
		if got := isAvatarURL(raw); got != want {
			t.Errorf("isAvatarURL(%q) = %v, want %v", raw, got, want)
		}
	}
}
