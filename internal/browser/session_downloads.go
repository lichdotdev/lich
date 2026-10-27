package browser

import (
	"context"
	"errors"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// downloadWait bounds how long a click or a key press waits for a download it
// started to finish before answering with it still in progress: a small file
// is on disk within it, and a large one is not worth holding the tool for.
const downloadWait = 10 * time.Second

// downloadPoll is how often that wait looks.
const downloadPoll = 50 * time.Millisecond

// downloadNotice bounds how long a navigation Chrome aborted waits to hear
// whether it became a download: the two arrive in either order.
const downloadNotice = time.Second

// downloadQuiet is how long after a download begins a click or a key press
// holds its answer. Chrome aborts a navigation the page starts within about a
// second of a download (net::ERR_ABORTED, then it reloads the page): the
// agent's next step failed 10 times in 10 right after an <a download>, and
// none in 50 with this gap (Chromium 155, measured 2026-10-27).
const downloadQuiet = 1500 * time.Millisecond

// download is one of a session's downloads and the state the agent last
// heard it in, "" before it heard of it.
type download struct {
	guid string
	now  api.BrowserDownload
	told string
}

// downloadSink records each step of the page's downloads and tells the
// window when one is complete on disk.
func (s *Service) downloadSink(sess *session) func(guid string, d api.BrowserDownload) {
	return func(guid string, d api.BrowserDownload) {
		d.Owner = sess.owner
		s.mu.Lock()
		completed := sess.noteDownloadLocked(guid, d)
		s.mu.Unlock()
		if completed {
			events.Send(s.emit, api.BrowserDownloadEvent, d)
		}
	}
}

// noteDownloadLocked keeps d as guid's latest step, true when it is the one
// that completed it.
func (sess *session) noteDownloadLocked(guid string, d api.BrowserDownload) bool {
	for _, dl := range sess.downloads {
		if dl.guid == guid {
			was := dl.now.State
			dl.now = d
			return d.State == api.DownloadCompleted && was != api.DownloadCompleted
		}
	}
	sess.downloads = append(sess.downloads, &download{guid: guid, now: d})
	sess.downloadBegun = time.Now()
	return d.State == api.DownloadCompleted
}

// downloadsAfter waits up to wait for the downloads the agent has not heard
// of yet to end, and out the quiet after the last one began, then returns
// what it has not heard.
func (s *Service) downloadsAfter(sess *session, wait time.Duration) []api.BrowserDownload {
	deadline := time.Now().Add(wait)
	for s.freshInProgress(sess) && time.Now().Before(deadline) {
		time.Sleep(downloadPoll)
	}
	s.mu.Lock()
	hold := time.Until(sess.downloadBegun.Add(s.quiet))
	s.mu.Unlock()
	if hold > 0 {
		time.Sleep(hold)
	}
	return s.untold(sess)
}

func (s *Service) freshInProgress(sess *session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, dl := range sess.downloads {
		if dl.told == "" && dl.now.State == api.DownloadInProgress {
			return true
		}
	}
	return false
}

// untold is each download in a state the agent has not heard, which it now
// has. A download it heard end is forgotten.
func (s *Service) untold(sess *session) []api.BrowserDownload {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []api.BrowserDownload
	kept := sess.downloads[:0]
	for _, dl := range sess.downloads {
		if dl.told != dl.now.State {
			d := dl.now
			d.Owner = ""
			out = append(out, d)
			dl.told = d.State
		}
		if dl.now.State == api.DownloadInProgress {
			kept = append(kept, dl)
		}
	}
	sess.downloads = kept
	return out
}

// navigate loads u in sess's page. A URL that answers with a file is a
// download, not a page: Chrome aborts the navigation and the page stays where
// it was, which is a load that worked.
func (s *Service) navigate(ctx context.Context, sess *session, u string) (download bool, err error) {
	before := time.Now()
	err = sess.page.Navigate(ctx, u)
	if errors.Is(err, errNavAborted) && s.downloadSince(sess, before) {
		return true, nil
	}
	return false, err
}

func (s *Service) downloadSince(sess *session, t time.Time) bool {
	deadline := time.Now().Add(downloadNotice)
	for {
		s.mu.Lock()
		// Not After: Windows' clock can give the navigation and the download
		// the same instant.
		began := !sess.downloadBegun.Before(t)
		s.mu.Unlock()
		if began || time.Now().After(deadline) {
			return began
		}
		time.Sleep(downloadPoll)
	}
}
