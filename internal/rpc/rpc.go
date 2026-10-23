// Package rpc exposes the app's Go services to the window over plain HTTP
// on the local transport listener: the window is its own process and never
// imports internal/ (docs/native-window.md). One POST per call:
//
//	POST /rpc/<service>.<Method>?token=...   body: JSON array of arguments
//	→ 200 with the method's JSON result (null when it only returns error)
//	→ 4xx/5xx with {"error": "..."} otherwise
//
// A method declared in api is exposed with Handle, which the compiler holds to
// the method's options and result types: the array carries that one options
// object, or nothing. The rest is reflection over registered services — the
// net/rpc pattern — matching the array to the method's parameters by
// position; a method whose first parameter is a context.Context gets the
// request's. Both hand over a context that is done when the caller hangs up.
// Token auth is applied by the transport mount, not here.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// bodyLimit bounds one RPC request body; arguments are ids, paths and small
// strings, so anything larger is malformed or hostile.
const bodyLimit = 1 << 20

var (
	errType     = reflect.TypeFor[error]()
	contextType = reflect.TypeFor[context.Context]()
)

type Handler struct {
	routes   map[string]route
	services map[string]reflect.Value
	denied   map[string]bool
}

// route answers one api method: the call's result, or badArguments when the
// body does not decode into its options.
type route func(ctx context.Context, body []byte) (any, error)

type badArguments struct{ err error }

func (b badArguments) Error() string { return b.err.Error() }

func New() *Handler {
	return &Handler{
		routes:   make(map[string]route),
		services: make(map[string]reflect.Value),
		denied:   make(map[string]bool),
	}
}

// Handle exposes fn as m. It answers before a Register of the same service,
// so a service moves to api one method at a time.
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

// Register exposes every exported method of svc under name.<Method>.
func (h *Handler) Register(name string, svc any) {
	h.services[name] = reflect.ValueOf(svc)
}

// Deny hides one method ("store.Close") from dispatch — for methods that are
// public for Go callers but must not be reachable from the window.
func (h *Handler) Deny(method string) {
	h.denied[method] = true
}

// ServeHTTP handles one call.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/rpc/")
	if route, ok := h.routes[name]; ok && !h.denied[name] {
		h.serveRoute(w, r, name, route)
		return
	}
	method, err := h.lookup(name)
	if err != nil {
		slog.Warn("rpc: unknown method", "method", name, "err", err)
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	body, ok := readBody(w, r, name)
	if !ok {
		return
	}
	args, err := decodeArgs(r.Context(), method.Type(), body)
	if err != nil {
		slog.Warn("rpc: bad arguments", "method", name, "err", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeResult(w, name, method.Call(args))
}

func (h *Handler) serveRoute(w http.ResponseWriter, r *http.Request, name string, route route) {
	body, ok := readBody(w, r, name)
	if !ok {
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

func readBody(w http.ResponseWriter, r *http.Request, name string) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, bodyLimit))
	if err != nil {
		slog.Warn("rpc: unreadable body", "method", name, "err", err)
		writeError(w, http.StatusBadRequest, "unreadable body")
		return nil, false
	}
	return body, true
}

func (h *Handler) lookup(name string) (reflect.Value, error) {
	if h.denied[name] {
		return reflect.Value{}, fmt.Errorf("method %q not available", name)
	}
	service, methodName, ok := strings.Cut(name, ".")
	if !ok {
		return reflect.Value{}, fmt.Errorf("malformed method %q", name)
	}
	svc, ok := h.services[service]
	if !ok {
		return reflect.Value{}, fmt.Errorf("unknown service %q", service)
	}
	method := svc.MethodByName(methodName)
	if !method.IsValid() {
		return reflect.Value{}, fmt.Errorf("unknown method %q", name)
	}
	return method, nil
}

// decodeArgs unmarshals the JSON argument array positionally into the
// method's parameter types, handing ctx to a leading context.Context.
func decodeArgs(ctx context.Context, t reflect.Type, body []byte) ([]reflect.Value, error) {
	var raw []json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("arguments must be a JSON array: %w", err)
		}
	}
	args := make([]reflect.Value, 0, t.NumIn())
	if t.NumIn() > 0 && t.In(0) == contextType {
		args = append(args, reflect.ValueOf(ctx))
	}
	skipped := len(args)
	if len(raw) != t.NumIn()-skipped {
		return nil, fmt.Errorf("want %d arguments, got %d", t.NumIn()-skipped, len(raw))
	}
	for i, arg := range raw {
		value := reflect.New(t.In(skipped + i))
		if err := json.Unmarshal(arg, value.Interface()); err != nil {
			return nil, fmt.Errorf("argument %d: %w", i, err)
		}
		args = append(args, value.Elem())
	}
	return args, nil
}

// writeResult maps Go returns onto the response: a trailing non-nil error is
// a 500, otherwise the first non-error value (or null) is the 200 body.
// Every service error is also logged with the method name — the RPC is the
// single funnel every client-triggered failure passes through, which makes
// this the one line that turns them into an audit trail.
func writeResult(w http.ResponseWriter, method string, results []reflect.Value) {
	var payload any
	for _, result := range results {
		if result.Type().Implements(errType) {
			if err, _ := result.Interface().(error); err != nil {
				writeReply(w, method, nil, err)
				return
			}
			continue
		}
		if payload == nil {
			payload = result.Interface()
		}
	}
	writeReply(w, method, payload, nil)
}

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
