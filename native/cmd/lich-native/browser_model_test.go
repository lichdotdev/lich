package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func TestOnlyANewAgentBrowserIsOffered(t *testing.T) {
	m, _ := newLayoutsModel(t)
	var offered []string
	m.browserOpened = func(st api.BrowserState) { offered = append(offered, st.Owner) }
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true})
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "wa", Open: true, ByAgent: true})
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "wa", Open: true, ByAgent: true, URL: "https://example.com"})
	if len(offered) != 1 || offered[0] != "wa" {
		t.Fatalf("offered %v, want the agent's new browser once", offered)
	}
}

func TestIdleQuestionGoesUpAndDownOnlyOnChange(t *testing.T) {
	m, _ := newLayoutsModel(t)
	var heard []api.BrowserState
	m.browserIdle = func(st api.BrowserState) { heard = append(heard, st) }
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true})
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true, Idle: true})
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: true, Idle: true, Title: "reloaded"})
	send(m, api.BrowserStateEvent.Name, api.BrowserState{Owner: "orch", Open: false, Idle: true})
	if len(heard) != 2 || !heard[0].Idle || heard[1].Open {
		t.Fatalf("heard %+v, want the question raised once and taken down with the browser", heard)
	}
}

func TestIdleBrowserToastKeepsOnSwipe(t *testing.T) {
	kept, closed := 0, 0
	toast := idleBrowserToast(api.BrowserState{Owner: "s", URL: "http://localhost:5173/settings"}, "Claude 3", "skipo",
		func() { kept++ }, func() { closed++ })
	if toast.Duration != ui.ToastForever || toast.ID != idleBrowserToastID("s") {
		t.Fatalf("toast %+v, want it up until answered under the session's id", toast)
	}
	toast.OnDismiss()
	toast.Cancel.OnClick()
	toast.Action.OnClick()
	if kept != 2 || closed != 1 {
		t.Fatalf("kept %d, closed %d: want swipe and Keep open to keep, Close to close", kept, closed)
	}
	if len(toast.Lines) != 2 || toast.Lines[0].Text != "http://localhost:5173/settings" || toast.Lines[1].Text != "skipo" {
		t.Fatalf("lines %+v, want the page and the project", toast.Lines)
	}
}

func TestBrowserFrameIsDecodedForThePane(t *testing.T) {
	m, _ := newLayoutsModel(t)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 6)), nil); err != nil {
		t.Fatal(err)
	}
	send(m, api.BrowserFrameEvents.Of("orch").Name, api.BrowserFrame{Data: base64.StdEncoding.EncodeToString(buf.Bytes())})
	if _, frame, _ := m.browserView("orch"); frame.size != image.Pt(8, 6) {
		t.Fatalf("frame size %v", frame.size)
	}
}

func TestBrowserFramesDecodeTwoAtOnce(t *testing.T) {
	m, _ := newLayoutsModel(t)
	kick := make(chan struct{}, frameDecoders)
	decoded := m.decodeFrames(kick)
	name := api.BrowserFrameEvents.Of("orch").Name
	m.mu.Lock()
	for range 2 {
		if !takenWhileDecoding(m, jpegFrame(t, name, 8), kick) {
			m.mu.Unlock()
			t.Fatal("a frame waited for a busy decoder")
		}
	}
	m.mu.Unlock()
	close(kick)
	<-decoded
}

