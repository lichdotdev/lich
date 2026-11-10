package appupdate

import (
	"fmt"
	"time"

	"github.com/lichdotdev/lich/internal/semver"
)

// staleAfter is how old the last lookup may be before RequiredUpdate runs one
// itself: twice the window's hourly poll, so a lich with its window open never
// spends a second request of GitHub's 60 an hour, and one running without a
// window still learns of a required release.
const staleAfter = 2 * time.Hour

// RequiredUpdate is the error a new session is refused with while a release
// that marks itself required is pending, nil otherwise. It answers from the
// last lookup, whoever asked for it. A lookup that failed reports nothing
// required, as it reports no update, and a build that is not a release is
// never asked to update, so it looks nothing up.
func (s *Service) RequiredUpdate() error {
	if !semver.IsRelease(s.version) {
		return nil
	}
	s.seen.Lock()
	st, at := s.last, s.checkedAt
	s.seen.Unlock()
	if time.Since(at) > staleAfter {
		st = s.status()
	}
	if !st.Required {
		return nil
	}
	return requiredError(st)
}

// requiredError names the release and the way this install updates, so the
// command line and an agent's tool result are enough to act on.
func requiredError(st Status) error {
	how := "download it from " + st.ReleaseURL
	switch {
	case st.CanSelfApply:
		how = "install it from the update prompt in the lich window, or " + how
	case st.InstallCommand != "":
		how = "run `" + st.InstallCommand + "` in a lich shell session, or " + how
	}
	return fmt.Errorf(
		"lich %s is a required security update, and this is %s: no new agent session opens until it is "+
			"installed. Running sessions are untouched, and a shell session still opens. To update, %s",
		st.LatestVersion, st.CurrentVersion, how,
	)
}
