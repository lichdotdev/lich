// Package api is the contract between the lich backend and its clients: the
// window (native/backend), `lich` and `lich mcp`. Each RPC is declared here
// once, as a Method whose type parameters are what it takes and what it
// returns, so a client and the backend that disagree on either do not compile.
//
// On the wire a call is a JSON array holding its one options object, or empty
// for a method that takes nothing: the envelope internal/rpc has always read,
// which keeps a `lich` of another release talking to this backend. Fields are
// matched by name, never by position, so an option one side does not know is
// dropped and one it does not send is its zero value.
//
// The package is pure Go with no dependency beyond the standard library: the
// window's module imports it, and must not reach internal/.
// testdata/contract.golden records every method's shape; a change to it is a
// change to the contract, visible in review.
package api

import (
	"fmt"
	"reflect"
)

// Method names one RPC. Req is its options object (None for a method that
// takes nothing) and Resp its result (None for one that only fails or not).
type Method[Req, Resp any] struct {
	Name string
}

// None is the request of a method that takes nothing and the result of one
// that returns only an error.
type None struct{}

// Args is req in the wire envelope.
func (m Method[Req, Resp]) Args(req Req) []any {
	if _, empty := any(req).(None); empty {
		return []any{}
	}
	return []any{req}
}

// Declared is one Method with its type parameters as values, for code that
// walks the whole contract (the golden test).
type Declared struct {
	Name     string
	Request  reflect.Type
	Response reflect.Type
}

var declared = map[string]Declared{}

// Methods returns every Method this package declares.
func Methods() map[string]Declared {
	return declared
}

func define[Req, Resp any](name string) Method[Req, Resp] {
	if _, dup := declared[name]; dup {
		panic(fmt.Sprintf("api: %s declared twice", name))
	}
	declared[name] = Declared{Name: name, Request: reflect.TypeFor[Req](), Response: reflect.TypeFor[Resp]()}
	return Method[Req, Resp]{Name: name}
}
