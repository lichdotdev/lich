package main

import (
	"context"
	"encoding/json"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gioui.org/app"
	"gioui.org/f32"

	"github.com/omartelo/lich/native/lichclient"
)

func TestParseURIListKeepsLocalFilesOnly(t *testing.T) {
	data := "# dragged from Files\r\n" +
		"file:///home/me/shot%20one.png\r\n" +
		"https://example.com/x.png\r\n" +
		"file://localhost/tmp/log.txt\r\n" +
		"file://otherhost/tmp/remote.txt\r\n" +
		"\r\n" +
		"file:///home/me/caf%C3%A9.md"
	got := parseURIList(data)
	want := []string{
		filepath.FromSlash("/home/me/shot one.png"),
		filepath.FromSlash("/tmp/log.txt"),
		filepath.FromSlash("/home/me/café.md"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func enter(x, y float32, types ...string) app.DropEvent {
	return app.DropEvent{Kind: app.DropEnter, Position: f32.Pt(x, y), Types: types}
}

func move(x, y float32) app.DropEvent {
	return app.DropEvent{Kind: app.DropMove, Position: f32.Pt(x, y)}
}

func drop(x, y float32, list string) app.DropEvent {
	return app.DropEvent{Kind: app.DropDrop, Position: f32.Pt(x, y), Type: uriListMIME, Data: []byte(list)}
}

func TestTerminalDropHintsOnlyOverTheTerminal(t *testing.T) {
	var d terminalDrop
	d.setRect(image.Rect(100, 50, 500, 400))
	steps := []struct {
		e    app.DropEvent
		want bool
	}{
		{enter(10, 10, "text/plain", uriListMIME), false},
		{move(200, 200), true},
		{move(600, 200), false},
		{move(200, 200), true},
		{app.DropEvent{Kind: app.DropLeave}, false},
		{enter(200, 200, "text/plain"), false},
		{move(250, 250), false},
	}
	for i, s := range steps {
		d.handle(s.e)
		if d.hinting() != s.want {
			t.Fatalf("step %d: hinting %v, want %v", i, d.hinting(), s.want)
		}
	}
}

func TestTerminalDropLandsOnlyOnTheTerminal(t *testing.T) {
	var d terminalDrop
	d.setRect(image.Rect(100, 50, 500, 400))
	list := "file:///tmp/a.png\r\n"
	d.handle(enter(20, 20, uriListMIME))
	if got := d.handle(drop(20, 20, list)); got != nil {
		t.Fatalf("drop on the sidebar gave %q", got)
	}
	d.handle(enter(200, 200, uriListMIME))
	got := d.handle(drop(200, 200, list))
	if !reflect.DeepEqual(got, []string{filepath.FromSlash("/tmp/a.png")}) || d.hinting() {
		t.Fatalf("drop on the terminal gave %q, hinting %v", got, d.hinting())
	}
	d.setRect(image.Rectangle{})
	d.handle(enter(200, 200, uriListMIME))
	if got := d.handle(drop(200, 200, list)); got != nil {
		t.Fatalf("drop with no terminal on screen gave %q", got)
	}
}

// dropBackend is /drop as internal/drop serves it: the copy's path under the
// session, and its notice. It records what it was sent.
func dropBackend(t *testing.T, uploads *[]string) *lichclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/drop" || r.URL.Query().Get("token") != "tok" {
			http.Error(w, "unexpected "+r.URL.String(), http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		name, session := r.URL.Query().Get("name"), r.URL.Query().Get("session")
		*uploads = append(*uploads, session+"/"+name+"="+string(body))
		_ = json.NewEncoder(w).Encode(map[string]string{"path": "/copies/" + session + "/" + name, "notice": "copy of " + name})
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
}

func writeFile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveDropPastesRealPathsForAnUnconfinedSession(t *testing.T) {
	var uploads []string
	c := dropBackend(t, &uploads)
	dir := t.TempDir()
	file := writeFile(t, filepath.Join(dir, "a.txt"), "x")
	r := resolveDrop(context.Background(), c, dropTarget{sessionID: "s1", checkout: "/elsewhere"}, []string{file, dir})
	if !reflect.DeepEqual(r.paths, []string{file, dir}) || r.notices != nil || r.skipped != nil || uploads != nil {
		t.Fatalf("got %+v, uploads %q", r, uploads)
	}
}

func TestResolveDropCopiesWhatAConfinedSessionCannotRead(t *testing.T) {
	var uploads []string
	c := dropBackend(t, &uploads)
	checkout, outside := t.TempDir(), t.TempDir()
	inside := writeFile(t, filepath.Join(checkout, "in.txt"), "in")
	away := writeFile(t, filepath.Join(outside, "out.txt"), "out")
	big := filepath.Join(outside, "big.bin")
	if err := os.WriteFile(big, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, maxDropUpload+1); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(outside, "folder")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(outside, "gone.txt")

	target := dropTarget{sessionID: "s1", checkout: checkout, confined: true}
	r := resolveDrop(context.Background(), c, target, []string{inside, away, big, folder, missing})

	if want := []string{inside, "/copies/s1/out.txt"}; !reflect.DeepEqual(r.paths, want) {
		t.Fatalf("paths %q, want %q", r.paths, want)
	}
	if want := []string{"copy of out.txt"}; !reflect.DeepEqual(r.notices, want) {
		t.Fatalf("notices %q, want %q", r.notices, want)
	}
	if want := []string{"s1/out.txt=out"}; !reflect.DeepEqual(uploads, want) {
		t.Fatalf("uploads %q, want %q", uploads, want)
	}
	if len(r.skipped) != 3 ||
		!strings.HasPrefix(r.skipped[0], "big.bin (over 32MB") ||
		!strings.Contains(r.skipped[1], "folders outside this sandboxed session's checkout") ||
		!strings.HasPrefix(r.skipped[2], "gone.txt (") {
		t.Fatalf("skipped %q", r.skipped)
	}
}

func TestResolveDropSkipsAFailedUpload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "disk full", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	c := lichclient.New(lichclient.Runtime{Port: port, Token: "tok"})
	away := writeFile(t, filepath.Join(t.TempDir(), "out.txt"), "out")
	r := resolveDrop(context.Background(), c, dropTarget{sessionID: "s1", checkout: t.TempDir(), confined: true}, []string{away})
	if r.paths != nil || len(r.skipped) != 1 || !strings.Contains(r.skipped[0], "disk full (HTTP 500)") {
		t.Fatalf("got %+v", r)
	}
}
