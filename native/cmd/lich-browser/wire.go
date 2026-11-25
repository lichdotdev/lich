package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"

	"github.com/lichdotdev/lich/browserwire"
)

// maxLine bounds one request: a script or a list of file paths, far below.
const maxLine = 64 << 20

var errNotARequest = errors.New("not a request")

// wire is lich-browser's side of the backend's pipe: requests in, one JSON
// line each, and answers and events out.
type wire struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func newWire(out io.Writer) *wire {
	return &wire{enc: json.NewEncoder(out)}
}

func (w *wire) send(m browserwire.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.enc.Encode(m); err != nil {
		log.Printf("wire: %v", err)
	}
}

// reply answers request id with result, or with err when it failed.
func (w *wire) reply(id int64, result any, err error) {
	m := browserwire.Message{ID: id}
	if err != nil {
		m.Error = err.Error()
		w.send(m)
		return
	}
	if result == nil {
		result = struct{}{}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		m.Error = err.Error()
	} else {
		m.Result = raw
	}
	w.send(m)
}

func (w *wire) event(name string, params any) {
	raw, err := json.Marshal(params)
	if err != nil {
		log.Printf("wire: event %s: %v", name, err)
		return
	}
	w.send(browserwire.Message{Event: name, Params: raw})
}

// read hands each request in r to handle, in order, until r ends. A line
// that is no request is answered when it has an id and logged otherwise.
func (w *wire) read(r io.Reader, handle func(browserwire.Request)) error {
	lines := bufio.NewScanner(r)
	lines.Buffer(make([]byte, 0, 64<<10), maxLine)
	for lines.Scan() {
		var req browserwire.Request
		if err := json.Unmarshal(lines.Bytes(), &req); err != nil || req.Method == "" {
			if req.ID != 0 {
				w.reply(req.ID, nil, errNotARequest)
			}
			log.Printf("wire: not a request: %.200s", lines.Bytes())
			continue
		}
		handle(req)
	}
	return lines.Err()
}
