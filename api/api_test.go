package api

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/contract.golden from the declared methods")

const goldenPath = "testdata/contract.golden"

var marshalerType = reflect.TypeFor[json.Marshaler]()

// wireShapes are the types that encode themselves, each with the struct its
// MarshalJSON writes, which the golden shows in its place.
var wireShapes = map[reflect.Type]reflect.Type{
	reflect.TypeFor[RevertResult](): reflect.TypeFor[revertWire](),
}

// TestContractMatchesGolden makes every change to a method's name, options or
// result, and to an event's name or payload, show up as a diff of
// testdata/contract.golden. A renamed JSON field
// compiles on both sides and still breaks a client of another release; the
// golden is where review sees it. Run `go test ./api -update` after a change
// meant to move the contract.
func TestContractMatchesGolden(t *testing.T) {
	got := render(Methods(), Events())
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	// The repository carries no .gitattributes, so a Windows checkout may hand
	// back CRLF for the golden committed with LF: the checkout's spelling, not
	// drift.
	if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Error("the contract moved; if that is meant, run `go test ./api -update` and review the diff of " + goldenPath)
	}
}

// TestEveryRequestIsOneObject keeps positional arguments out of the contract:
// a request that is a bare string or bool is matched by position, and breaks
// the release its meaning changes.
func TestEveryRequestIsOneObject(t *testing.T) {
	for name, m := range Methods() {
		if m.Request.Kind() != reflect.Struct {
			t.Errorf("%s takes %s; a request is one struct, or None", name, m.Request)
		}
	}
}

// TestWireShapesAreWhatMarshalJSONWrites keeps wireShapes honest: the golden
// shows the wire struct, so it has to be the keys MarshalJSON writes.
func TestWireShapesAreWhatMarshalJSONWrites(t *testing.T) {
	for typ, wire := range wireShapes {
		data, err := json.Marshal(reflect.New(typ).Elem().Interface())
		if err != nil {
			t.Fatal(err)
		}
		var written map[string]json.RawMessage
		if err := json.Unmarshal(data, &written); err != nil {
			t.Fatal(err)
		}
		var keys []string
		for field := range wire.Fields() {
			if name, _, wired := jsonName(field); wired {
				keys = append(keys, name)
			}
		}
		if got := slices.Sorted(maps.Keys(written)); !slices.Equal(got, slices.Sorted(slices.Values(keys))) {
			t.Errorf("%s writes %v, its wire shape %s names %v", typ, got, wire, keys)
		}
	}
}

func TestArgsWrapsOneObjectAndLeavesNoneEmpty(t *testing.T) {
	if got := ThemesSelect.Args(SelectThemeOptions{ID: "nord"}); !reflect.DeepEqual(got, []any{SelectThemeOptions{ID: "nord"}}) {
		t.Errorf("Args(options) = %v, want the options alone", got)
	}
	if got := ThemesList.Args(None{}); len(got) != 0 || got == nil {
		t.Errorf("Args(None) = %#v, want an empty, non-nil list (it encodes as [])", got)
	}
}

func TestDefineRefusesADuplicateName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a second declaration of themes.List did not panic")
		}
	}()
	define[None, None]("themes.List")
}

func TestDefineEventRefusesADuplicateName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a second declaration of session-status did not panic")
		}
	}()
	defineEvent[None]("session-status")
}

func TestEventFamilyNamesOneEventPerSession(t *testing.T) {
	if got := TerminalExitEvents.Of("s1").Name; got != "terminal:exit:s1" {
		t.Errorf("Of(s1) = %q", got)
	}
	if id, ok := TerminalExitEvents.ID("terminal:exit:s1"); !ok || id != "s1" {
		t.Errorf("ID = %q, %v; want s1", id, ok)
	}
	if _, ok := TerminalExitEvents.ID("terminal:data:s1"); ok {
		t.Error("another family's event read as this one's")
	}
}

// render prints each method as `name(Request) Response` and each event as
// `event name Payload`, sorted, followed by every named type they reach, each
// once.
func render(methods map[string]Declared, events map[string]DeclaredEvent) string {
	r := &renderer{seen: map[reflect.Type]bool{}}
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(methods)) {
		m := methods[name]
		fmt.Fprintf(&b, "%s(%s) %s\n", name, r.ref(m.Request), r.ref(m.Response))
	}
	b.WriteString("\n")
	for _, name := range slices.Sorted(maps.Keys(events)) {
		fmt.Fprintf(&b, "event %s %s\n", name, r.ref(events[name].Payload))
	}
	for len(r.queue) > 0 {
		next := r.queue[0]
		r.queue = r.queue[1:]
		b.WriteString("\n")
		b.WriteString(r.define(next))
	}
	return b.String()
}

type renderer struct {
	seen  map[reflect.Type]bool
	queue []reflect.Type
}

// ref names t as a field or a signature shows it, queueing the named types
// of this package for their own definition.
func (r *renderer) ref(t reflect.Type) string {
	if t.Name() != "" && t.PkgPath() == reflect.TypeFor[None]().PkgPath() {
		if !r.seen[t] {
			r.seen[t] = true
			r.queue = append(r.queue, t)
		}
		return t.Name()
	}
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return "base64"
	}
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + r.ref(t.Elem())
	case reflect.Slice:
		return "[]" + r.ref(t.Elem())
	case reflect.Map:
		return "map[" + r.ref(t.Key()) + "]" + r.ref(t.Elem())
	}
	return t.String()
}

func (r *renderer) define(t reflect.Type) string {
	if t.Implements(marshalerType) {
		wire, ok := wireShapes[t]
		if !ok {
			panic(fmt.Sprintf("api: %s encodes itself; add the struct its MarshalJSON writes to wireShapes", t.Name()))
		}
		return r.defineStruct(t.Name()+" (MarshalJSON)", wire)
	}
	if t.Kind() != reflect.Struct {
		return fmt.Sprintf("%s = %s\n", t.Name(), r.ref(reflectUnderlying(t)))
	}
	return r.defineStruct(t.Name(), t)
}

func (r *renderer) defineStruct(name string, t reflect.Type) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s {\n", name)
	for field := range t.Fields() {
		name, omitempty, wired := jsonName(field)
		if !wired {
			continue
		}
		if field.Anonymous && field.Tag.Get("json") == "" {
			fmt.Fprintf(&b, "\t...%s\n", r.ref(field.Type))
			continue
		}
		mark := ""
		if omitempty {
			mark = ",omitempty"
		}
		fmt.Fprintf(&b, "\t%s%s %s\n", name, mark, r.ref(field.Type))
	}
	b.WriteString("}\n")
	return b.String()
}

// reflectUnderlying is t without its name: Syntax's map type.
func reflectUnderlying(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Map:
		return reflect.MapOf(t.Key(), t.Elem())
	case reflect.Slice:
		return reflect.SliceOf(t.Elem())
	}
	panic(fmt.Sprintf("api: the golden does not render named %s types (%s)", t.Kind(), t.Name()))
}

// jsonName is the key encoding/json writes field under, and whether it
// writes the field at all.
func jsonName(field reflect.StructField) (name string, omitempty, wired bool) {
	if !field.IsExported() {
		return "", false, false
	}
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "" {
		name = field.Name
	}
	return name, strings.Contains(opts, "omitempty"), true
}
