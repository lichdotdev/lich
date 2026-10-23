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

// TestContractMatchesGolden makes every change to a method's name, options or
// result show up as a diff of testdata/contract.golden. A renamed JSON field
// compiles on both sides and still breaks a client of another release; the
// golden is where review sees it. Run `go test ./api -update` after a change
// meant to move the contract.
func TestContractMatchesGolden(t *testing.T) {
	got := render(Methods())
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
		t.Errorf("the contract moved; if that is meant, run `go test ./api -update` and review the diff.\ngot:\n%s", got)
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

// render prints each method as `name(Request) Response`, sorted, followed by
// every named type they reach, each once.
func render(methods map[string]Declared) string {
	r := &renderer{seen: map[reflect.Type]bool{}}
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(methods)) {
		m := methods[name]
		fmt.Fprintf(&b, "%s(%s) %s\n", name, r.ref(m.Request), r.ref(m.Response))
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
		return fmt.Sprintf("%s = its own MarshalJSON\n", t.Name())
	}
	if t.Kind() != reflect.Struct {
		return fmt.Sprintf("%s = %s\n", t.Name(), r.ref(reflectUnderlying(t)))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s {\n", t.Name())
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
