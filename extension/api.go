package extension

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
)

// Permission names a part of lich an extension reads or acts on through calls
// and events. The manifest lists every one it uses, and lich refuses a call or
// a subscription that needs one it did not list.
type Permission string

const (
	// PermissionProjectsRead lists the open projects and their worktrees.
	PermissionProjectsRead Permission = "projects:read"
	// PermissionSessionsRead lists the live sessions and what each is doing.
	PermissionSessionsRead Permission = "sessions:read"
	// PermissionSessionsWrite opens sessions and types prompts into them.
	PermissionSessionsWrite Permission = "sessions:write"
	// PermissionSessionEvents subscribes to session.status.
	PermissionSessionEvents Permission = "events:session"
)

// Permissions is every permission this build knows, the names a manifest may list.
var Permissions = []Permission{
	PermissionProjectsRead, PermissionSessionsRead, PermissionSessionsWrite, PermissionSessionEvents,
}

var (
	// CallMethod asks lich to run a method. lich answers every call with a
	// CallResult under the same ID, an error included: an unknown method, a
	// permission the manifest does not list, params that do not decode.
	CallMethod = kind[CallData]("call")
	CallResult = kind[ResultData]("result")
	// LichEvent is an event the extension subscribed to with Subscribe.
	LichEvent = kind[LichEventData]("lich.event")
)

// CallData is one call. ID is the extension's to choose and comes back on
// the result; two calls in flight should not share one.
type CallData struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// ResultData answers the call with ID: Data on success, Error otherwise.
type ResultData struct {
	ID    string          `json:"id"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// LichEventData is one event: Name is an Event's name, Data its payload.
type LichEventData struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Method names a call whose params are P and whose result is R, and the
// permission it needs; empty needs none.
type Method[P, R any] struct {
	Name       string
	Permission Permission
}

// Call returns the message asking for m under id.
func (m Method[P, R]) Call(id string, params P) Message {
	return CallMethod.New(CallData{ID: id, Method: m.Name, Params: encode(params)})
}

// Params decodes what a call to m carries.
func (m Method[P, R]) Params(call CallData) (P, error) {
	var params P
	if len(call.Params) == 0 {
		return params, nil
	}
	if err := json.Unmarshal(call.Params, &params); err != nil {
		return params, fmt.Errorf("%s: params: %w", m.Name, err)
	}
	return params, nil
}

// Answer returns the result of the call id carrying data.
func (m Method[P, R]) Answer(id string, data R) Message {
	return CallResult.New(ResultData{ID: id, Data: encode(data)})
}

// Result decodes the answer to a call to m, its error as a Go error.
func (m Method[P, R]) Result(res ResultData) (R, error) {
	var data R
	if res.Error != "" {
		return data, fmt.Errorf("%s: %w", m.Name, errors.New(res.Error))
	}
	if err := json.Unmarshal(res.Data, &data); err != nil {
		return data, fmt.Errorf("%s: result: %w", m.Name, err)
	}
	return data, nil
}

// Event names an event whose payload is D, and the permission subscribing needs.
type Event[D any] struct {
	Name       string
	Permission Permission
}

// New returns the message carrying data as e.
func (e Event[D]) New(data D) Message {
	return LichEvent.New(LichEventData{Name: e.Name, Data: encode(data)})
}

// Decode reads the payload of ev, which must be e.
func (e Event[D]) Decode(ev LichEventData) (D, error) {
	var data D
	if ev.Name != e.Name {
		return data, fmt.Errorf("extension: event %q is not %q", ev.Name, e.Name)
	}
	err := json.Unmarshal(ev.Data, &data)
	return data, err
}

// encode marshals a value of a type this package declares, which always
// encodes: no channel, function or map with a non-string key is among them.
func encode(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("extension: %T does not encode: %v", v, err))
	}
	return raw
}

type methodDeclaration struct {
	params, result reflect.Type
	permission     Permission
}

type eventDeclaration struct {
	data       reflect.Type
	permission Permission
}

var (
	methods = map[string]methodDeclaration{}
	events  = map[string]eventDeclaration{}
)

func method[P, R any](name string, permission Permission) Method[P, R] {
	if _, dup := methods[name]; dup {
		panic(fmt.Sprintf("extension: method %s declared twice", name))
	}
	methods[name] = methodDeclaration{params: reflect.TypeFor[P](), result: reflect.TypeFor[R](), permission: permission}
	return Method[P, R]{Name: name, Permission: permission}
}

func event[D any](name string, permission Permission) Event[D] {
	if _, dup := events[name]; dup {
		panic(fmt.Sprintf("extension: event %s declared twice", name))
	}
	events[name] = eventDeclaration{data: reflect.TypeFor[D](), permission: permission}
	return Event[D]{Name: name, Permission: permission}
}

// MethodNames is every method this build declares, sorted.
func MethodNames() []string {
	return sortedKeys(methods)
}

// EventPermission is the permission subscribing to the event name needs, and
// whether this build declares that event.
func EventPermission(name string) (Permission, bool) {
	decl, ok := events[name]
	return decl.permission, ok
}

func sortedKeys[V any](in map[string]V) []string {
	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
