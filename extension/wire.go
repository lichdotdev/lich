// Package extension is the contract between lich and an extension: the UI
// tree an extension asks lich to draw, the toasts it raises, and the events
// lich sends back. lich draws every node with its own UI kit.
//
// Lengths are dp and text sizes sp. A color is a theme token ("muted-foreground")
// or "#rrggbb[aa]". An icon is a Lucide name, or "lobe:name" or "devicon:name".
//
// The package is pure Go with no dependency beyond the standard library, since
// both lich and every extension compile against it. testdata/ records the JSON
// of every component and message; a change there is a change to the contract.
package extension

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ProtocolVersion is the version of the JSON this package writes.
const ProtocolVersion = 1

// Handler names an event the extension handles; empty means none.
type Handler string

// Icon names a glyph, see the package comment.
type Icon string

// Node is one element of a tree lich draws. Key keeps a node's state, such as
// a field's cursor or a list's scroll, while its siblings change.
type Node struct {
	Type     string `json:"type"`
	Key      string `json:"key,omitempty"`
	Props    any    `json:"props"`
	Children []Node `json:"children,omitempty"`
}

// WithKey returns the node with its key set.
func (n Node) WithKey(key string) Node {
	n.Key = key
	return n
}

// UnmarshalJSON decodes Props into the type its component declares.
func (n *Node) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type     string          `json:"type"`
		Key      string          `json:"key"`
		Props    json.RawMessage `json:"props"`
		Children []Node          `json:"children"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	decl, ok := components[wire.Type]
	if !ok {
		return fmt.Errorf("extension: unknown component %q", wire.Type)
	}
	props, err := decl.decode(wire.Props)
	if err != nil {
		return fmt.Errorf("extension: component %q: %w", wire.Type, err)
	}
	*n = Node{Type: wire.Type, Key: wire.Key, Props: props, Children: wire.Children}
	return nil
}

// Component names a node type whose props are P.
type Component[P any] struct {
	Name string
}

// New returns a node of this component.
func (c Component[P]) New(props P, children ...Node) Node {
	return Node{Type: c.Name, Props: props, Children: children}
}

// Kind names a message whose data is D.
type Kind[D any] struct {
	Name string
}

// New returns a message of this kind.
func (k Kind[D]) New(data D) Message {
	return Message{Version: ProtocolVersion, Kind: k.Name, Data: data}
}

// Message is one line of the conversation between lich and an extension.
type Message struct {
	Version int    `json:"v"`
	Kind    string `json:"kind"`
	Data    any    `json:"data"`
}

// UnmarshalJSON decodes Data into the type its kind declares.
func (m *Message) UnmarshalJSON(data []byte) error {
	var wire struct {
		Version int             `json:"v"`
		Kind    string          `json:"kind"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Version != ProtocolVersion {
		return fmt.Errorf("extension: protocol version %d, this build speaks %d", wire.Version, ProtocolVersion)
	}
	decl, ok := messages[wire.Kind]
	if !ok {
		return fmt.Errorf("extension: unknown message %q", wire.Kind)
	}
	decoded, err := decl.decode(wire.Data)
	if err != nil {
		return fmt.Errorf("extension: message %q: %w", wire.Kind, err)
	}
	*m = Message{Version: wire.Version, Kind: wire.Kind, Data: decoded}
	return nil
}

type declaration struct {
	shape  reflect.Type
	decode func(json.RawMessage) (any, error)
}

var (
	components = map[string]declaration{}
	messages   = map[string]declaration{}
)

func declarationOf[T any]() declaration {
	return declaration{shape: reflect.TypeFor[T](), decode: func(raw json.RawMessage) (any, error) {
		var value T
		if len(raw) == 0 {
			return value, nil
		}
		err := json.Unmarshal(raw, &value)
		return value, err
	}}
}

func register[T any](into map[string]declaration, name string) {
	if _, dup := into[name]; dup {
		panic(fmt.Sprintf("extension: %s declared twice", name))
	}
	into[name] = declarationOf[T]()
}

func component[P any](name string) Component[P] {
	register[P](components, name)
	return Component[P]{Name: name}
}

func kind[D any](name string) Kind[D] {
	register[D](messages, name)
	return Kind[D]{Name: name}
}
