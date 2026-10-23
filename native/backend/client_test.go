package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeBackend answers /rpc/ the way internal/rpc does: positional JSON args
// in, the result or {"error"} out.
func fakeBackend(t *testing.T, handle func(method string, args []json.RawMessage) (any, int)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var args []json.RawMessage
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &args); err != nil {
			t.Errorf("body %q is not a JSON array: %v", body, err)
		}
		result, status := handle(strings.TrimPrefix(r.URL.Path, "/rpc/"), args)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(result)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return New(Runtime{Port: port, Token: "tok"})
}

func TestCallSendsPositionalArgsAndDecodes(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		if method != "project.Diff" || len(args) != 1 || string(args[0]) != `"/repo"` {
			t.Errorf("got %s %s", method, args)
		}
		return DiffStats{Files: 2, Branch: "main"}, http.StatusOK
	})
	d, err := c.Diff(context.Background(), "/repo")
	if err != nil || d.Files != 2 || d.Branch != "main" {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestCallWithNoArgsSendsEmptyArray(t *testing.T) {
	c := fakeBackend(t, func(_ string, args []json.RawMessage) (any, int) {
		if args == nil {
			t.Error("args decoded as null, want []")
		}
		return nil, http.StatusOK
	})
	if _, err := c.LoadState(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStateDecodesTheDelegatingSession(t *testing.T) {
	c := fakeBackend(t, func(string, []json.RawMessage) (any, int) {
		return json.RawMessage(`[{"id":"p","sessions":[{"id":"s2","originSessionId":"s1","originLabel":"orchestrator"}]}]`), http.StatusOK
	})
	ps, err := c.LoadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s := ps[0].Sessions[0]; s.OriginSessionID != "s1" || s.OriginLabel != "orchestrator" {
		t.Fatalf("got %+v", s)
	}
}

func TestCallReturnsBackendError(t *testing.T) {
	c := fakeBackend(t, func(string, []json.RawMessage) (any, int) {
		return map[string]string{"error": "want 2 arguments, got 1"}, http.StatusBadRequest
	})
	err := c.Call(context.Background(), "store.RenameSession", nil, "id")
	if err == nil || !strings.Contains(err.Error(), "want 2 arguments, got 1") {
		t.Fatalf("got %v", err)
	}
}

func TestErrorTextIsTheBackendSentence(t *testing.T) {
	c := fakeBackend(t, func(string, []json.RawMessage) (any, int) {
		return map[string]string{"error": "no file manager found"}, http.StatusInternalServerError
	})
	err := c.Call(context.Background(), "system.OpenFolder", nil, "/tmp")
	if got := ErrorText(err); got != "no file manager found" {
		t.Fatalf("ErrorText = %q, want the backend's sentence alone", got)
	}
	if plain := errors.New("dial tcp: refused"); ErrorText(plain) != plain.Error() {
		t.Fatalf("ErrorText of a local error = %q, want it unchanged", ErrorText(plain))
	}
}

func TestFrameRoundTrip(t *testing.T) {
	frame, err := encodeFrame("abc", []byte("ls\r"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame, []byte("\x03abcls\r")) {
		t.Fatalf("encoded %q", frame)
	}
	id, payload, err := decodeFrame(frame)
	if err != nil || id != "abc" || string(payload) != "ls\r" {
		t.Fatalf("decoded %q %q %v", id, payload, err)
	}
	for _, bad := range [][]byte{nil, {0}, {5, 'a'}} {
		if _, _, err := decodeFrame(bad); err == nil {
			t.Errorf("decodeFrame(%q) accepted", bad)
		}
	}
	if _, err := encodeFrame("", nil); err == nil {
		t.Error("empty id accepted")
	}
}

func TestRevertLinesSendsTheLinesAsTheBackendNamesThem(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		want := `[{"side":"old","line":4,"text":"b"},{"side":"new","line":4,"text":"B"}]`
		if method != "project.RevertLines" || len(args) != 3 || string(args[1]) != `"main.go"` || string(args[2]) != want {
			t.Errorf("got %s %s", method, args)
		}
		return RevertResult{Patch: "p"}, http.StatusOK
	})
	r, err := c.RevertLines(context.Background(), "/repo", "main.go", []RevertLine{{"old", 4, "b"}, {"new", 4, "B"}})
	if err != nil || r.Patch != "p" {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestFileLinesSendsRefAndRange(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		if method != "project.FileLines" || len(args) != 5 || string(args[2]) != `""` || string(args[3]) != "3" || string(args[4]) != "4" {
			t.Errorf("got %s %s", method, args)
		}
		return []string{"x", "y"}, http.StatusOK
	})
	lines, err := c.FileLines(context.Background(), "/repo", "main.go", "", 3, 4)
	if err != nil || len(lines) != 2 {
		t.Fatalf("got %q, %v", lines, err)
	}
}

func TestRetargetMovesEveryCallToTheNewBackend(t *testing.T) {
	successor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" || r.URL.Query().Get("token") != "new" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(successor.Close)
	u, _ := url.Parse(successor.URL)
	port, _ := strconv.Atoi(u.Port())
	c := New(Runtime{Port: port, Token: "stale"})
	if c.Answers(context.Background()) {
		t.Fatal("a backend that refuses the token answers")
	}
	c.Retarget(Runtime{Port: port, Token: "new"})
	if !c.Answers(context.Background()) {
		t.Fatal("the retargeted client does not reach the successor")
	}
	successor.Close()
	if c.Answers(context.Background()) {
		t.Fatal("a backend that is gone answers")
	}
}

func TestRuntimePathFollowsLichDev(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	daily, err := RuntimePath(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	dev, err := RuntimePath(func(k string) string { return map[string]string{"LICH_DEV": "1"}[k] })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(daily, filepath.Join("lich", "runtime.json")) || !strings.HasSuffix(dev, filepath.Join("lich", "runtime-dev.json")) {
		t.Fatalf("paths = %q, %q", daily, dev)
	}
}
