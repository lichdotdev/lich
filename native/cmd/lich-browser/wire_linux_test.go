package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/browserwire"
)

// The tests drive a built lich-browser over its wire, as the backend does,
// on the X display they run on (CI's Xvfb).

var built string

func TestMain(m *testing.M) {
	if os.Getenv("DISPLAY") == "" {
		fmt.Println("lich-browser: no X display, the wire tests skip")
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "lich-browser-test-")
	if err != nil {
		panic(err)
	}
	built = filepath.Join(dir, "lich-browser")
	if out, err := exec.Command("go", "build", "-o", built, ".").CombinedOutput(); err != nil {
		fmt.Printf("build lich-browser: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// waitFor bounds every wait on the wire: a page load, an answer, an event.
const waitFor = 15 * time.Second

type client struct {
	t     *testing.T
	in    io.WriteCloser
	mu    sync.Mutex
	id    int64
	reply map[int64]chan browserwire.Message
	// events are every event so far, in order; more is signalled on each.
	events []browserwire.Message
	more   chan struct{}
}

func start(t *testing.T) *client {
	t.Helper()
	if built == "" {
		t.Skip("no X display")
	}
	cmd := exec.Command(built)
	cmd.Env = append(os.Environ(), "WAYLAND_DISPLAY=/nonexistent/wl-none")
	cmd.Stderr = &testLog{t: t}
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, in: in, reply: map[int64]chan browserwire.Message{}, more: make(chan struct{}, 1)}
	read := make(chan struct{})
	go c.read(out, read)
	t.Cleanup(func() {
		in.Close()
		<-read
		if err := cmd.Wait(); err != nil {
			t.Errorf("lich-browser exited: %v", err)
		}
	})
	return c
}

// testLog is lich-browser's stderr, in the test's log.
type testLog struct{ t *testing.T }

func (l *testLog) Write(p []byte) (int, error) {
	l.t.Logf("stderr: %s", p)
	return len(p), nil
}

func (c *client) read(out io.Reader, done chan struct{}) {
	defer close(done)
	lines := bufio.NewScanner(out)
	lines.Buffer(nil, maxLine)
	for lines.Scan() {
		var m browserwire.Message
		if err := json.Unmarshal(lines.Bytes(), &m); err != nil {
			c.t.Errorf("stdout carries a line that is no wire message: %q", lines.Bytes())
			continue
		}
		c.mu.Lock()
		if m.Event != "" {
			c.events = append(c.events, m)
			select {
			case c.more <- struct{}{}:
			default:
			}
		} else if ch := c.reply[m.ID]; ch != nil {
			ch <- m
		}
		c.mu.Unlock()
	}
}

// call sends a request and waits for its answer.
func (c *client) call(method string, params, result any) error {
	c.t.Helper()
	raw, _ := json.Marshal(params)
	c.mu.Lock()
	c.id++
	id := c.id
	ch := make(chan browserwire.Message, 1)
	c.reply[id] = ch
	c.mu.Unlock()
	line, _ := json.Marshal(browserwire.Request{ID: id, Method: method, Params: raw})
	if _, err := c.in.Write(append(line, '\n')); err != nil {
		c.t.Fatal(err)
	}
	select {
	case m := <-ch:
		if m.Error != "" {
			return fmt.Errorf("%s", m.Error)
		}
		if result != nil {
			if err := json.Unmarshal(m.Result, result); err != nil {
				c.t.Fatal(err)
			}
		}
		return nil
	case <-time.After(waitFor):
		c.t.Fatalf("%s: no answer in %v", method, waitFor)
		return nil
	}
}

func (c *client) must(method string, params, result any) {
	c.t.Helper()
	if err := c.call(method, params, result); err != nil {
		c.t.Fatalf("%s: %v", method, err)
	}
}

// event waits for an event named name, from index from on, that match
// accepts, and answers it and the index after it.
func (c *client) event(from int, name string, match func(json.RawMessage) bool) (json.RawMessage, int) {
	c.t.Helper()
	deadline := time.After(waitFor)
	for {
		c.mu.Lock()
		for i := from; i < len(c.events); i++ {
			if e := c.events[i]; e.Event == name && (match == nil || match(e.Params)) {
				c.mu.Unlock()
				return e.Params, i + 1
			}
		}
		from = len(c.events)
		c.mu.Unlock()
		select {
		case <-c.more:
		case <-deadline:
			c.t.Fatalf("no %s event in %v", name, waitFor)
		}
	}
}

func (c *client) mark() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

// open opens page id on url and waits for its load to finish.
func (c *client) open(id, url string, w, h int, scale float64) {
	c.t.Helper()
	from := c.mark()
	c.must(browserwire.Open, browserwire.OpenParams{Page: id, URL: url, Scale: scale, W: w, H: h}, nil)
	c.loaded(from, id)
}

func (c *client) loaded(from int, page string) {
	c.t.Helper()
	c.event(from, browserwire.EventLoad, func(raw json.RawMessage) bool {
		var l browserwire.LoadEvent
		return json.Unmarshal(raw, &l) == nil && l.Page == page && l.Step == browserwire.LoadFinished
	})
}

func (c *client) eval(page, script string) string {
	c.t.Helper()
	var r browserwire.EvalResult
	c.must(browserwire.Eval, browserwire.EvalParams{Page: page, Script: script}, &r)
	return r.JSON
}
