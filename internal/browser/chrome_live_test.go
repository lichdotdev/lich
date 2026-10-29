package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/chromium"
)

func liveChrome(t *testing.T) (*chromePage, *httptest.Server) {
	t.Helper()
	if _, err := chromium.FindBrowser(exec.LookPath); err != nil {
		t.Skip("no Chromium-family browser on this machine")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<title>Probe</title>
<form onsubmit="event.preventDefault(); document.body.dataset.sent = this.q.value">
<input id="f" name="q" style="position:absolute;left:10px;top:10px;width:200px;height:30px"
 oninput="document.body.dataset.seen = this.value"></form>`)
	}))
	t.Cleanup(srv.Close)
	p, err := chromeLaunch(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	return p.(*chromePage), srv
}

func TestChromeClearReachesTheFieldsListener(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	field := Target{Selector: "#f"}
	if err := p.Type(ctx, "abc", false, field); err != nil {
		t.Fatal(err)
	}
	if err := p.Type(ctx, "", true, field); err != nil {
		t.Fatal(err)
	}
	var seen string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.seen`, &seen)); err != nil {
		t.Fatal(err)
	}
	if seen != "" {
		t.Fatalf("the page's input listener last saw %q; clearing fired no input event", seen)
	}
}

func TestChromeCookiesKeepASessionCookie(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	in := []*network.CookieParam{{Name: "sid", Value: "1", URL: srv.URL}}
	if err := p.SetCookies(ctx, in); err != nil {
		t.Fatal(err)
	}
	got, err := p.Cookies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.Name == "sid" {
			if c.Value != "1" || c.Expires != nil {
				t.Fatalf("sid read back as %+v; a session cookie must stay one", c)
			}
			return
		}
	}
	t.Fatalf("sid missing from %d cookies", len(got))
}

func TestPromoteRelaunchesOnTheProfileWithItsCookies(t *testing.T) {
	_, srv := liveChrome(t)
	// headless both times: what is under test is the relaunch on the same
	// profile, and a headed window needs a display the suite may not have.
	launch := func(dir string, _ bool) (Page, error) { return chromeLaunch(dir, false) }
	s := newService(launch, func(string, string) (string, error) { return t.TempDir(), nil }, time.Now, idleAfter)
	t.Cleanup(func() {
		if err := s.Cleanup(); err != nil {
			t.Error(err)
		}
	})
	if _, err := s.Open("s", srv.URL); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	in := []*network.CookieParam{{Name: "sid", Value: "1", URL: srv.URL}}
	if err := s.byOwner["s"].page.SetCookies(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenVisible("s"); err != nil {
		t.Fatal(err)
	}
	got, err := s.byOwner["s"].page.Cookies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.Name == "sid" && c.Value == "1" {
			return
		}
	}
	t.Fatalf("session cookie lost in the promotion: %d cookies", len(got))
}

func TestCookieParamGivesAPersistentCookieItsExpiry(t *testing.T) {
	when := time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)
	got := cookieParam(&network.Cookie{Name: "k", Expires: float64(when.Unix())})
	if got.Expires == nil || !got.Expires.Time().Equal(when) {
		t.Fatalf("expires = %v, want %v", got.Expires, when)
	}
}

