package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// framedApp is a system that draws everything inside a same-origin iframe:
// the frame stands at 40,100 of the viewport past a 5px border, and its
// document holds a field, a Save button and a Far button below its own
// scroll.
const (
	framedTop = `<body style="margin:0">Shell
<iframe id="app" src="/inner" style="position:absolute;left:40px;top:100px;width:400px;height:300px;border:5px solid"></iframe>
</body>`
	framedInner = `<body style="margin:0">Inside the frame
<input id="name" value="old" style="display:block;width:150px;height:30px">
<button class="save" style="display:block;width:150px;height:40px" onclick="parent.document.body.dataset.hit = 'save'">Save</button>
<div style="height:800px"></div>
<button id="far" style="display:block;width:150px;height:40px" onclick="parent.document.body.dataset.hit = 'far'">Far</button>
</body>`
)

func framedPage(t *testing.T) (*chromePage, context.Context) {
	t.Helper()
	p, _ := liveChrome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/inner" {
			fmt.Fprint(w, framedInner)
			return
		}
		fmt.Fprint(w, framedTop)
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
	for loaded := false; !loaded; time.Sleep(20 * time.Millisecond) {
		if err := p.evaluate(ctx, `!!document.getElementById('app').contentDocument.querySelector('#far')`, &loaded); err != nil {
			t.Fatal(err)
		}
	}
	return p, ctx
}

func framedHit(t *testing.T, p *chromePage, ctx context.Context) string {
	t.Helper()
	var hit string
	if err := p.evaluate(ctx, `document.body.dataset.hit || ''`, &hit); err != nil {
		t.Fatal(err)
	}
	return hit
}

func TestChromeInfoReadsASameOriginFrameAndItsIndexesClick(t *testing.T) {
	t.Parallel()
	p, ctx := framedPage(t)
	info, err := p.Info(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.Text, "Shell") || !strings.Contains(info.Text, "Inside the frame") {
		t.Fatalf("text %q, want the page's and the frame's", info.Text)
	}
	var save *Interactive
	for i := range info.Interactive {
		if info.Interactive[i].Text == "Save" {
			save = &info.Interactive[i]
		}
	}
	if save == nil {
		t.Fatalf("listed %+v, want the frame's Save", info.Interactive)
	}
	if save.Rect.X != 45 || save.Rect.Y < 105 || save.Rect.W != 150 {
		t.Errorf("Save at %+v, want it in the top viewport, past the frame's 45,105", save.Rect)
	}
	if _, _, err := p.Click(ctx, Target{Index: &save.Index}); err != nil {
		t.Fatal(err)
	}
	if hit := framedHit(t, p, ctx); hit != "save" {
		t.Fatalf("click by index hit %q, want save", hit)
	}
}

func TestChromeClickBySelectorReachesIntoAFrameAndScrollsIt(t *testing.T) {
	t.Parallel()
	p, ctx := framedPage(t)
	if _, _, err := p.Click(ctx, Target{Selector: "iframe#app >>> #far"}); err != nil {
		t.Fatal(err)
	}
	if hit := framedHit(t, p, ctx); hit != "far" {
		t.Fatalf("click by selector hit %q, want far", hit)
	}
	if _, _, err := p.Click(ctx, Target{Selector: "iframe#app >>> #gone"}); err == nil {
		t.Fatal("a selector into the frame that finds nothing clicked")
	}
}

func TestChromeTypeClearsAFieldInsideAFrame(t *testing.T) {
	t.Parallel()
	p, ctx := framedPage(t)
	if err := p.Type(ctx, "new", true, Target{Selector: "iframe#app >>> #name"}); err != nil {
		t.Fatal(err)
	}
	var value string
	if err := p.evaluate(ctx, `document.getElementById('app').contentDocument.getElementById('name').value`, &value); err != nil {
		t.Fatal(err)
	}
	if value != "new" {
		t.Fatalf("field holds %q, want new", value)
	}
}

func TestChromeInfoWithinAndWaitForReachIntoAFrame(t *testing.T) {
	t.Parallel()
	p, ctx := framedPage(t)
	info, err := p.Info(ctx, "iframe#app >>> body")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(info.Text, "Shell") || !strings.Contains(info.Text, "Inside the frame") {
		t.Fatalf("text %q, want only the frame's", info.Text)
	}
	has, err := p.HasText(ctx, "Inside the frame")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("the frame's text is not on the page for wait_for")
	}
}
