// Package rpc exposes the app's Go services to the window, `lich` and `lich
// mcp` over plain HTTP on the local transport listener: the window is its own
// process and never imports internal/ (docs/native-window.md). One POST per
// call:
//
//	POST /rpc/<service>.<Method>?token=...   body: [options] or []
//	→ 200 with the method's JSON result (null when it only returns error)
//	→ 4xx/5xx with {"error": "..."} otherwise
//
// Only a method declared in api and exposed with Handle answers; the compiler
// holds the handler to the method's options and result types. Its context is
// done when the caller hangs up. Token auth is applied by the transport mount,
// not here.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// bodyLimit bounds one RPC request body; options are ids, paths and small
// strings, so anything larger is malformed or hostile.
const bodyLimit = 1 << 20

type Handler struct {
	routes map[string]route
}

// route answers one api method: the call's result, or badArguments when the
// body does not decode into its options.
type route func(ctx context.Context, body []byte) (any, error)

type badArguments struct{ err error }

func (b badArguments) Error() string { return b.err.Error() }

func New() *Handler {
	return &Handler{routes: make(map[string]route)}
}

// Handle exposes fn as m.
func Handle[Req, Resp any](h *Handler, m api.Method[Req, Resp], fn func(context.Context, Req) (Resp, error)) {
	if _, dup := h.routes[m.Name]; dup {
		panic(fmt.Sprintf("rpc: %s handled twice", m.Name))
	}
	h.routes[m.Name] = func(ctx context.Context, body []byte) (any, error) {
		req, err := decodeOptions[Req](body)
		if err != nil {
			return nil, badArguments{err}
		}
		resp, err := fn(ctx, req)
		if err != nil {
			return nil, err
		}
		if _, empty := any(resp).(api.None); empty {
			return nil, nil
		}
		return resp, nil
	}
}

// decodeOptions reads the envelope api.Method.Args writes. An empty one is
// the zero options, which is also what a client from before the method took
// any sends.
func decodeOptions[Req any](body []byte) (Req, error) {
	var req Req
	var raw []json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			return req, fmt.Errorf("arguments must be a JSON array: %w", err)
		}
	}
	switch len(raw) {
	case 0:
		return req, nil
	case 1:
		if err := json.Unmarshal(raw[0], &req); err != nil {
			return req, fmt.Errorf("options: %w", err)
		}
		return req, nil
	}
	return req, fmt.Errorf("want one options object, got %d arguments", len(raw))
}

// ServeHTTP handles one call.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/rpc/")
	route, ok := h.routes[name]
	if !ok {
		slog.Warn("rpc: unknown method", "method", name)
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown method %q", name))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, bodyLimit))
	if err != nil {
		slog.Warn("rpc: unreadable body", "method", name, "err", err)
		writeError(w, http.StatusBadRequest, "unreadable body")
		return
	}
	result, err := route(r.Context(), body)
	var bad badArguments
	if errors.As(err, &bad) {
		slog.Warn("rpc: bad arguments", "method", name, "err", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeReply(w, name, result, err)
}

// writeReply maps a call's outcome onto the response: an error is a 500,
// otherwise the result (or null) is the 200 body. Every service error is also
// logged with the method name — the RPC is the single funnel every
// client-triggered failure passes through, which makes this the one line that
// turns them into an audit trail.
func writeReply(w http.ResponseWriter, method string, payload any, err error) {
	if err != nil {
		slog.Warn("rpc: call failed", "method", method, "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Headers are gone — the response cannot change anymore, but the
		// audit trail can still say the reply never reached the client.
		slog.Warn("rpc: encode response", "method", method, "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
