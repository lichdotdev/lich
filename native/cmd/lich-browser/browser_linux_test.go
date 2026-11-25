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
		{"statements, by their completion value", "const a = 1; a + 1", "2", ""},
		{"a promise, awaited", "new Promise(r => setTimeout(() => r([1, 2]), 10))", "[1,2]", ""},
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

func TestSubresourcesOfTheFrontTabAreToldForSettle(t *testing.T) {
	c := start(t)
	s := site(t)
	from := c.mark()
	c.open("b1", s.URL+"/images", 400, 300, 1)
	var started, finished int
	for range 2 {
		_, from = c.event(from, browserwire.EventRequest, func(raw json.RawMessage) bool {
			var r browserwire.RequestEvent
			json.Unmarshal(raw, &r)
			if r.Started {
				started++
			}
			return r.Started
		})
	}
	c.event(from, browserwire.EventRequest, func(raw json.RawMessage) bool {
		var r browserwire.RequestEvent
		json.Unmarshal(raw, &r)
		if r.Finished {
			finished++
		}
		return r.Finished
	})
	if started < 2 || finished < 1 {
		t.Fatalf("%d starts and %d finishes, want both images' starts and a finish", started, finished)
	}
}

func TestARequestForAPageNotOpenFails(t *testing.T) {
	c := start(t)
	if err := c.call(browserwire.Eval, browserwire.EvalParams{Page: "b9", Script: "1"}, nil); err == nil || !strings.Contains(err.Error(), "b9") {
		t.Fatalf("eval on a page never opened answered %v, want an error naming it", err)
	}
}