func TestChromeScreencastSendsFramesAndPageInfo(t *testing.T) {
	p, srv := liveChrome(t)
	frames := make(chan string, 4)
	titles := make(chan string, 4)
	p.Listen(func(data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(_, title string) {
		select {
		case titles <- title:
		default:
		}
	}, func(string, api.BrowserDownload) {}, func(string) {}, func(*api.BrowserDialog) {})
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Screencast(ctx, Viewport{W: 400, H: 300, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-frames:
		img, err := base64.StdEncoding.DecodeString(data)
		if err != nil || len(img) < 2 || img[0] != 0xFF || img[1] != 0xD8 {
			t.Fatalf("frame is not a JPEG (err %v, %d bytes)", err, len(img))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no screencast frame")
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case title := <-titles:
			if title == "Probe" {
				return
			}
		case <-deadline:
			t.Fatal("no page info with the document's title")
		}
	}
}

func TestChromeScreencastDrawsAHiDPIPaneAtTheFrameScale(t *testing.T) {
	p, srv := liveChrome(t)
	frames := make(chan string, 1)
	p.Listen(func(data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {}, func(*api.BrowserDialog) {})
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Screencast(ctx, Viewport{W: 400, H: 300, Scale: 2}); err != nil {
		t.Fatal(err)
	}
	// The first frames can predate the layout at the pane's size.
	deadline := time.After(10 * time.Second)
	last := "none"
	for {
		select {
		case data := <-frames:
			raw, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width == 600 && cfg.Height == 450 {
				return
			}
			last = fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
		case <-deadline:
			t.Fatalf("last frame %s, want the 2x pane drawn at 1.5x, 600x450", last)
		}
	}
}

func TestChromeWindowInputTypesAndSubmits(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"press", "release"} {
		if err := p.Mouse(ctx, api.BrowserMouseOptions{Type: kind, X: 50, Y: 25, Button: "left", Clicks: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.InsertText(ctx, "ação"); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"down", "up"} {
		if err := p.Key(ctx, api.BrowserKeyOptions{Type: kind, Key: "Enter", Code: "Enter", KeyCode: 13}); err != nil {
			t.Fatal(err)
		}
	}
	var sent string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.sent || ""`, &sent)); err != nil {
		t.Fatal(err)
	}
	if sent != "ação" {
		t.Fatalf("form sent %q, want the clicked field's typed text", sent)
	}
}

// newTabRig is a launched page that listens, on a server whose every page
// links to /elsewhere in a new tab, at start with its frames on. The link
// leaves the new tab its opener on /opener, as window.open does.
type newTabRig struct {
	p       *chromePage
	srv     *httptest.Server
	urls    chan string
	dialogs chan *api.BrowserDialog
}

func startNewTabRig(t *testing.T, ctx context.Context, start string) newTabRig {
	t.Helper()
	pg, err := chromeLaunch(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pg.Close(); err != nil {
			t.Error(err)
		}
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		rel := ""
		if r.URL.Path == "/opener" {
			rel = "opener"
		}
		fmt.Fprintf(w, `<title>%s</title><a href="/elsewhere" target="_blank" rel="%s" style="position:absolute;left:0;top:0;font-size:40px">away</a>`, r.URL.Path, rel)
	}))
	t.Cleanup(srv.Close)
	r := newTabRig{p: pg.(*chromePage), srv: srv, urls: make(chan string, 8), dialogs: make(chan *api.BrowserDialog, 8)}
	r.p.Listen(func(string) {}, func(url, _ string) { r.urls <- url }, func(string, api.BrowserDownload) {}, func(string) {},
		func(d *api.BrowserDialog) { r.dialogs <- d })
	if err := r.p.Navigate(ctx, srv.URL+start); err != nil {
		t.Fatal(err)
	}
	if err := r.p.Screencast(ctx, Viewport{W: 400, H: 300, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	return r
}

// followLink clicks the link and waits for its tab to come to the front.
func (r newTabRig) followLink(t *testing.T, ctx context.Context) {
	t.Helper()
	for _, kind := range []string{"press", "release"} {
		if err := r.p.Mouse(ctx, api.BrowserMouseOptions{Type: kind, X: 20, Y: 20, Button: "left", Clicks: 1}); err != nil {
			t.Fatal(err)
		}
	}
	r.waitURL(t, r.srv.URL+"/elsewhere")
}

func (r newTabRig) waitURL(t *testing.T, want string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case u := <-r.urls:
			if u == want {
				return
			}
		case <-deadline:
			t.Fatalf("the page never showed %s", want)
		}
	}
}

func (r newTabRig) eval(t *testing.T, ctx context.Context, script string, out any) {
	t.Helper()
	if err := r.p.run(ctx, chromedp.Evaluate(script, out)); err != nil {
		t.Fatal(err)
	}
}

func TestChromeLinkToANewTabComesToTheFrontAndKeepsTheOneUnderIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	r := startNewTabRig(t, ctx, "/")
	r.eval(t, ctx, `window.loggedIn = true`, nil)
	r.followLink(t, ctx)
	quick, stop := context.WithTimeout(ctx, inputWait)
	defer stop()
	if err := r.p.Mouse(quick, api.BrowserMouseOptions{Type: "move", X: 50, Y: 50}); err != nil {
		t.Fatalf("the new tab takes no input: %v", err)
	}
	var href string
	r.eval(t, ctx, `location.href`, &href)
	if href != r.srv.URL+"/elsewhere" {
		t.Fatalf("the page drives %q, want the new tab", href)
	}
	// Closed after the call answers: a tab closing on it fails the call.
	r.eval(t, ctx, `setTimeout(() => window.close(), 0)`, nil)
	r.waitURL(t, r.srv.URL+"/")
	var loggedIn bool
	r.eval(t, ctx, `window.loggedIn === true`, &loggedIn)
	if !loggedIn {
		t.Fatal("the tab under the new one came back reloaded, its script's state gone")
	}
}

func TestChromeDialogTheTabBehindOpensIsAnsweredThere(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	r := startNewTabRig(t, ctx, "/opener")
	r.followLink(t, ctx)
	// opener.eval, so the alert is the opener's and not this tab's.
	r.eval(t, ctx, `opener.eval('setTimeout(() => alert("saved"), 0)')`, nil)
	select {
	case d := <-r.dialogs:
		if d == nil || d.Message != "saved" {
			t.Fatalf("dialog = %+v, want the one the tab behind opened", d)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the dialog the tab behind opened never showed")
	}
	if err := r.p.AnswerDialog(ctx, true, ""); err != nil {
		t.Fatal(err)
	}
	var href string
	r.eval(t, ctx, `location.href`, &href)
	if href != r.srv.URL+"/elsewhere" {
		t.Fatalf("after the dialog the page drives %q, want the front tab", href)
	}
}

func TestChromeScrollTurnsTheWheelAtThePoint(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<body style="margin:0"><div style="height:5000px">tall</div></body>`)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	x, y, err := p.Scroll(ctx, nil, 400)
	if err != nil {
		t.Fatal(err)
	}
	if x <= 0 || y <= 0 {
		t.Fatalf("scrolled at %v,%v, want the viewport's middle", x, y)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		var scrolled float64
		if err := p.run(ctx, chromedp.Evaluate(`scrollY`, &scrolled)); err != nil {
			t.Fatal(err)
		}
		if scrolled > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the wheel turned and the page did not scroll")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestChromeSelectionReadsAFieldAndNotAPassword(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<input id="q" value="hello world"><input id="pw" type="password" value="secret"><p id="p">plain text</p>`)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ js, want string }{
		{`q.focus(); q.setSelectionRange(6, 11)`, "world"},
		{`pw.focus(); pw.select()`, ""},
		{`document.activeElement.blur(); getSelection().selectAllChildren(document.getElementById('p'))`, "plain text"},
	}
	for _, c := range cases {
		if err := p.run(ctx, chromedp.Evaluate(c.js, nil)); err != nil {
			t.Fatal(err)
		}
		got, err := p.Selection(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("after %s: selection %q, want %q", c.js, got, c.want)
		}
	}
}

func TestChromeClickReachesAnElementScrolledOutOfView(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<body style="margin:0"><div style="height:3000px">form</div>
<button id="save" onclick="document.body.dataset.saved = 'yes'">Salvar</button></body>`)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	x, y, err := p.Click(ctx, Target{Selector: "#save"})
	if err != nil {
		t.Fatal(err)
	}
	if y < 0 || y > 600 || x < 0 || x > 800 {
		t.Fatalf("clicked at %v,%v, outside the 800x600 viewport", x, y)
	}
	var saved string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.saved || ""`, &saved)); err != nil {
		t.Fatal(err)
	}
	if saved != "yes" {
		t.Fatal("the button below the fold was not clicked")
	}
}

func TestChromeClickRefusesAPointOutsideTheViewport(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	x, y := 400.0, 715.0
	_, _, err := p.Click(ctx, Target{X: &x, Y: &y})
	if err == nil || !strings.Contains(err.Error(), "800x600") {
		t.Fatalf("click at 400,715 in an 800x600 viewport: %v, want an error naming the viewport", err)
	}
}

func TestChromeClickByIndexHitsTheLastReadNotAnEarlierOne(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<body style="margin:0">
<button onclick="document.body.dataset.hit = 'top'">Top</button>
<div style="height:3000px"></div>
<button onclick="document.body.dataset.hit = 'bottom'">Bottom</button></body>`)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Info(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil)); err != nil {
		t.Fatal(err)
	}
	info, err := p.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Interactive) != 1 || info.Interactive[0].Text != "Bottom" {
		t.Fatalf("second read listed %+v, want only Bottom", info.Interactive)
	}
	index := info.Interactive[0].Index
	if _, _, err := p.Click(ctx, Target{Index: &index}); err != nil {
		t.Fatal(err)
	}
	var hit string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.hit || ""`, &hit)); err != nil {
		t.Fatal(err)
	}
	if hit != "bottom" {
		t.Fatalf("index %d from the last read clicked %q, want bottom", index, hit)
	}
}
