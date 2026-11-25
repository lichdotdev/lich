package browser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/browserwire"
)

// fakeLichBrowser is lich-browser at the wire: it reads the backend's
// requests off a pipe, answers them as the real one would from a page it
// keeps in memory, and sends events, its own and the test's.
type fakeLichBrowser struct {
	t       *testing.T
	version int
	// eval answers a script the test cares about: its JSON, or the error.
	eval func(script string) (json, err string, ok bool)
	// failURL fails its navigation; downloadURL turns its navigation into a
	// download, as WebKit fails one that became a download.
	failURL, downloadURL string
	// snapW and snapH are the snapshot's viewport, drawn at snapScale.
	snapW, snapH int
	snapScale    float64
	// icon is every tab's icon.
	icon []byte

	mu     sync.Mutex
	spawns int
	out    io.WriteCloser
	in     io.ReadCloser
	calls  []browserwire.Request
	pages  map[string]*fakeWirePage
	clock  int64
	ntab   int
}

type fakeWirePage struct {
	tabs  []browserwire.Tab
	front int
}

func newFakeLichBrowser(t *testing.T) *fakeLichBrowser {
	return &fakeLichBrowser{t: t, version: browserwire.Version, pages: map[string]*fakeWirePage{}, snapW: 800, snapH: 600, snapScale: 1}
}

// wkService is a service holding its pages in f, through the wiring New
// uses.
func wkService(t *testing.T, f *fakeLichBrowser) (*Service, *emitted) {
	t.Helper()
	s := testService(t, &fakeLauncher{})
	out := &emitted{}
	s.emit = out.emit
	s.useLichBrowser(&lichBrowser{spawn: f.spawn})
	return s, out
}

func (f *fakeLichBrowser) spawn() (*wireConn, error) {
	toFake, fromBackend := io.Pipe()
	toBackend, fromFake := io.Pipe()
	f.mu.Lock()
	f.spawns++
	f.out, f.in = fromFake, toFake
	f.mu.Unlock()
	go f.serve(toFake, fromFake)
	stop := func() { _ = fromBackend.Close(); _ = toBackend.Close() }
	return newWireConn(toBackend, fromBackend, stop), nil
}

// kill ends the process under the backend.
func (f *fakeLichBrowser) kill() {
	f.mu.Lock()
	out, in := f.out, f.in
	f.pages = map[string]*fakeWirePage{}
	f.mu.Unlock()
	_ = out.Close()
	_ = in.Close()
}

func (f *fakeLichBrowser) serve(in io.Reader, out io.WriteCloser) {
	r := bufio.NewReader(in)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		var req browserwire.Request
		if err := json.Unmarshal(line, &req); err != nil {
			f.t.Errorf("a request that is no JSON: %q", line)
			return
		}
		f.mu.Lock()
		f.calls = append(f.calls, req)
		f.mu.Unlock()
		result, errText := f.answer(out, req)
		reply := map[string]any{"id": req.ID}
		if errText != "" {
			reply["error"] = errText
		} else if result != nil {
			reply["result"] = result
		}
		f.write(out, reply)
	}
}

func (f *fakeLichBrowser) write(out io.Writer, msg any) {
	line, err := json.Marshal(msg)
	if err != nil {
		f.t.Error(err)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = out.Write(append(line, '\n'))
}

// send is an event from the running process.
func (f *fakeLichBrowser) send(event string, params any) {
	f.mu.Lock()
	out := f.out
	f.mu.Unlock()
	f.write(out, map[string]any{"event": event, "params": params})
}

func (f *fakeLichBrowser) answer(out io.Writer, req browserwire.Request) (any, string) {
	var pp struct {
		Page   string `json:"page"`
		URL    string `json:"url"`
		Tab    string `json:"tab"`
		Script string `json:"script"`
	}
	_ = json.Unmarshal(req.Params, &pp)
	switch req.Method {
	case browserwire.Hello:
		return browserwire.HelloResult{Version: f.version}, ""
	case browserwire.Clock:
		f.mu.Lock()
		f.clock += int64(time.Millisecond)
		ns := f.clock
		f.mu.Unlock()
		return browserwire.ClockResult{NS: ns}, ""
	case browserwire.Eval:
		return f.evalAnswer(pp.Script)
	case browserwire.Snapshot:
		return f.snapshot(), ""
	case browserwire.Open, browserwire.NewTab, browserwire.SelectTab, browserwire.CloseTab, browserwire.Close:
		return nil, f.tabRequest(out, req.Method, pp.Page, pp.URL, pp.Tab)
	case browserwire.Navigate, browserwire.Reload, browserwire.Back, browserwire.Forward:
		f.loadPage(out, pp.Page, pp.URL)
	case browserwire.Dialog:
		f.write(out, map[string]any{"event": browserwire.EventDialog, "params": browserwire.DialogEvent{Page: pp.Page}})
	}
	return nil, ""
}

func (f *fakeLichBrowser) evalAnswer(script string) (any, string) {
	if f.eval != nil {
		if j, e, ok := f.eval(script); ok {
			return browserwire.EvalResult{JSON: j}, e
		}
	}
	switch {
	case script == "document.readyState":
		return browserwire.EvalResult{JSON: `"complete"`}, ""
	case strings.HasPrefix(script, "((quiet, max)"):
		return browserwire.EvalResult{JSON: "true"}, ""
	case strings.HasPrefix(script, pageInfoScript):
		return browserwire.EvalResult{JSON: `{"url":"https://example.com/","title":"Example","text":"hello","interactive":[],"next":3}`}, ""
	}
	return browserwire.EvalResult{}, ""
}

func (f *fakeLichBrowser) snapshot() browserwire.SnapshotResult {
	w, h := int(float64(f.snapW)*f.snapScale), int(float64(f.snapH)*f.snapScale)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		f.t.Error(err)
	}
	return browserwire.SnapshotResult{PNG: buf.Bytes(), W: f.snapW, H: f.snapH}
}

