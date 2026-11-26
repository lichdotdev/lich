package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/browserwire"
)

// The page under test: an input that logs every input event it gets, and
// whether the browser made it.
const formPage = `<!doctype html><title>Form</title>
<link rel=icon href=/icon.png>
<input id=field style="position:absolute;left:10px;top:10px;width:200px;height:30px">
<script>
window.got = [];
for (const type of ['mousedown', 'click', 'keydown', 'beforeinput', 'input']) {
	addEventListener(type, e => got.push({type, trusted: e.isTrusted}), true);
}
</script>`

const downloadBody = "the file's bytes"

func site(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(formPage))
	})
	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, r *http.Request) {
		img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
		for i := range img.Pix {
			img.Pix[i] = 0xff
		}
		img.Set(1, 1, color.NRGBA{R: 0xff, A: 0xff})
		png.Encode(w, img)
	})
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="notes.txt"`)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(downloadBody))
	})
	mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src=/icon.png?a><img src=/icon.png?b>`))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func TestHelloAnswersTheWireVersion(t *testing.T) {
	c := start(t)
	var r browserwire.HelloResult
	c.must(browserwire.Hello, nil, &r)
	if r.Version != browserwire.Version {
		t.Fatalf("hello answered version %d, want %d", r.Version, browserwire.Version)
	}
}

func TestEvalAnswersAsTheConsoleDoes(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,<title>Hi</title>", 400, 300, 1)
	for _, tc := range []struct {
		name, script, want, err string
	}{
		{"an expression", "document.title", `"Hi"`, ""},
		{"statements, by their last expression", "const a = 1; a + 1", "2", ""},
		{"a const declared again by a later eval", "const a = 5\na * 2", "10", ""},
		{"a let one eval declares, read by the next", "let kept = 7", "", ""},
		{"what the eval before declared", "kept", "7", ""},
		{"an expression over lines", "kept\n+ 1", "8", ""},
		{"top-level await", "const t = await Promise.resolve(document.title); t + '!'", `"Hi!"`, ""},
		{"a promise, awaited", "new Promise(r => setTimeout(() => r([1, 2]), 10))", "[1,2]", ""},
		{"undefined, as no JSON", "undefined", "", ""},
		{"a class declared again", "class K {}; class K { static v = 3 }; K.v", "3", ""},
		{"a throw, as the page words it", `throw new TypeError("nope")`, "", "TypeError: nope"},
		{"a rejection", `Promise.reject(new Error("later"))`, "", "Error: later"},
		{"no script", "(1", "", "SyntaxError"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r browserwire.EvalResult
			err := c.call(browserwire.Eval, browserwire.EvalParams{Page: "b1", Script: tc.script}, &r)
			if tc.err != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tc.err) {
					t.Fatalf("eval %q failed with %v, want %q", tc.script, err, tc.err)
				}
				return
			}
			if err != nil || r.JSON != tc.want {
				t.Fatalf("eval %q = %q, %v; want %q", tc.script, r.JSON, err, tc.want)
			}
		})
	}
}

func TestAPageNoWindowShowsRunsAsOneInSight(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,", 400, 300, 1)
	got := c.eval("b1", "new Promise(r => requestAnimationFrame(() => r([document.visibilityState, innerWidth, innerHeight])))")
	if got != `["visible",400,300]` {
		t.Fatalf("the page sees %s, want a visible 400x300 viewport", got)
	}
}

