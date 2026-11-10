package browser

import (
	"time"
)

// startupGrace is how long after its launch a browser may still reload the
// page over a download. Chrome holds back a download the page asks for in the
// browser's first seconds and reloads the page as it starts it; nothing shows
// before the reload starts, no event and no request (Chromium 157, Helium
// 0.21.1.1, measured 2026-11-08). 35 of 39 downloads asked for 0.7 to 3.3
// seconds after the launch (<a download>, an attachment link, location.href)
// brought a reload 4.3 to 5.3 seconds after it, 7.4 once; the other four began
// without one, and a download asked for 5 seconds or later began at once with
// none. Whether Chromium 155's reload 1.46 to 1.70 seconds after a download
// began, the hold this replaced, was the same young-browser effect is not
// known. An action that follows another inside
// this window waits for it to pass, or for a reload to land, so the reload
// cannot wipe it.
const startupGrace = 6 * time.Second

// reloadWait is how long an action waits on a reload in flight past
// startupGrace: a reload takes about 250 ms from its start to its document.
const reloadWait = 2 * time.Second

// holdForStartup returns once an action on sess can no longer be wiped by the
// reload a download the agent's last click or key press asked for brings. A
// download that is pending shows nothing, so only an action that follows one
// whose answer showed the page as it was waits: a page that changed, or
// opened a dialog, was no download request.
func (s *Service) holdForStartup(sess *session) {
	s.mu.Lock()
	still, seen := sess.still, sess.reloads
	sess.beforeURL, sess.beforeTitle = sess.url, sess.title
	s.mu.Unlock()
	if !still {
		return
	}
	young := sess.launched.Add(s.startup)
	stop := young.Add(s.reloadWait)
	for {
		s.mu.Lock()
		reloading, landed := sess.reloading, sess.reloads > seen
		s.mu.Unlock()
		now := time.Now()
		if !reloading && (landed || !now.Before(young)) || !now.Before(stop) {
			return
		}
		time.Sleep(downloadPoll)
	}
}

// reloadSink records when the page starts a reload and when its document is
// ready.
func (s *Service) reloadSink(sess *session) func(started bool) {
	return func(started bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case started:
			sess.reloading = true
		case sess.reloading:
			sess.reloading = false
			sess.reloads++
		}
	}
}
