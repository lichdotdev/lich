package browser

import (
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/browserwire"
)

func decodeParams[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestWebkitOpenLoadsThePageAndReportsItsTabs(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)

	h, err := s.Open("s", "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if h.URL != "https://example.com/" || h.Title != "Title of https://example.com/" {
		t.Fatalf("handle %+v: want the page lich-browser loaded", h)
	}
	if hello := f.called(browserwire.Hello); len(hello) != 1 {
		t.Fatalf("%d hellos, want 1 before anything else", len(hello))
	}
	states := out.states()
	last := states[len(states)-1]
	if !last.Open || len(last.Tabs) != 1 || last.Tabs[0].XID == 0 || !last.Tabs[0].Front {
		t.Fatalf("browser-state %+v: want the open tab with its X window", last)
	}
	tabs, err := s.Tabs("s")
	if err != nil || len(tabs) != 1 || tabs[0].XID != 0 {
		t.Fatalf("agent's tabs %+v, %v: want one, with no X window", tabs, err)
	}
}

// lich-browser answers open and navigate at once and loads after, open's
// own blank load included: an answer read before the load came back with
// about:blank and no title.
func TestWebkitOpenAndNavigateAnswerWithTheLoadedPage(t *testing.T) {
	f := newFakeLichBrowser(t)
	f.lateLoads = true
	s, _ := wkService(t, f)
	h, err := s.Open("s", "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if h.URL != "https://example.com/" || h.Title != "Title of https://example.com/" {
		t.Fatalf("open answered %q %q, want the loaded page's URL and title", h.URL, h.Title)
	}
	h, err = s.Navigate(h.ID, "https://example.org/b")
	if err != nil {
		t.Fatal(err)
	}
	if h.URL != "https://example.org/b" || h.Title != "Title of https://example.org/b" {
		t.Fatalf("navigate answered %q %q, want the loaded page's URL and title", h.URL, h.Title)
	}
}

// A history step answers once its load landed, as Chrome's does: lich-browser
// answers the request at once and loads after.
func TestWebkitHistoryStepsAnswerOnceLoaded(t *testing.T) {
	for _, tc := range []struct {
		name string
		step func(s *Service, id string) error
	}{
		{"reload", (*Service).Reload},
		{"back", (*Service).Back},
		{"forward", (*Service).Forward},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.lateLoads = true
			s, _ := wkService(t, f)
			h, err := s.Open("s", "https://example.com/")
			if err != nil {
				t.Fatal(err)
			}
			begun := time.Now()
			if err := tc.step(s, h.ID); err != nil {
				t.Fatal(err)
			}
			if took := time.Since(begun); took < lateLoad {
				t.Fatalf("%s answered after %v, before its load landed %v after the request", tc.name, took, lateLoad)
			}
		})
	}
}

// The agent's screenshot is the page alone: the window's layer (its notes,
// the agent's cursor and chip) is the window's, drawn into the page.
func TestWebkitAgentScreenshotLeavesTheWindowsLayerOut(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, _ := wkService(t, f)
	h, err := s.Open("s", "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	for _, shoot := range []func(id, dest string) (string, error){s.Screenshot, s.AnnotatedScreenshot} {
		if _, err := shoot(h.ID, filepath.Join(t.TempDir(), "shot.png")); err != nil {
			t.Fatal(err)
		}
	}
	shots := f.called(browserwire.Snapshot)
	if len(shots) != 2 {
		t.Fatalf("%d snapshots, want one per screenshot", len(shots))
	}
	for _, raw := range shots {
		if !decodeParams[browserwire.SnapshotParams](t, raw).Bare {
			t.Fatalf("snapshot %s: the agent's screenshot holds the window's layer", raw)
		}
	}
}

func TestWebkitHelloRefusesAnotherWire(t *testing.T) {
	f := newFakeLichBrowser(t)
	f.version = browserwire.Version + 1
	s, _ := wkService(t, f)
	if _, err := s.Open("s", "https://example.com/"); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("Open = %v, want the wire's version refused", err)
	}
}

func TestWebkitNavigationOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		wantErr   bool
		wantURL   string
	}{
		{name: "loads", url: "https://example.com/a", wantURL: "https://example.com/a"},
		{name: "fails", url: "https://down.example/", wantErr: true},
		{name: "becomes a download", url: "https://example.com/x.zip", wantURL: "https://example.com/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.failURL, f.downloadURL = "https://down.example/", "https://example.com/x.zip"
			s, _ := wkService(t, f)
			if _, err := s.Open("s", "https://example.com/"); err != nil {
				t.Fatal(err)
			}
			h, err := s.Navigate("s", tc.url)
			begun := time.Now()
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "page load error Could not resolve host") {
					t.Fatalf("Navigate = %v, want the page not loaded", err)
				}
				return
			}
			if err != nil || h.URL != tc.wantURL {
				t.Fatalf("Navigate = %+v, %v: want at %s", h, err, tc.wantURL)
			}
			if took := time.Since(begun); took >= downloadNotice {
				t.Fatalf("Navigate took %v: a load that says it became a download waits for nothing", took)
			}
		})
	}
}

func TestWebkitActionsAreNativeInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(s *Service) error
		want []string
	}{
		{"click", func(s *Service) error { return s.Click("s", Target{Selector: "#go"}) },
			[]string{"mouse move 10,20", "mouse down 10,20", "mouse up 10,20"}},
		{"press", func(s *Service) error { return s.Press("s", "enter", Target{}) },
			[]string{"key down Enter/Enter", "key up Enter/Enter"}},
		{"type", func(s *Service) error { return s.Type("s", "hi", true, Target{}) },
			[]string{"eval clear", "text hi"}},
		{"scroll", func(s *Service) error { return s.Scroll("s", 300) },
			[]string{"mouse wheel 400,300 dy 300"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.eval = func(script string) (string, string, bool) {
				switch {
				case strings.HasPrefix(script, resolveJS):
					return `{"x":10,"y":20}`, "", true
				case script == viewMiddleJS:
					return `{"x":400,"y":300}`, "", true
				}
				return "", "", false
			}
			s, _ := wkService(t, f)
			if _, err := s.Open("s", "https://example.com/"); err != nil {
				t.Fatal(err)
			}
			if err := tc.act(s); err != nil {
				t.Fatal(err)
			}
			if got := inputs(t, f); strings.Join(got, "; ") != strings.Join(tc.want, "; ") {
				t.Fatalf("input %q, want %q", got, tc.want)
			}
		})
	}
}

// inputs is the native input lich-browser got, and the field clears.
func inputs(t *testing.T, f *fakeLichBrowser) []string {
	f.mu.Lock()
	calls := append([]browserwire.Request{}, f.calls...)
	f.mu.Unlock()
	var out []string
	for _, c := range calls {
		switch c.Method {
		case browserwire.Mouse:
			m := decodeParams[browserwire.MouseParams](t, c.Params)
			line := m.Kind + " " + ftoa(m.X) + "," + ftoa(m.Y)
			if m.Kind == browserwire.MouseWheel {
				line += " dy " + ftoa(m.DY)
			}
			out = append(out, "mouse "+line)
		case browserwire.Key:
			k := decodeParams[browserwire.KeyParams](t, c.Params)
			out = append(out, "key "+k.Kind+" "+k.Key+"/"+k.Code)
		case browserwire.Text:
			out = append(out, "text "+decodeParams[browserwire.TextParams](t, c.Params).Text)
		case browserwire.Eval:
			if decodeParams[browserwire.EvalParams](t, c.Params).Script == clearFieldJS {
				out = append(out, "eval clear")
			}
		}
	}
	return out
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func TestWebkitEvalAnswersAsChromeDoes(t *testing.T) {
	for _, tc := range []struct {
		name, json, err string
		want, wantErr   string
	}{
		{name: "string", json: `"hi"`, want: "hi"},
		{name: "object", json: `{"a":1}`, want: `{"a":1}`},
		{name: "undefined", want: "undefined"},
		{name: "throw", err: "ReferenceError: nope is not defined", wantErr: "the script threw: ReferenceError: nope is not defined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.eval = func(script string) (string, string, bool) {
				return tc.json, tc.err, script == "agent()"
			}
			s, _ := wkService(t, f)
			if _, err := s.Open("s", "https://example.com/"); err != nil {
				t.Fatal(err)
			}
			got, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "agent()"})
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("Eval = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("Eval = %q, %v: want %q", got, err, tc.want)
			}
		})
	}
}

