package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// livePages serves each path's body as a page of its own.
func livePages(t *testing.T, pages map[string]string) (*chromePage, *httptest.Server) {
	t.Helper()
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, pages[r.URL.Path])
	}))
	t.Cleanup(srv.Close)
	return p, srv
}

func liveOpen(t *testing.T, p *chromePage, url string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	t.Cleanup(cancel)
	if err := p.Navigate(ctx, url); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func liveEval(t *testing.T, ctx context.Context, p *chromePage, script string, out any) {
	t.Helper()
	if err := p.run(ctx, chromedp.Evaluate(script, out)); err != nil {
		t.Fatal(err)
	}
}

// indexOf is the index info lists the element labelled text at.
func indexOf(t *testing.T, info PageInfo, text string) int {
	t.Helper()
	for _, e := range info.Interactive {
		if e.Text == text {
			return e.Index
		}
	}
	t.Fatalf("%q is not in %+v", text, info.Interactive)
	return 0
}

func liveInfo(t *testing.T, ctx context.Context, p *chromePage) PageInfo {
	t.Helper()
	info, err := p.Info(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func liveHit(t *testing.T, ctx context.Context, p *chromePage) string {
	t.Helper()
	var hit string
	liveEval(t, ctx, p, `document.body.dataset.hit || ""`, &hit)
	return hit
}

const twoButtons = `<body style="margin:0">
<button onclick="document.body.dataset.hit = 'a'">A</button>
<button onclick="document.body.dataset.hit = 'b'">B</button></body>`

func TestChromeIndexSurvivesAnElementInsertedAboveIt(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": twoButtons})
	ctx := liveOpen(t, p, srv.URL)
	before := liveInfo(t, ctx, p)
	b := indexOf(t, before, "B")
	liveEval(t, ctx, p, `(() => { const n = document.createElement('button'); n.textContent = 'New';
n.onclick = () => document.body.dataset.hit = 'new'; document.body.prepend(n); })()`, nil)
	after := liveInfo(t, ctx, p)
	if got := indexOf(t, after, "B"); got != b {
		t.Fatalf("B was %d and is %d after a button went in above it", b, got)
	}
	if after.Interactive[0].Text != "New" {
		t.Fatalf("the list is %+v, want page order with New first", after.Interactive)
	}
	for _, e := range before.Interactive {
		if after.Interactive[0].Index <= e.Index {
			t.Fatalf("New took %d, not past %+v", after.Interactive[0].Index, before.Interactive)
		}
	}
	if _, _, err := p.Click(ctx, Target{Index: &b}); err != nil {
		t.Fatal(err)
	}
	if hit := liveHit(t, ctx, p); hit != "b" {
		t.Fatalf("index %d clicked %q, want b", b, hit)
	}
}

func TestChromeIndexOfARemovedElementIsNotHandedOutAgain(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": twoButtons})
	ctx := liveOpen(t, p, srv.URL)
	a := indexOf(t, liveInfo(t, ctx, p), "A")
	liveEval(t, ctx, p, `(() => { document.querySelector('button').remove(); const n = document.createElement('button');
n.textContent = 'C'; n.onclick = () => document.body.dataset.hit = 'c'; document.body.prepend(n); })()`, nil)
	if c := indexOf(t, liveInfo(t, ctx, p), "C"); c == a {
		t.Fatalf("C took %d, the removed A's index", c)
	}
	_, _, err := p.Click(ctx, Target{Index: &a})
	if err == nil || !strings.Contains(err.Error(), "read it again") {
		t.Fatalf("click on the removed A's index: %v, want the page-changed error", err)
	}
	if hit := liveHit(t, ctx, p); hit != "" {
		t.Fatalf("the removed A's index clicked %q", hit)
	}
}

func TestChromeIndexOfAClonedElementIsNotTheClones(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": twoButtons})
	ctx := liveOpen(t, p, srv.URL)
	b := indexOf(t, liveInfo(t, ctx, p), "B")
	liveEval(t, ctx, p, `(() => { const n = document.querySelectorAll('button')[1].cloneNode(true); n.textContent = 'Copy';
n.onclick = () => document.body.dataset.hit = 'copy'; document.body.prepend(n); })()`, nil)
	if _, _, err := p.Click(ctx, Target{Index: &b}); err != nil {
		t.Fatal(err)
	}
	if hit := liveHit(t, ctx, p); hit != "b" {
		t.Fatalf("index %d clicked %q before a read, want b and not its clone", b, hit)
	}
	after := liveInfo(t, ctx, p)
	if c := indexOf(t, after, "Copy"); c == b {
		t.Fatalf("the clone kept B's index %d", b)
	}
}

func TestChromeIndexFromAnotherDocumentFindsNothing(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": twoButtons, "/next": twoButtons})
	ctx := liveOpen(t, p, srv.URL)
	a := indexOf(t, liveInfo(t, ctx, p), "A")
	if err := p.Navigate(ctx, srv.URL+"/next"); err != nil {
		t.Fatal(err)
	}
	if got := indexOf(t, liveInfo(t, ctx, p), "A"); got == a {
		t.Fatalf("the next page's A took %d, the last page's A's index", got)
	}
	_, _, err := p.Click(ctx, Target{Index: &a})
	if err == nil || !strings.Contains(err.Error(), "read it again") {
		t.Fatalf("click on the last page's index: %v, want the page-changed error", err)
	}
	if hit := liveHit(t, ctx, p); hit != "" {
		t.Fatalf("the last page's index clicked %q on the next one", hit)
	}
}

func TestChromeIndexOfAnElementScrolledAwayStillClicks(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": `<body style="margin:0">
<button onclick="document.body.dataset.hit = 'top'">Top</button>
<div style="height:3000px"></div>
<button onclick="document.body.dataset.hit = 'bottom'">Bottom</button></body>`})
	ctx := liveOpen(t, p, srv.URL)
	top := indexOf(t, liveInfo(t, ctx, p), "Top")
	liveEval(t, ctx, p, `window.scrollTo(0, document.body.scrollHeight)`, nil)
	if info := liveInfo(t, ctx, p); len(info.Interactive) != 1 || info.Interactive[0].Text != "Bottom" {
		t.Fatalf("read at the bottom listed %+v, want only Bottom", info.Interactive)
	}
	if _, _, err := p.Click(ctx, Target{Index: &top}); err != nil {
		t.Fatal(err)
	}
	if hit := liveHit(t, ctx, p); hit != "top" {
		t.Fatalf("index %d clicked %q, want top", top, hit)
	}
}

func TestChromeIndexOfAnElementNoLongerShownIsNotClicked(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": twoButtons})
	ctx := liveOpen(t, p, srv.URL)
	b := indexOf(t, liveInfo(t, ctx, p), "B")
	liveEval(t, ctx, p, `document.querySelectorAll('button')[1].style.display = 'none'`, nil)
	_, _, err := p.Click(ctx, Target{Index: &b})
	if err == nil || !strings.Contains(err.Error(), "read it again") {
		t.Fatalf("click on a hidden element's index: %v, want the page-changed error", err)
	}
}
