package webview

import "sync"

// Load is a main-frame load event of a page: Loading from the load's start
// until it finished, failed or was cancelled, and URL where a load the page
// started itself (a link, a form, back or forward) landed after any
// redirect; empty for a load the window asked for.
type Load struct {
	Loading bool
	URL     string
}

// The steps of a main-frame load, WebKitLoadEvent's values: a failed or
// cancelled load ends with loadFinished too.
const (
	loadStarted    = 0
	loadRedirected = 1
	loadCommitted  = 2
	loadFinished   = 3
)

// loads turns a page's load steps into Loads, telling the window's own loads
// from the page's.
type loads struct {
	mu sync.Mutex
	// asked is the URL the window last asked for, until its load starts.
	asked string
	// own is whether the load under way is the window's.
	own bool
	// at is the URL the page's last load landed on.
	at string
}

// ask records that the window asked the page for url.
func (l *loads) ask(url string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked, l.at = url, url
}

// url is where the page is, as far as its loads say.
func (l *loads) url() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.at
}

// step takes one load step, at uri, and says what the window is told, if
// anything.
func (l *loads) step(kind int, uri string) (Load, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch kind {
	case loadStarted:
		l.own = l.asked != "" && uri == l.asked
		l.asked = ""
		return Load{Loading: true}, true
	case loadCommitted:
		l.at = uri
		if l.own {
			return Load{}, false
		}
		return Load{Loading: true, URL: uri}, true
	case loadFinished:
		return Load{}, true
	}
	return Load{}, false
}
