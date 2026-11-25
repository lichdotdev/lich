package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/lichdotdev/lich/browserwire"
)

// errBrowserGone is a call on a lich-browser process that ended: its pages
// went with it.
var errBrowserGone = errors.New("lich-browser exited, and the pages it held are gone")

// wireConn is one lich-browser process seen from the backend: requests out
// on its stdin, answers and events in from its stdout, one JSON object per
// line (docs/lich-browser.md).
type wireConn struct {
	wmu sync.Mutex
	w   io.Writer
	// stop ends the process; it runs once, when the connection fails.
	stop func()

	mu      sync.Mutex
	next    int64
	pending map[int64]chan browserwire.Message
	pages   map[string]*webkitPage
	done    chan struct{}
	err     error
}

// newWireConn reads r until it ends, then fails every call in flight and
// every page, and calls stop.
func newWireConn(r io.Reader, w io.Writer, stop func()) *wireConn {
	c := &wireConn{w: w, stop: stop, pending: map[int64]chan browserwire.Message{}, pages: map[string]*webkitPage{}, done: make(chan struct{})}
	go c.read(r)
	return c
}

func (c *wireConn) alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// call sends method with params and decodes its result into out, nil to
// drop it.
func (c *wireConn) call(ctx context.Context, method string, params, out any) error {
	req := browserwire.Request{Method: method}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return err
		}
		req.Params = raw
	}
	reply := make(chan browserwire.Message, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return c.err
	}
	c.next++
	req.ID = c.next
	c.pending[req.ID] = reply
	c.mu.Unlock()
	defer c.forget(req.ID)
	if err := c.send(req); err != nil {
		c.fail(err)
		return errBrowserGone
	}
	select {
	case m := <-reply:
		return decodeReply(method, m, out)
	case <-c.done:
		return c.err
	case <-ctx.Done():
		return fmt.Errorf("lich-browser did not answer %s: %w", method, ctx.Err())
	}
}

func decodeReply(method string, m browserwire.Message, out any) error {
	if m.Error != "" {
		return errors.New(m.Error)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(m.Result, out); err != nil {
		return fmt.Errorf("lich-browser's answer to %s: %w", method, err)
	}
	return nil
}

func (c *wireConn) send(req browserwire.Request) error {
	line, err := json.Marshal(req)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.w.Write(append(line, '\n'))
	return err
}

func (c *wireConn) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// read hands each answer to its call and each event to its page, in the
// order lich-browser wrote them: a page's settle reads its load events in
// the order they came. A line is as long as a snapshot's PNG.
func (c *wireConn) read(r io.Reader) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 1 {
			c.dispatch(line)
		}
		if err != nil {
			c.fail(err)
			return
		}
	}
}

func (c *wireConn) dispatch(line []byte) {
	var m browserwire.Message
	if err := json.Unmarshal(line, &m); err != nil {
		slog.Warn("browser: a line from lich-browser that is no message", "err", err)
		return
	}
	if m.ID != 0 {
		c.mu.Lock()
		reply := c.pending[m.ID]
		c.mu.Unlock()
		if reply != nil {
			reply <- m
		}
		return
	}
	var of browserwire.PageParams
	if err := json.Unmarshal(m.Params, &of); err != nil {
		slog.Warn("browser: an event from lich-browser naming no page", "event", m.Event, "err", err)
		return
	}
	c.mu.Lock()
	p := c.pages[of.Page]
	c.mu.Unlock()
	if p != nil {
		p.event(m.Event, m.Params)
	}
}

// fail ends the connection once: the calls waiting get errBrowserGone, and
// every page it held is gone.
func (c *wireConn) fail(cause error) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = errBrowserGone
	pages := c.pages
	c.pages = map[string]*webkitPage{}
	close(c.done)
	c.mu.Unlock()
	if !errors.Is(cause, io.EOF) {
		slog.Warn("browser: lost lich-browser", "err", cause)
	}
	c.stop()
	for _, p := range pages {
		p.gone()
	}
}

func (c *wireConn) add(p *webkitPage) {
	c.mu.Lock()
	c.pages[p.id] = p
	c.mu.Unlock()
}

func (c *wireConn) remove(p *webkitPage) {
	c.mu.Lock()
	delete(c.pages, p.id)
	c.mu.Unlock()
}
