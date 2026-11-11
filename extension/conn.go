package extension

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strconv"
	"sync"
)

// maxInboundLine bounds one line from lich. lich writes nothing larger than
// what the window's /events socket carries, 4 MiB a frame.
const maxInboundLine = 4 << 20

// ErrClosed is what a call returns once lich has closed the connection.
var ErrClosed = errors.New("extension: lich closed the connection")

// Conn is an extension's end of its conversation with lich, over its stdin
// and stdout: NewConn(os.Stdin, os.Stdout). Every method is safe to call
// from several goroutines.
type Conn struct {
	w       io.Writer
	writeMu sync.Mutex

	mu      sync.Mutex
	next    uint64
	pending map[string]chan ResultData
	inbox   []Message
	ended   bool
	wake    chan struct{}

	messages chan Message
}

// NewConn starts reading r, which lich writes to, and writes to w, which lich reads.
func NewConn(r io.Reader, w io.Writer) *Conn {
	c := &Conn{
		w: w, pending: map[string]chan ResultData{},
		wake: make(chan struct{}, 1), messages: make(chan Message),
	}
	go c.read(r)
	go c.forward()
	return c
}

// Messages delivers every message from lich other than a call's result, in
// the order lich sent them: event, surface.opened, surface.closed and
// lich.event. It is closed once lich closes the connection and everything
// before that was delivered, which is the extension's cue to exit. Messages
// wait for the reader without holding up results, so a handler may call lich
// and wait for the answer on the goroutine reading this channel.
func (c *Conn) Messages() <-chan Message {
	return c.messages
}

// Send writes one message to lich: render, toast and the other kinds an
// extension sends.
func (c *Conn) Send(msg Message) error {
	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.w.Write(append(line, '\n'))
	return err
}

// Call runs m in lich with params and waits for its result, or for ctx.
// A result that arrives after ctx ended is dropped.
func Call[P, R any](ctx context.Context, c *Conn, m Method[P, R], params P) (R, error) {
	var zero R
	id, answer, err := c.expect()
	if err != nil {
		return zero, err
	}
	if err := c.Send(m.Call(id, params)); err != nil {
		c.forget(id)
		return zero, err
	}
	select {
	case res, ok := <-answer:
		if !ok {
			return zero, ErrClosed
		}
		return m.Result(res)
	case <-ctx.Done():
		c.forget(id)
		return zero, ctx.Err()
	}
}

// Projects calls ListProjects.
func (c *Conn) Projects(ctx context.Context) ([]Project, error) {
	return Call(ctx, c, ListProjects, NoParams{})
}

// Sessions calls ListSessions.
func (c *Conn) Sessions(ctx context.Context) ([]Session, error) {
	return Call(ctx, c, ListSessions, NoParams{})
}

// Worktrees calls ListWorktrees for project.
func (c *Conn) Worktrees(ctx context.Context, project string) ([]Worktree, error) {
	return Call(ctx, c, ListWorktrees, WorktreesParams{Project: project})
}

// OpenSession calls OpenSession.
func (c *Conn) OpenSession(ctx context.Context, params OpenSessionParams) (OpenedSession, error) {
	return Call(ctx, c, OpenSession, params)
}

// SendToSession calls SendToSession.
func (c *Conn) SendToSession(ctx context.Context, params SendParams) (SendResult, error) {
	return Call(ctx, c, SendToSession, params)
}

// Subscribe starts the events named, which then arrive on Messages as
// lich.event.
func (c *Conn) Subscribe(ctx context.Context, names ...string) error {
	_, err := Call(ctx, c, Subscribe, SubscribeParams{Names: names})
	return err
}

func (c *Conn) expect() (string, chan ResultData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return "", nil, ErrClosed
	}
	c.next++
	id := strconv.FormatUint(c.next, 10)
	answer := make(chan ResultData, 1)
	c.pending[id] = answer
	return id, answer, nil
}

func (c *Conn) forget(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Conn) read(r io.Reader) {
	in := bufio.NewScanner(r)
	in.Buffer(nil, maxInboundLine)
	for in.Scan() {
		var msg Message
		if err := json.Unmarshal(in.Bytes(), &msg); err != nil {
			// Stderr is lich's log: a kind from a newer lich lands there
			// rather than ending the conversation.
			log.Printf("extension: dropped a line from lich: %v", err)
			continue
		}
		c.take(msg)
	}
	if err := in.Err(); err != nil {
		log.Printf("extension: reading from lich: %v", err)
	}
	c.end()
}

func (c *Conn) take(msg Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if res, ok := msg.Data.(ResultData); ok {
		if answer, waiting := c.pending[res.ID]; waiting {
			delete(c.pending, res.ID)
			answer <- res
		}
		return
	}
	c.inbox = append(c.inbox, msg)
	c.signal()
}

func (c *Conn) end() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ended = true
	for _, answer := range c.pending {
		close(answer)
	}
	clear(c.pending)
	c.signal()
}

// signal wakes forward. Called with mu held.
func (c *Conn) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Conn) forward() {
	defer close(c.messages)
	for {
		c.mu.Lock()
		if len(c.inbox) == 0 {
			ended := c.ended
			c.mu.Unlock()
			if ended {
				return
			}
			<-c.wake
			continue
		}
		msg := c.inbox[0]
		c.inbox = c.inbox[1:]
		c.mu.Unlock()
		c.messages <- msg
	}
}