func TestWebkitScreenshotIsTheShotsSize(t *testing.T) {
	for _, tc := range []struct {
		name         string
		w, h         int
		scale        float64
		wantW, wantH int
	}{
		{name: "1x", w: 800, h: 600, scale: 1, wantW: 800, wantH: 600},
		{name: "HiDPI", w: 800, h: 600, scale: 2, wantW: 800, wantH: 600},
		{name: "wide pane", w: 2000, h: 1000, scale: 1, wantW: 1280, wantH: 640},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeLichBrowser(t)
			f.snapW, f.snapH, f.snapScale = tc.w, tc.h, tc.scale
			s, _ := wkService(t, f)
			if _, err := s.Open("s", "https://example.com/"); err != nil {
				t.Fatal(err)
			}
			path, err := s.Screenshot("s", filepath.Join(t.TempDir(), "shot.png"))
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			cfg, err := png.DecodeConfig(file)
			if err != nil || cfg.Width != tc.wantW || cfg.Height != tc.wantH {
				t.Fatalf("shot %dx%d, %v: want %dx%d", cfg.Width, cfg.Height, err, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestWebkitDialogHoldsTheAgentUntilAnswered(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	id := f.onlyPage()
	f.send(browserwire.EventDialog, browserwire.DialogEvent{Page: id, Dialog: &browserwire.JSDialog{Type: browserwire.DialogConfirm, Message: "Sure?"}})
	waitFor(t, "the dialog on browser-state", func() bool {
		st := out.states()
		return st[len(st)-1].Dialog != nil
	})
	if _, err := s.Info("s", ""); !errors.Is(err, errDialogOpen) {
		t.Fatalf("Info = %v, want held by the dialog", err)
	}
	info, err := s.AnswerDialog(t.Context(), api.BrowserDialogOptions{ID: "s", Accept: true})
	if err != nil || info.Dialog != nil || info.URL != "https://example.com/" {
		t.Fatalf("AnswerDialog = %+v, %v: want the page after the dialog", info, err)
	}
	answers := f.called(browserwire.Dialog)
	if len(answers) != 1 || !decodeParams[browserwire.DialogParams](t, answers[0]).Accept {
		t.Fatalf("dialog requests %s, want one accept", answers)
	}
}

func TestWebkitPageEventsReachTheAgent(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	id := f.onlyPage()
	f.send(browserwire.EventConsole, browserwire.ConsoleEvent{Page: id, Text: "Uncaught TypeError: x"})
	f.send(browserwire.EventDownload, browserwire.DownloadEvent{Page: id, GUID: "g", Name: "a.csv", Path: "/d/a.csv", State: browserwire.DownloadCompleted, Received: 4})
	waitFor(t, "the download", func() bool { return len(out.named(api.BrowserDownloadEvent.Name)) == 1 })
	info, err := s.PressKey(t.Context(), api.BrowserPressOptions{ID: "s", Key: "Tab"})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.ConsoleErrors) != 1 || len(info.Downloads) != 1 || info.Downloads[0].Path != "/d/a.csv" {
		t.Fatalf("page answer %+v: want the console error and the download", info)
	}
}

func TestWebkitDeadProcessClosesItsBrowsersAndRestarts(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)
	for _, owner := range []string{"a", "b"} {
		if _, err := s.Open(owner, "https://example.com/"); err != nil {
			t.Fatal(err)
		}
	}
	f.kill()
	waitFor(t, "both browsers closed", func() bool {
		closed := map[string]bool{}
		for _, st := range out.states() {
			if !st.Open {
				closed[st.Owner] = true
			}
		}
		return closed["a"] && closed["b"]
	})
	if _, err := s.Open("a", "https://example.com/again"); err != nil {
		t.Fatal(err)
	}
	if f.spawns != 2 {
		t.Fatalf("%d starts, want lich-browser started again", f.spawns)
	}
}

func TestWebkitCrashedPageCloses(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	f.send(browserwire.EventCrashed, browserwire.PageParams{Page: f.onlyPage()})
	waitFor(t, "the browser closed", func() bool {
		st := out.states()
		return !st[len(st)-1].Open
	})
	waitFor(t, "the page closed in lich-browser", func() bool { return len(f.called(browserwire.Close)) == 1 })
}

func TestWebkitWindowReachesEveryProcess(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, _ := wkService(t, f)
	if _, err := s.Window(t.Context(), api.BrowserWindowOptions{XID: 7}); err != nil {
		t.Fatal(err)
	}
	if f.spawns != 0 {
		t.Fatal("the window's X window started lich-browser")
	}
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Window(t.Context(), api.BrowserWindowOptions{}); err != nil {
		t.Fatal(err)
	}
	var got []uint64
	for _, raw := range f.called(browserwire.Window) {
		got = append(got, decodeParams[browserwire.WindowParams](t, raw).XID)
	}
	if len(got) != 2 || got[0] != 7 || got[1] != 0 {
		t.Fatalf("window requests %v, want 7 at the start and 0 once it closed", got)
	}
}

func TestWebkitOpensInAWindowOfItsOwn(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)
	if h, err := s.OpenVisible("fresh"); err != nil || !h.Headed {
		t.Fatalf("OpenVisible = %+v, %v: want a page in a window of its own", h, err)
	}
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	if h, err := s.OpenVisible("s"); err != nil || !h.Headed || h.URL != "https://example.com/" {
		t.Fatalf("OpenVisible = %+v, %v: want the same page in its own window", h, err)
	}
	if n := len(f.called(browserwire.Open)); n != 2 {
		t.Fatalf("%d opens, want the page moved out as it was, not opened again", n)
	}
	if n := len(f.called(browserwire.PopOut)); n != 2 {
		t.Fatalf("%d pop outs, want 2", n)
	}
	if _, err := s.OpenVisible("s"); err != nil || len(f.called(browserwire.Raise)) != 1 {
		t.Fatalf("OpenVisible again = %v, want its window raised", err)
	}
	if _, err := s.Attach("s", Viewport{W: 800, H: 600, Scale: 1}); !errors.Is(err, errInWindow) {
		t.Fatalf("Attach = %v, want the page left in its window", err)
	}
	st := out.states()
	if last := st[len(st)-1]; !last.Headed || last.Tabs[0].XID != 0 {
		t.Fatalf("browser-state %+v: want headed, its tab embedded nowhere", last)
	}
	f.mu.Lock()
	var id string
	for pid, p := range f.pages {
		if p.tabs[0].URL == "https://example.com/" {
			id = pid
		}
	}
	f.mu.Unlock()
	f.send(browserwire.EventClosed, browserwire.PageParams{Page: id})
	waitFor(t, "the browser closed with its window", func() bool {
		hs, _ := s.List("s")
		return len(hs) == 0
	})
	if n := len(f.called(browserwire.Close)); n != 0 {
		t.Fatalf("%d closes sent for a page lich-browser already ended", n)
	}
}

func TestWebkitPageDownloadsWhereChromesDo(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, _ := wkService(t, f)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	want, _ := downloadFolder()
	if got := decodeParams[browserwire.OpenParams](t, f.called(browserwire.Open)[0]).Downloads; got != want {
		t.Fatalf("open's downloads %q, want %q", got, want)
	}
}

func TestWebkitScaleIsAZoomNeverARelaunch(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, _ := wkService(t, f)
	s.rescaleWait = time.Millisecond
	if _, err := s.Attach("s", Viewport{W: 900, H: 700, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	s.SetDefaultView(Viewport{W: 1200, H: 800, Scale: 2}, 2)
	if err := s.Resize("s", Viewport{W: 900, H: 700, Scale: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.Resize("s", Viewport{W: 950, H: 700, Scale: 2}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if n := len(f.called(browserwire.Open)); n != 1 {
		t.Fatalf("%d opens, want the page kept across the scale change", n)
	}
	var zooms []float64
	for _, raw := range f.called(browserwire.Zoom) {
		zooms = append(zooms, decodeParams[browserwire.ZoomParams](t, raw).Scale)
	}
	if len(zooms) != 2 || zooms[0] != 1 || zooms[1] != 2 {
		t.Fatalf("zooms %v, want the pane's 1 from the launch's 1.5, then 2 once", zooms)
	}
	if sizes := f.called(browserwire.Size); len(sizes) < 3 {
		t.Fatalf("%d sizes, want the pane's each time", len(sizes))
	}
}

func TestWebkitTabs(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, _ := wkService(t, f)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: api.TabClose, Tab: "t1"}); !errors.Is(err, errLastTab) {
		t.Fatalf("closing the last tab = %v, want refused", err)
	}
	if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: api.TabNew}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the second tab", func() bool { tabs, _ := s.Tabs("s"); return len(tabs) == 2 })
	for _, tc := range []struct {
		action, tab string
		want        error
	}{
		{api.TabSelect, "nope", errNoTab},
		{api.TabSelect, "t1", nil},
		{api.TabClose, "t2", nil},
	} {
		if err := s.TabAct(api.BrowserTabOptions{ID: "s", Action: tc.action, Tab: tc.tab}); !errors.Is(err, tc.want) {
			t.Fatalf("%s %s = %v, want %v", tc.action, tc.tab, err, tc.want)
		}
	}
	waitFor(t, "one tab again", func() bool { tabs, _ := s.Tabs("s"); return len(tabs) == 1 })
}

func TestWebkitPaneCalls(t *testing.T) {
	f := newFakeLichBrowser(t)
	f.icon = pngOf(t, 16)
	f.eval = func(script string) (string, string, bool) {
		switch {
		case script == selectionScript:
			return `"picked"`, "", true
		case strings.Contains(script, "elementFromPoint"):
			return `{"selector":"#a"}`, "", true
		}
		return "", "", false
	}
	s, out := wkService(t, f)
	s.SetPicker(func(string) (string, error) { return "/tmp/one", nil }, nil)
	if _, err := s.Attach("s", Viewport{W: 900, H: 700, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.DevTools("s"); err != nil || len(f.called(browserwire.Inspect)) != 1 {
		t.Fatalf("DevTools = %v, want the inspector asked for", err)
	}
	if text, err := s.SelectedText(t.Context(), api.SessionIDOptions{ID: "s"}); err != nil || text != "picked" {
		t.Fatalf("Selection = %q, %v", text, err)
	}
	f.send(browserwire.EventFiles, browserwire.FilesEvent{Page: f.onlyPage()})
	waitFor(t, "the chooser answered", func() bool { return len(f.called(browserwire.Files)) == 1 })
	if got := decodeParams[browserwire.FilesParams](t, f.called(browserwire.Files)[0]).Paths; len(got) != 1 || got[0] != "/tmp/one" {
		t.Fatalf("files %v, want the picked one", got)
	}
	st := out.states()
	key := st[len(st)-1].Tabs[0].Icon
	if b, err := s.Icon(t.Context(), api.BrowserIconOptions{Key: key}); err != nil || len(b) == 0 {
		t.Fatalf("icon %q = %v: want the tab's PNG", key, err)
	}
	for _, input := range []error{
		mustNil(s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "move"})),
		mustNil(s.KeyInput(t.Context(), api.BrowserKeyOptions{ID: "s", Type: "down"})),
		mustNil(s.TextInput(t.Context(), api.BrowserTextOptions{ID: "s", Text: "x"})),
	} {
		if input != nil {
			t.Fatal(input)
		}
	}
	if n := len(f.called(browserwire.Mouse)) + len(f.called(browserwire.Key)) + len(f.called(browserwire.Text)); n != 0 {
		t.Fatalf("%d pane inputs reached lich-browser, which takes them from the X server", n)
	}
	if _, err := s.MouseInput(t.Context(), api.BrowserMouseOptions{ID: "s", Type: "wheel", X: 5, Y: 6, DY: 120}); err != nil {
		t.Fatal(err)
	}
	wheel := f.called(browserwire.Mouse)
	if len(wheel) != 1 {
		t.Fatalf("%d wheels reached lich-browser, want the one Select sends, which X gives the page none of", len(wheel))
	}
	if m := decodeParams[browserwire.MouseParams](t, wheel[0]); m.Kind != browserwire.MouseWheel || m.X != 5 || m.Y != 6 || m.DY != 120 {
		t.Fatalf("wheel %+v, want the pane's at 5,6 by 120", m)
	}
}

func TestWebkitWindowLayerIsNeverTheAgents(t *testing.T) {
	f := newFakeLichBrowser(t)
	s, out := wkService(t, f)
	if _, err := s.Attach("s", Viewport{W: 900, H: 700, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	acts, evals := len(out.named(api.BrowserAgentEvent.Name)), len(f.called(browserwire.Eval))
	if _, err := s.Draw(t.Context(), api.BrowserDrawOptions{ID: "s", Script: "draw()"}); err != nil {
		t.Fatal(err)
	}
	draws := f.called(browserwire.Draw)
	if len(draws) != 1 || decodeParams[browserwire.DrawParams](t, draws[0]).Script != "draw()" {
		t.Fatalf("draws %s, want the window's script passed on", draws)
	}
	png, err := s.Snapshot(t.Context(), api.SessionIDOptions{ID: "s"})
	if err != nil || len(png) == 0 {
		t.Fatalf("Snapshot = %d bytes, %v", len(png), err)
	}
	shots := f.called(browserwire.Snapshot)
	if len(shots) != 1 || !decodeParams[browserwire.SnapshotParams](t, shots[0]).Bare {
		t.Fatalf("snapshots %s, want one with the window's layer left out", shots)
	}
	if n := len(out.named(api.BrowserAgentEvent.Name)); n != acts {
		t.Fatalf("the window's layer made %d agent acts: the pane would show the agent acting", n-acts)
	}
	if n := len(f.called(browserwire.Eval)); n != evals {
		t.Fatalf("%d evals: the layer went through the agent's eval", n-evals)
	}
}

func TestChromePageHasNoWindowLayer(t *testing.T) {
	s, _, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Draw(t.Context(), api.BrowserDrawOptions{ID: "s", Script: "x"}); !errors.Is(err, errNoLayer) {
		t.Fatalf("Draw on Chrome's page = %v, want errNoLayer", err)
	}
}

func mustNil(_ api.None, err error) error { return err }
