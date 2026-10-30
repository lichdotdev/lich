package project

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// maxAvatarBytes caps one avatar. GitHub answers a 96px one in a few KB; the
// cap keeps a wrong address from being held whole in the window.
const maxAvatarBytes = 1 << 20

// PullRequestAvatar reads the picture at opts.URL, an author's Avatar in a
// conversation. It goes through gh so an enterprise instance that serves
// avatars only to signed-in users gets the project's account like every other
// read; github.com's avatar host is not one gh holds a token for, so nothing
// is sent there.
func (s *Service) PullRequestAvatar(_ context.Context, opts api.AvatarOptions) ([]byte, error) {
	if !isAvatarURL(opts.URL) {
		return nil, fmt.Errorf("%q is not a GitHub avatar address", opts.URL)
	}
	out, err := s.gh(prReadTimeout, opts.Path, "api", opts.URL)
	if err != nil {
		return nil, err
	}
	if len(out) > maxAvatarBytes {
		return nil, fmt.Errorf("the avatar at %s is above %d MB", opts.URL, maxAvatarBytes>>20)
	}
	return out, nil
}

// isAvatarURL holds the address to the shapes GitHub serves avatars at:
// avatars.githubusercontent.com, an enterprise instance's avatars subdomain,
// or its /avatars/ path when subdomain isolation is off. Anything else would
// make this an authenticated `gh api` GET of whatever the caller named.
func isAvatarURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return false
	}
	return strings.HasPrefix(u.Host, "avatars.") || strings.HasPrefix(u.Path, "/avatars/")
}
