package extensions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/lichdotdev/lich/extension"
)

const (
	// maxLineBytes bounds a line from an extension and the event it becomes.
	// The window's /events socket reads at most 4 MiB a frame (maxFrame in
	// native/backend/client.go) and one frame over it ends that socket for
	// every session, so this leaves room for the envelope.
	maxLineBytes = 3 << 20
	// maxTreeNodes and maxTreeDepth bound what one render costs the window,
	// which walks the tree on its UI goroutine every frame, recursively:
	// nodes bound the frame time and depth the stack.
	maxTreeNodes = 5000
	maxTreeDepth = 64
	// maxJSONNesting is checked before decoding, because each Node's
	// UnmarshalJSON re-scans its whole subtree: decoding costs depth times
	// size, so a line must not get there on its depth alone. A node costs at
	// most 4 levels (node, props, entries, entry, lead) and the rest is room
	// for the message envelope and submenus.
	maxJSONNesting = 8 * maxTreeDepth

	// maxLogLineBytes bounds one stderr line, or a refusal's reason, in lich's log.
	maxLogLineBytes = 4 << 10
	// maxLogLines per logWindow is what one extension can write of each kind
	// to lich's log.
	maxLogLines = 100
	logWindow   = time.Minute

	msgRefused = "extensions: refused a line"
	msgStderr  = "extensions: stderr"
)

// fromExtension is what an extension may write; anything else is refused,
// including a kind added to the contract later and not yet considered here.
var fromExtension = map[string]bool{
	extension.Render.Name:       true,
	extension.ShowToast.Name:    true,
	extension.DismissToast.Name: true,
	extension.OpenExternal.Name: true,
	extension.Clipboard.Name:    true,
	extension.Focus.Name:        true,
	extension.CallMethod.Name:   true,
}

// toExtension is what extensions.Send may write to one.
var toExtension = map[string]bool{
	extension.FireEvent.Name:     true,
	extension.SurfaceOpened.Name: true,
	extension.SurfaceClosed.Name: true,
	extension.Language.Name:      true,
}

// handleLine vets one line of stdout. A bad line is refused and logged, and
// the extension goes on: one bug in it should not cost the user its screen.
func (s *Service) handleLine(x *installed, line []byte) {
	msg, encoded, err := vetMessage(line)
	if err != nil {
		x.refused.note(x.id, msgRefused, err.Error())
		return
	}
	if call, ok := msg.Data.(extension.CallData); ok {
		s.call(x, call)
		return
	}
	if render, ok := msg.Data.(extension.RenderData); ok {
		x.renders.submit(x.throttleKey(render.Surface), encoded, func(m json.RawMessage) { s.deliver(x, m) })
		return
	}
	s.deliver(x, encoded)
}

func vetLine(line []byte) (json.RawMessage, error) {
	_, encoded, err := vetMessage(line)
	return encoded, err
}

// vetMessage returns a line's message and its event payload, re-encoded from
// what was decoded so the window can only receive what was checked: no unknown
// field or duplicate key survives.
func vetMessage(line []byte) (extension.Message, json.RawMessage, error) {
	var msg extension.Message
	if nesting := jsonNesting(line); nesting > maxJSONNesting {
		return msg, nil, fmt.Errorf("JSON nested %d levels, the limit is %d", nesting, maxJSONNesting)
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return msg, nil, err
	}
	if !fromExtension[msg.Kind] {
		return msg, nil, fmt.Errorf("an extension may not send %q", msg.Kind)
	}
	if open, ok := msg.Data.(extension.OpenExternalData); ok {
		if err := checkExternalURL(open.URL); err != nil {
			return msg, nil, err
		}
	}
	if err := (&treeWalk{}).visit(reflect.ValueOf(msg.Data), 0); err != nil {
		return msg, nil, err
	}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return msg, nil, err
	}
	// Encoding can grow a line that fit: every < becomes <.
	if len(encoded) > maxLineBytes {
		return msg, nil, fmt.Errorf("message encodes to %d bytes, the limit is %d", len(encoded), maxLineBytes)
	}
	return msg, encoded, nil
}

