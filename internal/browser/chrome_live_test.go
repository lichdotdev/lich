package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
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
	return liveChromeAt(t, frameScale)
}

// liveChromeAt is a headless Chrome drawing at scale, and a page to load.
func liveChromeAt(t *testing.T, scale float64) (*chromePage, *httptest.Server) {
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
	p, err := chromeLaunch(t.TempDir(), false, scale)
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
	launch := func(dir string, _ bool, scale float64) (Page, error) { return chromeLaunch(dir, false, scale) }
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
	p.Listen(func(_, data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(_, title string) {
		select {
		case titles <- title:
		default:
		}
	}, func(string, api.BrowserDownload) {}, func(string) {}, func(*api.BrowserDialog) {}, func([]api.BrowserTab) {})
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
	p.Listen(func(_, data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {}, func(*api.BrowserDialog) {}, func([]api.BrowserTab) {})
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

func TestChromeLaunchedAtA1xWindowsScaleSendsThePanesOwnPixels(t *testing.T) {
	p, srv := liveChromeAt(t, 1)
	frames := make(chan string, 1)
	p.Listen(func(_, data string) {
		select {
		case frames <- data:
		default:
		}
	}, func(string, string) {}, func(string, api.BrowserDownload) {}, func(string) {}, func(*api.BrowserDialog) {}, func([]api.BrowserTab) {})
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Screencast(ctx, Viewport{W: 400, H: 300, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if ratio, err := p.Eval(ctx, "devicePixelRatio"); err != nil || ratio != "1" {
		t.Fatalf("devicePixelRatio = %q, %v, want 1: the browser draws at the window's scale", ratio, err)
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
			if cfg.Width == 400 && cfg.Height == 300 {
				return
			}
			last = fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
		case <-deadline:
			t.Fatalf("last frame %s, want the 1x pane's own 400x300", last)
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
	pg, err := chromeLaunch(t.TempDir(), false, frameScale)
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
	r.p.Listen(func(string, string) {}, func(url, _ string) { r.urls <- url }, func(string, api.BrowserDownload) {}, func(string) {},
		func(d *api.BrowserDialog) { r.dialogs <- d }, func([]api.BrowserTab) {})
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

func TestChromePageTextLeavesOutTheListedLabelsAndCutsAt1000(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<nav><a href="/a">Pricing</a><br><button>Sign in</button></nav>
<p>Pricing plans for teams</p><p>%s</p>`, strings.Repeat("x", 1500))
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	info, err := p.Info(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Interactive) != 2 {
		t.Fatalf("listed %+v, want the link and the button", info.Interactive)
	}
	if !strings.HasPrefix(info.Text, "Pricing plans for teams\n") || strings.Contains(info.Text, "Sign in") {
		t.Fatalf("text %.60q…, want the paragraph without the listed labels", info.Text)
	}
	if n := len([]rune(info.Text)); n != 1000 {
		t.Fatalf("text of %d runes, want it cut at 1000", n)
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
	if _, err := p.Info(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := p.run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil)); err != nil {
		t.Fatal(err)
	}
	info, err := p.Info(ctx, "")
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

func TestChromeScreenshotIsInCSSPixelsOnAHiDPIPane(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: frameScale}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir() + "/shot.png"
	w, h, err := p.Screenshot(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if w != 800 || h != 600 || img.Width != 800 || img.Height != 600 {
		t.Fatalf("screenshot = %dx%d image, %dx%d reported, want 800x600 for both", img.Width, img.Height, w, h)
	}
}

func TestChromeScreenshotOfAWidePaneIs1280AndClicksMapBack(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<body style="margin:0;height:100vh" onclick="document.body.dataset.at = event.clientX + ',' + event.clientY">`)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 2000, H: 1000, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir() + "/shot.png"
	w, h, err := p.Screenshot(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if w != 2000 || h != 1000 || img.Width != 1280 || img.Height != 640 {
		t.Fatalf("screenshot = %dx%d image of a %dx%d viewport, want 1280x640 of 2000x1000", img.Width, img.Height, w, h)
	}
	x, y := 640.0, 320.0
	if _, _, err := p.Click(ctx, Target{X: &x, Y: &y, shotW: w, shotH: h}); err != nil {
		t.Fatal(err)
	}
	var at string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.at`, &at)); err != nil {
		t.Fatal(err)
	}
	if at != "1000,500" {
		t.Fatalf("the image's 640,320 clicked the page at %s, want 1000,500", at)
	}
	x = 1280
	if _, _, err := p.Click(ctx, Target{X: &x, Y: &y, shotW: w, shotH: h}); err == nil || !strings.Contains(err.Error(), "1280x640") {
		t.Fatalf("a point past the image's edge answered %v, want it named against the 1280x640 image", err)
	}
}

func TestChromeClickRefusesAPointReadOffAnotherViewport(t *testing.T) {
	p, srv := liveChrome(t)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	x, y := 50.0, 25.0
	if _, _, err := p.Click(ctx, Target{X: &x, Y: &y, shotW: 800, shotH: 600}); err != nil {
		t.Fatalf("click read off this viewport: %v", err)
	}
	if err := p.Resize(ctx, Viewport{W: 700, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	_, _, err := p.Click(ctx, Target{X: &x, Y: &y, shotW: 800, shotH: 600})
	if err == nil || !strings.Contains(err.Error(), "new screenshot") {
		t.Fatalf("click read off 800x600 in a 700x600 viewport: %v, want an error asking for a new screenshot", err)
	}
}

func clickRig(t *testing.T, body string) (*chromePage, context.Context) {
	t.Helper()
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<body style="margin:0">`+body)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	t.Cleanup(cancel)
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	return p, ctx
}

func clicked(t *testing.T, p *chromePage, ctx context.Context) string {
	t.Helper()
	var hit string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.hit || ""`, &hit)); err != nil {
		t.Fatal(err)
	}
	return hit
}

func TestChromeClickRefusesAnElementUnderAnOverlayAndClicksOnceItIsGone(t *testing.T) {
	p, ctx := clickRig(t, `<button id="buy" onclick="document.body.dataset.hit = 'buy'">Buy</button>
<div id="consent-banner" style="position:fixed;inset:0;background:#0008"></div>`)
	_, _, err := p.Click(ctx, Target{Selector: "#buy"})
	if err == nil || !strings.Contains(err.Error(), "<div#consent-banner>") {
		t.Fatalf("click under an overlay: %v, want an error naming div#consent-banner", err)
	}
	if hit := clicked(t, p, ctx); hit != "" {
		t.Fatalf("the click reached %q under the overlay", hit)
	}
	if err := p.run(ctx, chromedp.Evaluate(`document.getElementById('consent-banner').remove()`, nil)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Click(ctx, Target{Selector: "#buy"}); err != nil {
		t.Fatal(err)
	}
	if hit := clicked(t, p, ctx); hit != "buy" {
		t.Fatalf("clicked %q after the overlay left, want buy", hit)
	}
}

func TestChromeClickAndTypeRefuseADisabledTarget(t *testing.T) {
	p, ctx := clickRig(t, `<button id="off" disabled onclick="document.body.dataset.hit = 'off'">Off</button>
<fieldset disabled><input id="in"></fieldset>
<div id="aria" role="button" aria-disabled="true" style="height:20px">Aria</div>`)
	for _, sel := range []string{"#off", "#in", "#aria"} {
		if _, _, err := p.Click(ctx, Target{Selector: sel}); err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("click %s: %v, want a disabled error", sel, err)
		}
	}
	if err := p.Type(ctx, "x", false, Target{Selector: "#in"}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("type into a fieldset-disabled input: %v, want a disabled error", err)
	}
	if hit := clicked(t, p, ctx); hit != "" {
		t.Fatalf("the click reached %q", hit)
	}
}

func TestChromeClickIsNotCoveredByItsOwnChildOrItsLabel(t *testing.T) {
	p, ctx := clickRig(t, `<a id="link" href="#x" onclick="document.body.dataset.hit = 'link'; return false"><span style="display:block;padding:10px">Open</span></a>
<label style="display:block;margin-top:40px"><input id="box" type="checkbox" style="position:absolute;opacity:0;left:0;top:60px;width:20px;height:20px"
 onclick="document.body.dataset.hit = 'box'"><span style="display:block;position:absolute;left:0;top:60px;width:20px;height:20px;background:#ccc"></span></label>`)
	if _, _, err := p.Click(ctx, Target{Selector: "#link"}); err != nil {
		t.Fatal(err)
	}
	if hit := clicked(t, p, ctx); hit != "link" {
		t.Fatalf("clicked %q, want link", hit)
	}
	if _, _, err := p.Click(ctx, Target{Selector: "#box"}); err != nil {
		t.Fatalf("a checkbox under its own label's styled span: %v", err)
	}
	if hit := clicked(t, p, ctx); hit != "box" {
		t.Fatalf("clicked %q, want box", hit)
	}
}

func TestChromeClickReachesALinkWrappedOverTwoLines(t *testing.T) {
	p, ctx := clickRig(t, `<div style="width:200px;font:16px/48px monospace">aaaaaaaaaaaaaa <a id="link" href="#x" onclick="document.body.dataset.hit = 'link'; return false">go to the next place</a></div>`)
	if _, _, err := p.Click(ctx, Target{Selector: "#link"}); err != nil {
		t.Fatalf("click on a link wrapped over two lines: %v", err)
	}
	if hit := clicked(t, p, ctx); hit != "link" {
		t.Fatalf("clicked %q, want link", hit)
	}
}

func TestChromeInfoWithinReadsOnlyTheSubtreeAndItsIndexesClick(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<nav>Site navigation<br><a href="/docs">Docs</a><button>Menu</button></nav>
<main>Main column text<br><button onclick="document.body.dataset.hit = 'save'">Save</button></main>
<footer>Footer words<br><button>Legal</button></footer>`)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	info, err := p.Info(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}
	if info.Text != "Main column text" {
		t.Fatalf("text %q, want only main's, without the label of its button", info.Text)
	}
	if len(info.Interactive) != 1 || info.Interactive[0].Text != "Save" {
		t.Fatalf("listed %+v, want only Save", info.Interactive)
	}
	for _, within := range []string{"#nope", "a["} {
		_, err := p.Info(ctx, within)
		if err == nil {
			t.Fatalf("within %q: no error", within)
		}
		if within == "#nope" && !strings.Contains(err.Error(), "no element matches #nope") {
			t.Fatalf("within %q: %v, want it to say nothing matches", within, err)
		}
	}
	// A refused selector wiped nothing: the scoped read's index still resolves.
	index := info.Interactive[0].Index
	if _, _, err := p.Click(ctx, Target{Index: &index}); err != nil {
		t.Fatal(err)
	}
	var hit string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.hit || ""`, &hit)); err != nil {
		t.Fatal(err)
	}
	if hit != "save" {
		t.Fatalf("index %d from the scoped read clicked %q, want save", index, hit)
	}
	whole, err := p.Info(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(whole.Interactive) != 4 {
		t.Fatalf("whole page listed %+v, want the 4 buttons and links", whole.Interactive)
	}
}

func TestChromeAnnotatedScreenshotLabelsWhatAClickByIndexHits(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		icon := `<svg width="24" height="24"><circle cx="12" cy="12" r="10"/></svg>`
		fmt.Fprintf(w, `<body style="margin:0">
<button id="star" style="position:absolute;left:100px;top:100px;width:80px;height:60px">%s</button>
<button id="trash" style="position:absolute;left:400px;top:300px;width:80px;height:60px">%s</button>
<script>addEventListener('click', (e) => { document.body.dataset.hit = e.target.closest('button')?.id || '' })</script>`, icon, icon)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 2000, H: 1000, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	const layout = `[...document.querySelectorAll('button')].map((b) => JSON.stringify(b.getBoundingClientRect())).join()`
	var before, after string
	if err := p.run(ctx, chromedp.Evaluate(layout, &before)); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir() + "/shot.png"
	w, h, err := p.AnnotatedScreenshot(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if w != 2000 || h != 1000 || img.Bounds().Dx() != 1280 || img.Bounds().Dy() != 640 {
		t.Fatalf("annotated shot = %v image of %dx%d, want 1280x640 of 2000x1000", img.Bounds(), w, h)
	}
	// Inside the label's padding at trash's top left corner, left of its
	// digit, whose antialiasing differs between platforms' fonts: CSS
	// 400+3,300+10 at 0.64.
	if r, g, b, _ := img.At(258, 198).RGBA(); r>>8 < 0xb0 || g>>8 > 0x40 || b>>8 < 0x40 {
		t.Fatalf("pixel under trash's label = %d,%d,%d, want the label's red", r>>8, g>>8, b>>8)
	}
	var left int
	if err := p.run(ctx, chromedp.Evaluate(`document.querySelectorAll('`+annotateTag+`').length`, &left)); err != nil {
		t.Fatal(err)
	}
	if err := p.run(ctx, chromedp.Evaluate(layout, &after)); err != nil {
		t.Fatal(err)
	}
	if left != 0 || after != before {
		t.Fatalf("after the shot: %d overlays left, layout %s, want none and %s", left, after, before)
	}
	trash := 1
	if _, _, err := p.Click(ctx, Target{Index: &trash}); err != nil {
		t.Fatal(err)
	}
	var hit string
	if err := p.run(ctx, chromedp.Evaluate(`document.body.dataset.hit`, &hit)); err != nil {
		t.Fatal(err)
	}
	if hit != "trash" {
		t.Fatalf("a click on index 1 hit %q, want trash", hit)
	}
}