func TestBrowserFrameSurplusDecodesOnlyTheNewest(t *testing.T) {
	m, _ := newLayoutsModel(t)
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	name := api.BrowserFrameEvents.Of("orch").Name
	kick := make(chan struct{}, frameDecoders)
	decoded := m.decodeFrames(kick)
	m.mu.Lock()
	for range frameDecoders {
		if !takenWhileDecoding(m, jpegFrame(t, name, 8), kick) {
			m.mu.Unlock()
			t.Fatal("a frame waited for a busy decoder while another was free")
		}
	}
	newest := jpegFrame(t, name, 9)
	offered := make(chan struct{})
	go func() {
		// A stale frame that fails to decode: decoding it would log.
		m.offerBrowserFrame("orch", backend.Event{Name: name, Data: json.RawMessage(`{"data":"!!"}`)}, kick)
		m.offerBrowserFrame("orch", newest, kick)
		close(offered)
	}()
	select {
	case <-offered:
	case <-time.After(2 * time.Second):
		t.Error("offering a frame waited on a decode")
	}
	m.mu.Unlock()
	<-offered
	close(kick)
	<-decoded
	if _, f, _ := m.browserView("orch"); f.size != image.Pt(9, 6) {
		t.Fatalf("frame size %v, want the newest", f.size)
	}
	// Decoding the stale frame logs under the event's name. The log is the
	// process's: a layout save of an earlier test, failing after that test's
	// backend closed, lands here too and is not this test's to judge.
	if strings.Contains(logged.String(), name) {
		t.Fatalf("decoded a frame a newer one replaced: %s", logged.String())
	}
}

func TestOlderBrowserFrameDecodedLastKeepsTheNewer(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	if err := m.applyBrowserFrame("orch", jpegFrame(t, name, 9), 2); err != nil {
		t.Fatal(err)
	}
	if err := m.applyBrowserFrame("orch", jpegFrame(t, name, 8), 1); err != nil {
		t.Fatal(err)
	}
	if _, f, _ := m.browserView("orch"); f.size != image.Pt(9, 6) {
		t.Fatalf("frame size %v, want the newer frame's", f.size)
	}
}

func TestAStillPNGFrameReplacesTheJPEG(t *testing.T) {
	m, _ := newLayoutsModel(t)
	name := api.BrowserFrameEvents.Of("orch").Name
	if err := m.applyBrowserFrame("orch", tabFrame(t, name, "t1", 9), 1); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 7, 6))); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(api.BrowserFrame{Tab: "t1", Data: base64.StdEncoding.EncodeToString(buf.Bytes())})
	if err := m.applyBrowserFrame("orch", backend.Event{Name: name, Data: data}, 2); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	f := m.browsers.frames["orch"]["t1"]
	m.mu.Unlock()
	if f.size != image.Pt(7, 6) {
		t.Fatalf("tab t1's frame size %v, want the PNG's", f.size)
	}
}

// takenWhileDecoding offers frame with m.mu held, which keeps each decoder
// that takes one waiting to keep it, as a decode slower than the page sends
// frames would; it reports whether a decoder took it.
func takenWhileDecoding(m *model, frame backend.Event, kick chan<- struct{}) bool {
	m.offerBrowserFrame("orch", frame, kick)
	deadline := time.Now().Add(2 * time.Second)
	for pendingFrames(m) > 0 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	return pendingFrames(m) == 0
}

// jpegFrame is a frame event of a w by 6 page, of no tab in particular.
func jpegFrame(t *testing.T, name string, w int) backend.Event {
	t.Helper()
	return tabFrame(t, name, "", w)
}

// tabFrame is a frame event of tab, a w by 6 page.
func tabFrame(t *testing.T, name, tab string, w int) backend.Event {
	t.Helper()
	data, _ := json.Marshal(tabPayload(t, tab, w))
	return backend.Event{Name: name, Data: data}
}

func tabPayload(t *testing.T, tab string, w int) api.BrowserFrame {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, 6)), nil); err != nil {
		t.Fatal(err)
	}
	return api.BrowserFrame{Tab: tab, Data: base64.StdEncoding.EncodeToString(buf.Bytes())}
}

func pendingFrames(m *model) int {
	m.browsers.pendingMu.Lock()
	defer m.browsers.pendingMu.Unlock()
	return len(m.browsers.pending)
}

func TestTypedURL(t *testing.T) {
	cases := map[string]string{
		"":                       "about:blank",
		"localhost:3000/a":       "http://localhost:3000/a",
		"app.localhost":          "http://app.localhost",
		"127.0.0.1:8080":         "http://127.0.0.1:8080",
		"example.com":            "https://example.com",
		"http://example.com/x":   "http://example.com/x",
		"https://example.com":    "https://example.com",
		"example.com:8443/login": "https://example.com:8443/login",
	}
	page := filepath.Join(t.TempDir(), "index.html")
	cases[page] = page
	for in, want := range cases {
		if got := typedURL(in); got != want {
			t.Errorf("typedURL(%q) = %q, want %q", in, got, want)
		}
	}
	if displayURL("about:blank") != "" || displayURL("https://x.dev") != "https://x.dev" {
		t.Fatal("displayURL")
	}
}