// tabRequest runs a request on the page's tabs and sends them, as
// lich-browser sends tabs on every change.
func (f *fakeLichBrowser) tabRequest(out io.Writer, method, id, url, tab string) string {
	f.mu.Lock()
	p := f.pages[id]
	switch {
	case method == browserwire.Open:
		p = &fakeWirePage{}
		f.pages[id] = p
		f.addTabLocked(p, url)
	case p == nil:
		f.mu.Unlock()
		return fmt.Sprintf("page %s is not open", id)
	case method == browserwire.Close:
		delete(f.pages, id)
		f.mu.Unlock()
		return ""
	case method == browserwire.NewTab:
		f.addTabLocked(p, "about:blank")
	case method == browserwire.SelectTab:
		p.front = p.indexOf(tab)
	case method == browserwire.CloseTab:
		i := p.indexOf(tab)
		p.tabs = append(p.tabs[:i], p.tabs[i+1:]...)
		p.front = 0
	}
	ev := f.tabsLocked(id, p)
	f.mu.Unlock()
	f.write(out, map[string]any{"event": browserwire.EventTabs, "params": ev})
	return ""
}

func (f *fakeLichBrowser) addTabLocked(p *fakeWirePage, url string) {
	f.ntab++
	p.tabs = append(p.tabs, browserwire.Tab{ID: "t" + strconv.Itoa(f.ntab), URL: url, XID: uint64(0x400000 + f.ntab)})
	p.front = len(p.tabs) - 1
}

func (p *fakeWirePage) indexOf(tab string) int {
	for i, t := range p.tabs {
		if t.ID == tab {
			return i
		}
	}
	return 0
}

func (f *fakeLichBrowser) tabsLocked(id string, p *fakeWirePage) browserwire.TabsEvent {
	tabs := make([]browserwire.Tab, len(p.tabs))
	for i, t := range p.tabs {
		t.Front, t.Icon = i == p.front, f.icon
		tabs[i] = t
	}
	return browserwire.TabsEvent{Page: id, Tabs: tabs}
}

// loadPage sends the front tab's load, failed for failURL and downloadURL,
// before it answers the request that started it.
func (f *fakeLichBrowser) loadPage(out io.Writer, id, url string) {
	f.mu.Lock()
	p := f.pages[id]
	if p == nil {
		f.mu.Unlock()
		return
	}
	tab := p.tabs[p.front].ID
	f.clock += int64(time.Millisecond)
	at := f.clock
	f.mu.Unlock()
	load := func(step string) {
		f.write(out, map[string]any{"event": browserwire.EventLoad, "params": browserwire.LoadEvent{Page: id, Tab: tab, Step: step, URL: url, At: at}})
	}
	load(browserwire.LoadStarted)
	if url != "" && (url == f.failURL || url == f.downloadURL) {
		if url == f.downloadURL {
			f.write(out, map[string]any{"event": browserwire.EventDownload, "params": browserwire.DownloadEvent{Page: id, GUID: "g1", URL: url, Name: "x.zip", State: browserwire.DownloadInProgress}})
		}
		load(browserwire.LoadFailed)
		return
	}
	f.mu.Lock()
	if url != "" {
		p.tabs[p.front].URL, p.tabs[p.front].Title = url, "Title of "+url
	}
	ev := f.tabsLocked(id, p)
	f.mu.Unlock()
	load(browserwire.LoadCommitted)
	f.write(out, map[string]any{"event": browserwire.EventTabs, "params": ev})
	load(browserwire.LoadFinished)
}

// called is the params of every request of method, in order.
func (f *fakeLichBrowser) called(method string) []json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []json.RawMessage
	for _, c := range f.calls {
		if c.Method == method {
			out = append(out, c.Params)
		}
	}
	return out
}

// onlyPage is the id of the one page open.
func (f *fakeLichBrowser) onlyPage() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := range f.pages {
		return id
	}
	f.t.Fatal("no page open in lich-browser")
	return ""
}

// waitFor polls ok until it holds, failing the test after a second.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func pngOf(t *testing.T, side int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, side, side))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
