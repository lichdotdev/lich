// Package lichclient talks to a lich backend the way its web page does:
// positional-argument RPC over HTTP, the /events WebSocket for state changes,
// and the /ws WebSocket for terminal bytes.
//
// The backend serves one client per socket and a new connection drops the
// previous one, so a backend the lich window is using cannot also serve this
// client.
package lichclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/coder/websocket"
)

// Runtime is what a running lich writes to <config>/lich/runtime.json (or
// runtime-dev.json under LICH_DEV) so clients can find it.
type Runtime struct {
	PID   int    `json:"pid"`
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// ReadRuntime reads a runtime file.
func ReadRuntime(path string) (Runtime, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Runtime{}, err
	}
	var rt Runtime
	if err := json.Unmarshal(data, &rt); err != nil {
		return Runtime{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if rt.Port == 0 || rt.Token == "" {
		return Runtime{}, fmt.Errorf("%s: want port and token, got %+v", path, rt)
	}
	return rt, nil
}

// RuntimePath is the runtime file a lich on this machine writes, the way
// internal/singleton names it: LICH_DEV selects the dev instance's own.
func RuntimePath(getenv func(string) string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	name := "runtime.json"
	if getenv("LICH_DEV") != "" {
		name = "runtime-dev.json"
	}
	return filepath.Join(dir, "lich", name), nil
}

// Client is a connection to one lich backend.
type Client struct {
	http *http.Client

	mu    sync.Mutex
	base  string // http://127.0.0.1:<port>
	token string
}

// New returns a client for the backend rt describes.
func New(rt Runtime) *Client {
	c := &Client{http: http.DefaultClient}
	c.Retarget(rt)
	return c
}

// Retarget points c at the backend rt describes, for every holder of c: a
// restarted lich answers on a new token. Sockets already open stay on the old
// backend.
func (c *Client) Retarget(rt Runtime) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.base, c.token = "http://127.0.0.1:"+strconv.Itoa(rt.Port), rt.Token
}

// Answers reports whether the backend c points at is up and takes its token.
func (c *Client) Answers(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("http", "/ping"), nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusNoContent
}

func (c *Client) url(scheme, path string) string {
	c.mu.Lock()
	base, token := c.base, c.token
	c.mu.Unlock()
	u, _ := url.Parse(base)
	u.Scheme = scheme
	u.Path = path
	u.RawQuery = url.Values{"token": {token}}.Encode()
	return u.String()
}

// Call invokes service.Method with positional args and decodes the result
// into out, which may be nil for methods that return nothing.
func (c *Client) Call(ctx context.Context, method string, out any, args ...any) error {
	if args == nil {
		args = []any{}
	}
	body, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("http", "/rpc/"+method), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &failure) != nil || failure.Error == "" {
			failure.Error = string(data)
		}
		return &RemoteError{Method: method, Message: failure.Error, Status: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: decode result: %w", method, err)
	}
	return nil
}

// RemoteError is a call the backend refused. Message is the backend's own
// sentence, the one a person reads; Error adds the method and status for logs.
type RemoteError struct {
	Method, Message string
	Status          int
}

func (e *RemoteError) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Method, e.Message, e.Status)
}

// ErrorText is what a person reads for err, as the web's errorText: the
// backend's sentence when the backend refused, err as it is otherwise.
func ErrorText(err error) string {
	var remote *RemoteError
	if errors.As(err, &remote) {
		return remote.Message
	}
	return err.Error()
}

// Event is one message of the /events stream.
type Event struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// Events connects to /events and delivers its messages until ctx ends or the
// connection drops, then closes the channel. Events sent while no client is
// connected are lost: the backend does not replay them.
func (c *Client) Events(ctx context.Context) (<-chan Event, error) {
	conn, _, err := websocket.Dial(ctx, c.url("ws", "/events"), nil)
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}
	conn.SetReadLimit(maxFrame)
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		defer conn.CloseNow()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var ev Event
			if err := json.Unmarshal(data, &ev); err != nil {
				continue
			}
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// maxFrame bounds one WebSocket message. The backend caps its own reads at
// 1 MiB (internal/terminal/transport.go); output frames carry PTY reads, which
// are far smaller.
const maxFrame = 4 << 20

// Stream is the /ws socket: PTY output of every session in, keystrokes out.
type Stream struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// Terminal connects to /ws and calls onOutput, from its own goroutine, with
// every output frame until ctx ends or the connection drops; done is closed
// then.
func (c *Client) Terminal(ctx context.Context, onOutput func(id string, data []byte)) (s *Stream, done <-chan struct{}, err error) {
	conn, _, err := websocket.Dial(ctx, c.url("ws", "/ws"), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("terminal stream: %w", err)
	}
	conn.SetReadLimit(maxFrame)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer conn.CloseNow()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ != websocket.MessageBinary {
				continue
			}
			id, payload, err := decodeFrame(data)
			if err != nil {
				continue
			}
			onOutput(id, payload)
		}
	}()
	return &Stream{conn: conn}, finished, nil
}

// Send writes keystrokes to session id.
func (s *Stream) Send(ctx context.Context, id string, data []byte) error {
	frame, err := encodeFrame(id, data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Write(ctx, websocket.MessageBinary, frame)
}

// A /ws frame is [id length: 1 byte][id][payload] (internal/terminal/transport.go).
func encodeFrame(id string, payload []byte) ([]byte, error) {
	if len(id) == 0 || len(id) > 255 {
		return nil, fmt.Errorf("session id %q: want 1 to 255 bytes", id)
	}
	frame := make([]byte, 0, 1+len(id)+len(payload))
	frame = append(frame, byte(len(id)))
	frame = append(frame, id...)
	return append(frame, payload...), nil
}

var errShortFrame = errors.New("frame shorter than its id")

func decodeFrame(frame []byte) (id string, payload []byte, err error) {
	if len(frame) == 0 || int(frame[0]) == 0 || len(frame) < 1+int(frame[0]) {
		return "", nil, errShortFrame
	}
	n := int(frame[0])
	return string(frame[1 : 1+n]), frame[1+n:], nil
}