func TestALayoutOfHomeSessionsComesOnScreen(t *testing.T) {
	m, _ := newLayoutsModel(t)
	m.projects = append(m.projects, backend.Project{ID: "home", Name: "~", Path: "/home/u", Sessions: []backend.Session{{ID: "h1", Label: "home one"}, {ID: "h2", Label: "home two"}}})
	m.homeID = "home"
	ctx := context.Background()
	if !m.showPage(ctx, "h1") {
		t.Fatal("refused a Home session")
	}
	if lt, ok := m.shownLayoutTab(); !ok || paneShape(lt.Root) != "row(h1,browser:h1)" {
		t.Fatalf("on screen %+v: a Home session's page stayed off screen", lt)
	}
	if !m.splitBeside(ctx, "h2", "notes", edgeRight, false) {
		t.Fatal("split refused")
	}
	if id, _, _ := layoutScreen(t, m); id == "" {
		t.Fatal("a split holding a Home session stayed off screen")
	}
}

func TestAgentActionsBetweenTwoFramesAreAllKeptInOrder(t *testing.T) {
	m, _ := newLayoutsModel(t)
	x, y := 10.0, 20.0
	send(m, api.BrowserAgentEvent.Name, api.BrowserAgentAct{Owner: "orch", Action: "click", X: &x, Y: &y})
	send(m, api.BrowserAgentEvent.Name, api.BrowserAgentAct{Owner: "orch", Action: "scroll", X: &x, Y: &y, DY: 500})
	for range agentMarksKept {
		send(m, api.BrowserAgentEvent.Name, api.BrowserAgentAct{Owner: "orch"})
	}
	_, _, marks := m.browserView("orch")
	if len(marks) != agentMarksKept {
		t.Fatalf("kept %d marks, want the latest %d", len(marks), agentMarksKept)
	}
	for i := 1; i < len(marks); i++ {
		if marks[i].seq <= marks[i-1].seq {
			t.Fatalf("marks out of order: %+v", marks)
		}
	}
	m2, _ := newLayoutsModel(t)
	send(m2, api.BrowserAgentEvent.Name, api.BrowserAgentAct{Owner: "orch", Action: "click", X: &x, Y: &y})
	send(m2, api.BrowserAgentEvent.Name, api.BrowserAgentAct{Owner: "orch", Action: "scroll", X: &x, Y: &y, DY: 500})
	if _, _, two := m2.browserView("orch"); len(two) != 2 || two[0].action != "click" || two[1].action != "scroll" {
		t.Fatalf("marks %+v, want the click then the scroll", two)
	}
}

func TestADownloadReachesTheWindowAsAToastNamingItsFolder(t *testing.T) {
	m, _ := newLayoutsModel(t)
	var heard []api.BrowserDownload
	m.browserDownloaded = func(d api.BrowserDownload) { heard = append(heard, d) }
	path := filepath.Join("home", "u", "Downloads", "report.csv")
	send(m, api.BrowserDownloadEvent.Name, api.BrowserDownload{Owner: "orch", Path: path, Bytes: 12, State: api.DownloadCompleted})
	if len(heard) != 1 || heard[0].Path != path {
		t.Fatalf("heard %+v, want the one download", heard)
	}
	var opened string
	toast := downloadToast(heard[0], func(dir string) { opened = dir })
	dir := filepath.Join("home", "u", "Downloads")
	if toast.Title != "Downloaded report.csv" || len(toast.Lines) != 1 || toast.Lines[0].Text != dir {
		t.Fatalf("toast %q lines %+v", toast.Title, toast.Lines)
	}
	toast.Action.OnClick()
	if opened != dir {
		t.Fatalf("the toast's button opened %q, want %q", opened, dir)
	}
}