func TestSnapshotIsTheViewportAtTheScale(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,<body style='background:rgb(255,0,0)'>", 300, 200, 2)
	var r browserwire.SnapshotResult
	c.must(browserwire.Snapshot, browserwire.PageParams{Page: "b1"}, &r)
	img, err := png.Decode(bytes.NewReader(r.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if r.W != 300 || r.H != 200 || img.Bounds().Dx() != 600 || img.Bounds().Dy() != 400 {
		t.Fatalf("snapshot %dx%d CSS, %v image; want 300x200 drawn at 600x400", r.W, r.H, img.Bounds())
	}
	if red, _, _, _ := img.At(300, 200).RGBA(); red>>8 != 0xff {
		t.Fatalf("the snapshot's middle is %v, want the page's red", img.At(300, 200))
	}
}

// keysPage is a form with two fields, the first holding "ab", which records
// each key event it gets and its submit.
const keysPage = `data:text/html,<form id=f onsubmit="event.preventDefault(); window.submitted = true">
<input id=a value=ab><input id=b><button>Go</button></form>
<script>window.keys = []; for (const t of ['keydown', 'keyup']) document.addEventListener(t, e => keys.push(t + ':' + e.key), true);</script>`

// A named key does what it does on Chrome, with the page's input method
// whatever the desktop runs: the person's fcitx5 swallowed every key the
// wire sent, so no Enter, Tab or Backspace reached the page (rc.4).
func TestNamedKeysActOnThePage(t *testing.T) {
	pressNamedKeys(t, false)
}

// An embedded page knows the window's X window, where keys the person types
// over the page go on to. An input method that hands a key back later
// (fcitx5) had the wire's keys sent there too, so they never reached the
// page. The window here is another lich-browser's tab, which takes keys as a
// page. Measured under fcitx5 on its own D-Bus and Xvfb; with no input
// method, it guards the path every key takes.
func TestNamedKeysActOnAnEmbeddedPage(t *testing.T) {
	pressNamedKeys(t, true)
}

func pressNamedKeys(t *testing.T, embedded bool) {
	for _, tc := range []struct{ key, check, want string }{
		{"Backspace", "a.value", `"a"`},
		{"Enter", "!!window.submitted", "true"},
		{"Tab", "document.activeElement.id", `"b"`},
	} {
		t.Run(tc.key, func(t *testing.T) {
			c := start(t)
			if embedded {
				c.must(browserwire.Window, browserwire.WindowParams{XID: otherWindow(t)}, nil)
			}
			c.open("b1", keysPage, 400, 300, 1)
			c.eval("b1", "a.focus(), a.setSelectionRange(2, 2), 0")
			for _, kind := range []string{browserwire.KeyDown, browserwire.KeyUp} {
				c.must(browserwire.Key, browserwire.KeyParams{Page: "b1", Kind: kind, Key: tc.key, Code: tc.key}, nil)
			}
			if got := c.eval("b1", tc.check); got != tc.want {
				t.Fatalf("after %s, %s is %s, want %s; keys %s", tc.key, tc.check, got, tc.want, c.eval("b1", "keys"))
			}
		})
	}
}

// otherWindow is the X window of another lich-browser's tab.
func otherWindow(t *testing.T) uint64 {
	t.Helper()
	other := start(t)
	other.open("w", "data:text/html,", 200, 100, 1)
	raw, _ := other.event(0, browserwire.EventTabs, nil)
	var tabs browserwire.TabsEvent
	if err := json.Unmarshal(raw, &tabs); err != nil || len(tabs.Tabs) == 0 || tabs.Tabs[0].XID == 0 {
		t.Fatalf("the other window's tabs %s", raw)
	}
	return tabs.Tabs[0].XID
}

func TestNativeInputTypesIntoAnInputAndIsTrusted(t *testing.T) {
	c := start(t)
	s := site(t)
	c.open("b1", s.URL, 400, 300, 1)
	for _, kind := range []string{browserwire.MouseMove, browserwire.MouseDown, browserwire.MouseUp} {
		c.must(browserwire.Mouse, browserwire.MouseParams{Page: "b1", Kind: kind, X: 50, Y: 25, Clicks: 1}, nil)
	}
	for _, kind := range []string{browserwire.KeyDown, browserwire.KeyUp} {
		c.must(browserwire.Key, browserwire.KeyParams{Page: "b1", Kind: kind, Key: "a", Code: "KeyA"}, nil)
	}
	c.must(browserwire.Text, browserwire.TextParams{Page: "b1", Text: "bç"}, nil)
	c.must(browserwire.Key, browserwire.KeyParams{Page: "b1", Kind: browserwire.KeyDown, Key: "Backspace"}, nil)
	if got := c.eval("b1", "field.value"); got != `"ab"` {
		t.Fatalf("the input holds %s, want the key's a and the text's b, its ç taken back by Backspace; events %s", got, c.eval("b1", "JSON.stringify(got)"))
	}
	var got []struct {
		Type    string
		Trusted bool
	}
	json.Unmarshal([]byte(c.eval("b1", "got")), &got)
	seen := map[string]bool{}
	for _, e := range got {
		if !e.Trusted {
			t.Errorf("the page got an untrusted %s", e.Type)
		}
		seen[e.Type] = true
	}
	for _, want := range []string{"mousedown", "click", "keydown", "beforeinput", "input"} {
		if !seen[want] {
			t.Errorf("the page got no %s: %+v", want, got)
		}
	}
}

func TestTheWheelScrollsThePageByItsCSSPixels(t *testing.T) {
	for _, tc := range []struct {
		name  string
		h     int
		scale float64
	}{{"a short view", 300, 1}, {"a tall view", 600, 1}, {"a view drawn at 2x", 300, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			c := start(t)
			c.open("b1", "data:text/html,<body style='height:5000px'>", 400, tc.h, tc.scale)
			c.must(browserwire.Mouse, browserwire.MouseParams{Page: "b1", Kind: browserwire.MouseWheel, X: 100, Y: 100, DY: 200}, nil)
			if got := c.eval("b1", "new Promise(r => setTimeout(() => r(scrollY), 300))"); got != "200" {
				t.Fatalf("the page scrolled to %s, want 200", got)
			}
		})
	}
}

func TestADialogWaitsForItsAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, open, kind, text, want string
		accept                       bool
	}{
		{"alert", `alert("hi")`, browserwire.DialogAlert, "", "undefined", true},
		{"confirm accepted", `confirm("sure?")`, browserwire.DialogConfirm, "", "true", true},
		{"confirm dismissed", `confirm("sure?")`, browserwire.DialogConfirm, "", "false", false},
		{"prompt answered", `prompt("name?", "ann")`, browserwire.DialogPrompt, "bob", `"bob"`, true},
		{"prompt dismissed", `prompt("name?", "ann")`, browserwire.DialogPrompt, "", "null", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := start(t)
			c.open("b1", "data:text/html,", 400, 300, 1)
			from := c.mark()
			c.eval("b1", "setTimeout(() => { window.answer = String(JSON.stringify("+tc.open+")); }), 0")
			raw, next := c.event(from, browserwire.EventDialog, nil)
			var d browserwire.DialogEvent
			json.Unmarshal(raw, &d)
			if d.Dialog == nil || d.Dialog.Type != tc.kind {
				t.Fatalf("dialog event %s, want a %s", raw, tc.kind)
			}
			c.must(browserwire.Dialog, browserwire.DialogParams{Page: "b1", Accept: tc.accept, Text: tc.text}, nil)
			raw, _ = c.event(next, browserwire.EventDialog, nil)
			if json.Unmarshal(raw, &d); d.Dialog != nil {
				t.Fatalf("after the answer the dialog is %s, want null", raw)
			}
			if got := c.eval("b1", "window.answer"); got != `"`+strings.ReplaceAll(tc.want, `"`, `\"`)+`"` {
				t.Fatalf("the page got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestAWindowOpenedByThePageIsATabInFront(t *testing.T) {
	c := start(t)
	s := site(t)
	c.open("b1", s.URL, 400, 300, 1)
	from := c.mark()
	c.eval("b1", "open('/images'), 0")
	var tabs browserwire.TabsEvent
	c.event(from, browserwire.EventTabs, func(raw json.RawMessage) bool {
		return json.Unmarshal(raw, &tabs) == nil && len(tabs.Tabs) == 2 && strings.HasSuffix(tabs.Tabs[1].URL, "/images")
	})
	first, second := tabs.Tabs[0], tabs.Tabs[1]
	if !second.Front || first.Front || first.XID == 0 || second.XID == 0 || first.XID == second.XID {
		t.Fatalf("tabs %+v, want the opened one in front, each its own X window", tabs.Tabs)
	}
	c.must(browserwire.SelectTab, browserwire.TabParams{Page: "b1", Tab: first.ID}, nil)
	if got := c.eval("b1", "document.title"); got != `"Form"` {
		t.Fatalf("after selecting the first tab eval reads %s", got)
	}
	c.must(browserwire.CloseTab, browserwire.TabParams{Page: "b1", Tab: second.ID}, nil)
	if err := c.call(browserwire.CloseTab, browserwire.TabParams{Page: "b1", Tab: first.ID}, nil); err == nil {
		t.Fatal("the last tab closed: a page keeps one")
	}
}

func TestATabCarriesItsTitleAndIcon(t *testing.T) {
	c := start(t)
	s := site(t)
	c.open("b1", s.URL, 400, 300, 1)
	var tabs browserwire.TabsEvent
	c.event(0, browserwire.EventTabs, func(raw json.RawMessage) bool {
		return json.Unmarshal(raw, &tabs) == nil && len(tabs.Tabs) == 1 && len(tabs.Tabs[0].Icon) > 0
	})
	if _, err := png.Decode(bytes.NewReader(tabs.Tabs[0].Icon)); err != nil {
		t.Fatalf("the icon is no PNG: %v", err)
	}
	if tabs.Tabs[0].Title != "Form" {
		t.Fatalf("title %q, want Form", tabs.Tabs[0].Title)
	}
}

func TestADownloadLandsAndIsToldStepByStep(t *testing.T) {
	c := start(t)
	s := site(t)
	c.open("b1", s.URL, 400, 300, 1)
	from := c.mark()
	c.must(browserwire.Navigate, browserwire.NavigateParams{Page: "b1", URL: s.URL + "/file"}, nil)
	c.event(from, browserwire.EventLoad, func(raw json.RawMessage) bool {
		var l browserwire.LoadEvent
		return json.Unmarshal(raw, &l) == nil && l.Step == browserwire.LoadFailed && l.Download
	})
	var d browserwire.DownloadEvent
	c.event(from, browserwire.EventDownload, func(raw json.RawMessage) bool {
		return json.Unmarshal(raw, &d) == nil && d.State != browserwire.DownloadInProgress
	})
	if d.State != browserwire.DownloadCompleted || d.Name != "notes.txt" || filepath.Base(d.Path) != "notes.txt" || d.Page != "b1" {
		t.Fatalf("download %+v, want notes.txt completed", d)
	}
	if got, err := os.ReadFile(d.Path); err != nil || string(got) != downloadBody {
		t.Fatalf("the file holds %q, %v; want the served bytes", got, err)
	}
}

func TestTheConsoleTellsErrorsAndWhatNothingCaught(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,<body>", 400, 300, 1)
	for _, tc := range []struct{ name, script, want string }{
		{"console.error", `console.error("boom", {a: 1}), 0`, `boom {"a":1}`},
		// The page's own script: one the wire ran is another origin's.
		{"an uncaught throw", `document.body.append(Object.assign(document.createElement("script"), {textContent: "setTimeout(() => { throw new RangeError('late'); })"})), 0`, "Uncaught RangeError: late"},
		{"an unhandled rejection", `Promise.reject(new Error("no")), setTimeout(() => {}), 0`, "Uncaught (in promise) Error: no"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from := c.mark()
			c.eval("b1", tc.script)
			c.event(from, browserwire.EventConsole, func(raw json.RawMessage) bool {
				var e browserwire.ConsoleEvent
				return json.Unmarshal(raw, &e) == nil && strings.HasPrefix(e.Text, tc.want)
			})
		})
	}
}

func TestEachRequestOfTheFrontTabIsToldWithItsTypeAndFinish(t *testing.T) {
	c := start(t)
	s := site(t)
	c.open("b1", s.URL+"/images", 400, 300, 1)
	started, finished := map[string]string{}, map[string]bool{}
	from := 0
	for len(started) < 3 || len(finished) < len(started) {
		raw, next := c.event(from, browserwire.EventRequest, nil)
		from = next
		var r browserwire.RequestEvent
		json.Unmarshal(raw, &r)
		if r.Started {
			started[r.ID] = r.Type
		} else if _, ok := started[r.ID]; !ok || !r.Finished {
			t.Fatalf("request event %s names no request that started", raw)
		} else {
			finished[r.ID] = true
		}
	}
	types := map[string]int{}
	for _, kind := range started {
		types[kind]++
	}
	if types[browserwire.RequestDocument] != 1 || types[browserwire.RequestImage] < 2 {
		t.Fatalf("requests by type %v, want the page's document and both its images", types)
	}
}

func TestALoadThatFailsSaysWhy(t *testing.T) {
	c := start(t)
	s := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(s.Close)
	c.open("b1", "data:text/html,", 400, 300, 1)
	from := c.mark()
	c.must(browserwire.Navigate, browserwire.NavigateParams{Page: "b1", URL: s.URL}, nil)
	var l browserwire.LoadEvent
	c.event(from, browserwire.EventLoad, func(raw json.RawMessage) bool {
		return json.Unmarshal(raw, &l) == nil && l.Step == browserwire.LoadFailed
	})
	if l.Error == "" || l.Download {
		t.Fatalf("a load refused for its certificate failed with %+v, want WebKit's reason and no download", l)
	}
}

func TestAPagePopsOutAndBackWithoutReloading(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,<title>Kept</title>", 400, 300, 1)
	c.eval("b1", "window.kept = 42")
	for _, step := range []struct {
		method string
		popped bool
	}{{browserwire.PopOut, true}, {browserwire.Raise, true}, {browserwire.PopIn, false}} {
		from := c.mark()
		c.must(step.method, browserwire.PageParams{Page: "b1"}, nil)
		if step.method != browserwire.Raise {
			var tabs browserwire.TabsEvent
			raw, _ := c.event(from, browserwire.EventTabs, nil)
			json.Unmarshal(raw, &tabs)
			if out := tabs.Tabs[0].XID == 0; out != step.popped {
				t.Fatalf("after %s the tab's xid is %d, want it 0 only while out", step.method, tabs.Tabs[0].XID)
			}
		}
		if got := c.eval("b1", "window.kept"); got != "42" {
			t.Fatalf("after %s the page holds %s: it reloaded", step.method, got)
		}
	}
	if err := c.call(browserwire.PopIn, browserwire.PageParams{Page: "b1"}, nil); err == nil {
		t.Fatal("a page not out popped in")
	}
}

func TestTheWindowsLayerDrawsTheLatestAndABareSnapshotLeavesItOut(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,<body style='margin:0;background:rgb(255,255,255)'>", 200, 100, 1)
	layer := func(color string) string {
		return `(() => { let h = document.getElementById('` + browserwire.LayerID + `');
			if (!h) { h = document.createElement('div'); h.id = '` + browserwire.LayerID + `';
				h.style.cssText = 'position:fixed;inset:0'; document.documentElement.appendChild(h); }
			h.style.background = '` + color + `'; })()`
	}
	for _, color := range []string{"rgb(0,0,255)", "rgb(0,255,0)", "rgb(255,0,0)"} {
		c.must(browserwire.Draw, browserwire.DrawParams{Page: "b1", Script: layer(color)}, nil)
	}
	got := c.eval("b1", "new Promise(r => setTimeout(() => r(getComputedStyle(document.getElementById('"+browserwire.LayerID+"')).backgroundColor), 200))")
	if got != `"rgb(255, 0, 0)"` {
		t.Fatalf("the layer is %s, want the last drawing's red", got)
	}
	middle := func(bare bool) (r, g, b uint32) {
		var s browserwire.SnapshotResult
		c.must(browserwire.Snapshot, browserwire.SnapshotParams{Page: "b1", Bare: bare}, &s)
		img, err := png.Decode(bytes.NewReader(s.PNG))
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ = img.At(100, 50).RGBA()
		return r >> 8, g >> 8, b >> 8
	}
	if r, g, b := middle(true); r != 0xff || g != 0xff || b != 0xff {
		t.Fatalf("a bare snapshot shows %d,%d,%d, want the page's white under the layer", r, g, b)
	}
	if r, g, b := middle(false); r != 0xff || g != 0 || b != 0 {
		t.Fatalf("a snapshot shows %d,%d,%d, want the layer's red", r, g, b)
	}
	if got := c.eval("b1", "document.getElementById('"+browserwire.LayerID+"').style.visibility"); got != `""` {
		t.Fatalf("after a bare snapshot the layer's visibility is %s, want it shown again", got)
	}
}

// tabsWhere waits for the page's tabs to match ok, from index from on.
func (c *client) tabsWhere(from int, ok func([]browserwire.Tab) bool) []browserwire.Tab {
	c.t.Helper()
	var ev browserwire.TabsEvent
	c.event(from, browserwire.EventTabs, func(raw json.RawMessage) bool {
		return json.Unmarshal(raw, &ev) == nil && ok(ev.Tabs)
	})
	return ev.Tabs
}

// WebKit's inspector is a tab of the page's, as Chrome's DevTools are in the
// pane, never a window of its own; every request still acts on the page.
func TestTheInspectorIsATabAfterThePage(t *testing.T) {
	c := start(t)
	c.open("b1", "data:text/html,<title>Inspected</title>", 400, 300, 1)
	from := c.mark()
	c.must(browserwire.Inspect, browserwire.PageParams{Page: "b1"}, nil)
	tabs := c.tabsWhere(from, func(tabs []browserwire.Tab) bool { return len(tabs) == 2 })
	page, tools := tabs[0], tabs[1]
	if !tools.DevTools || !tools.Front || page.Front || tools.XID == 0 || tools.XID == page.XID {
		t.Fatalf("tabs %+v: want the inspector after the page, in front, in an X window of its own", tabs)
	}
	if got := c.eval("b1", "document.title"); got != `"Inspected"` {
		t.Fatalf("with the inspector in front eval read %s, want the page's title", got)
	}
	from = c.mark()
	c.must(browserwire.SelectTab, browserwire.TabParams{Page: "b1", Tab: page.ID}, nil)
	c.tabsWhere(from, func(tabs []browserwire.Tab) bool { return len(tabs) == 2 && tabs[0].Front && !tabs[1].Front })
	from = c.mark()
	c.must(browserwire.Inspect, browserwire.PageParams{Page: "b1"}, nil)
	c.tabsWhere(from, func(tabs []browserwire.Tab) bool { return len(tabs) == 2 && tabs[1].Front })
	from = c.mark()
	c.must(browserwire.CloseTab, browserwire.TabParams{Page: "b1", Tab: tools.ID}, nil)
	tabs = c.tabsWhere(from, func(tabs []browserwire.Tab) bool { return len(tabs) == 1 })
	if !tabs[0].Front || tabs[0].DevTools {
		t.Fatalf("after closing the inspector, tabs %+v: want the page alone in front", tabs)
	}
}

func TestARequestForAPageNotOpenFails(t *testing.T) {
	c := start(t)
	if err := c.call(browserwire.Eval, browserwire.EvalParams{Page: "b9", Script: "1"}, nil); err == nil || !strings.Contains(err.Error(), "b9") {
		t.Fatalf("eval on a page never opened answered %v, want an error naming it", err)
	}
}
