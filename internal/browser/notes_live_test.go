package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

const notesProbe = `<body style="margin:0;font:16px sans-serif">
<div class="plans">
 <div class="plan css-1a2b3c4"><b>Hobby</b><button class="cta">Get started</button></div>
 <div class="plan pro"><b>Pro</b><button class="cta">Start trial</button></div>
 <div class="plan"><b>Team</b><button class="cta" style="display:block;width:150px;height:40px">Contact sales</button></div>
</div>
<form id="login" style="margin-top:20px">
 <input id="pw" type="password" value="hunter2">
 <input id="q" name="q" placeholder="Search">
</form></body>`

func notesPage(t *testing.T) (*chromePage, context.Context) {
	t.Helper()
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, notesProbe)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	t.Cleanup(cancel)
	if err := p.Navigate(ctx, srv.URL+"/pricing?plan=pro#top"); err != nil {
		t.Fatal(err)
	}
	if err := p.Resize(ctx, Viewport{W: 800, H: 600, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	return p, ctx
}

func boxOf(t *testing.T, p *chromePage, ctx context.Context, selector string) api.BrowserRect {
	t.Helper()
	boxes, err := p.Locate(ctx, []string{selector})
	if err != nil {
		t.Fatal(err)
	}
	return boxes[0]
}

func TestChromeElementAtNamesTheElementAndASelectorThatFindsItAgain(t *testing.T) {
	p, ctx := notesPage(t)
	team := boxOf(t, p, ctx, ".plan:nth-of-type(3) .cta")
	el, err := p.ElementAt(ctx, float64(team.X+team.W/2), float64(team.Y+team.H/2))
	if err != nil {
		t.Fatal(err)
	}
	if el.Label != `button.cta "Contact sales"` {
		t.Errorf("label %q", el.Label)
	}
	if got := boxOf(t, p, ctx, el.Selector); got != team || got.W == 0 {
		t.Errorf("selector %q finds %+v, want the Team button at %+v", el.Selector, got, team)
	}
	if el.Box != team {
		t.Errorf("box %+v, want %+v", el.Box, team)
	}
	if strings.ContainsAny(el.URL, "?#") || !strings.HasSuffix(el.URL, "/pricing") {
		t.Errorf("url %q keeps its query or fragment", el.URL)
	}
}

func TestChromeElementAtSkipsAGeneratedClassName(t *testing.T) {
	p, ctx := notesPage(t)
	hobby := boxOf(t, p, ctx, ".plan b")
	el, err := p.ElementAt(ctx, float64(hobby.X+1), float64(hobby.Y+1))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(el.Selector, "css-") {
		t.Fatalf("selector %q names a CSS-in-JS class", el.Selector)
	}
}

func TestChromeElementAtKeepsFieldValuesOutOfTheMarkup(t *testing.T) {
	p, ctx := notesPage(t)
	if err := p.Type(ctx, "my secret query", false, Target{Selector: "#q"}); err != nil {
		t.Fatal(err)
	}
	q := boxOf(t, p, ctx, "#q")
	el, err := p.ElementAt(ctx, float64(q.X+q.W+4), float64(q.Y+q.H/2))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(el.Selector, "form#login") {
		t.Fatalf("picked %q, want the form", el.Selector)
	}
	for _, secret := range []string{"hunter2", "my secret query"} {
		if strings.Contains(el.HTML, secret) {
			t.Errorf("markup carries %q: %s", secret, el.HTML)
		}
	}
}

func TestChromeElementAtThePageRootAnswersNothing(t *testing.T) {
	p, ctx := notesPage(t)
	el, err := p.ElementAt(ctx, 790, 590)
	if err != nil {
		t.Fatal(err)
	}
	if el.Label != "" {
		t.Fatalf("empty page area picked %+v", el)
	}
}

func TestChromeLocateAnswersAnEmptyBoxForAnElementGone(t *testing.T) {
	p, ctx := notesPage(t)
	if got := boxOf(t, p, ctx, "#gone"); got != (api.BrowserRect{}) {
		t.Fatalf("box of a missing element = %+v", got)
	}
	if got := boxOf(t, p, ctx, "::bad"); got != (api.BrowserRect{}) {
		t.Fatalf("box of an invalid selector = %+v", got)
	}
}

// reactProbe gives two elements a React fiber, as a development build and a
// production build leave them: a named component with its source, and one
// whose name the minifier shortened.
const reactProbe = `<body style="margin:0">
<button id="dev" style="display:block;width:200px;height:40px">Dev</button>
<button id="prod" style="display:block;width:200px;height:40px">Prod</button>
<script>
function PlanCard() {}
function at() {}
document.getElementById('dev')['__reactFiber$x1'] = {type: 'button',
	_debugSource: {fileName: '/home/me/app/src/PlanCard.tsx', lineNumber: 31},
	return: {type: PlanCard, return: null}};
document.getElementById('prod')['__reactFiber$x1'] = {type: 'button', return: {type: at, return: null}};
</script></body>`

func TestChromeElementAtNamesTheReactComponentButNotAMinifiedOne(t *testing.T) {
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, reactProbe)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	if err := p.Navigate(ctx, srv.URL); err != nil {
		t.Fatal(err)
	}
	dev, err := p.ElementAt(ctx, 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if dev.Component != "PlanCard" || dev.Source != "src/PlanCard.tsx:31" {
		t.Errorf("dev build: component %q source %q, want PlanCard at src/PlanCard.tsx:31", dev.Component, dev.Source)
	}
	prod, err := p.ElementAt(ctx, 20, 60)
	if err != nil {
		t.Fatal(err)
	}
	if prod.Component != "" {
		t.Errorf("production build names component %q, a minifier's name", prod.Component)
	}
}