func encodeOutbound(raw json.RawMessage) ([]byte, error) {
	var msg extension.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}
	if !toExtension[msg.Kind] {
		return nil, fmt.Errorf("%q cannot be sent to an extension", msg.Kind)
	}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func checkExternalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("openExternal: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("openExternal needs an http or https URL, got %q", raw)
	}
	return nil
}

// jsonNesting is the deepest bracket nesting of a line, outside strings.
func jsonNesting(line []byte) int {
	depth, deepest := 0, 0
	inString, escaped := false, false
	for _, c := range line {
		switch {
		case escaped:
			escaped = false
		case inString:
			escaped = c == '\\'
			inString = c != '"'
		case c == '"':
			inString = true
		case c == '{' || c == '[':
			depth++
			deepest = max(deepest, depth)
		case c == '}' || c == ']':
			depth--
		}
	}
	return deepest
}

var nodeType = reflect.TypeFor[extension.Node]()

// treeWalk counts every Node reachable from a message by reflection, because a
// node can hide in any prop (a tooltip's content, a menu entry's lead) and the
// component list keeps growing: a prop the walk did not know would be a way
// around the limits.
type treeWalk struct {
	nodes int
}

func (w *treeWalk) visit(v reflect.Value, depth int) error {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return w.visit(v.Elem(), depth)
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if err := w.visit(v.Index(i), depth); err != nil {
				return err
			}
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			if err := w.visit(it.Value(), depth); err != nil {
				return err
			}
		}
	case reflect.Struct:
		return w.visitStruct(v, depth)
	}
	return nil
}

func (w *treeWalk) visitStruct(v reflect.Value, depth int) error {
	if v.Type() == nodeType {
		depth++
		w.nodes++
		if depth > maxTreeDepth {
			return fmt.Errorf("tree deeper than %d nodes", maxTreeDepth)
		}
		if w.nodes > maxTreeNodes {
			return fmt.Errorf("tree over %d nodes", maxTreeNodes)
		}
	}
	for i := range v.NumField() {
		if err := w.visit(v.Field(i), depth); err != nil {
			return err
		}
	}
	return nil
}

// framer splits a stream into lines holding at most max bytes of one in
// memory: an endless line is dropped as it arrives and reported once. A final
// line with no newline never completes and is not delivered.
type framer struct {
	max        int
	buf        []byte
	discarding bool
	onLine     func([]byte)
	onTooLong  func()
}

func newFramer(limit int, onLine func([]byte), onTooLong func()) *framer {
	return &framer{max: limit, onLine: onLine, onTooLong: onTooLong}
}

func (f *framer) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		chunk := p
		if i >= 0 {
			chunk = p[:i]
		}
		f.take(chunk)
		if i < 0 {
			break
		}
		if !f.discarding {
			f.onLine(f.buf)
		}
		f.buf, f.discarding = f.buf[:0], false
		p = p[i+1:]
	}
	return n, nil
}

func (f *framer) take(chunk []byte) {
	if f.discarding {
		return
	}
	if len(f.buf)+len(chunk) > f.max {
		f.discarding, f.buf = true, f.buf[:0]
		f.onTooLong()
		return
	}
	f.buf = append(f.buf, chunk...)
}

// logBudget keeps a chatty extension from filling lich's log, one window of
// maxLogLines at a time.
type logBudget struct {
	mu      sync.Mutex
	opened  time.Time
	used    int
	dropped int
}

func (b *logBudget) note(id, msg, text string) {
	if len(text) > maxLogLineBytes {
		text = text[:maxLogLineBytes]
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if now := time.Now(); now.Sub(b.opened) >= logWindow {
		if b.dropped > 0 {
			slog.Warn(msg, "extension", id, "droppedLines", b.dropped)
		}
		b.opened, b.used, b.dropped = now, 0, 0
	}
	if b.used >= maxLogLines {
		b.dropped++
		return
	}
	b.used++
	slog.Warn(msg, "extension", id, "text", text)
}
