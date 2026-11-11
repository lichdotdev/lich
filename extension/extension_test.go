package extension

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/ from the declared components and messages")

// fill sets every field of v, so a field nobody wrote an example for still
// reaches the golden. A string holds its JSON name, and a map holds one entry
// keyed by that name. A type nested in itself (a menu's submenu) stops at the
// second level.
func fill(v reflect.Value, name string, parents ...reflect.Type) {
	if slices.Contains(parents, v.Type()) {
		return
	}
	if v.Kind() == reflect.Struct {
		parents = append(parents, v.Type())
	}
	if v.Type() == reflect.TypeFor[Node]() {
		v.Set(reflect.ValueOf(Separator.New(NoProps{})))
		return
	}
	if v.Type() == reflect.TypeFor[json.RawMessage]() {
		v.SetBytes([]byte(`{}`))
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(name)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint8:
		v.SetUint(1)
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		value := reflect.New(v.Type().Elem()).Elem()
		fill(value, name, parents...)
		v.SetMapIndex(reflect.ValueOf(name).Convert(v.Type().Key()), value)
	case reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem(), name, parents...)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fill(v.Index(0), name, parents...)
	case reflect.Struct:
		for i := range v.NumField() {
			field := v.Type().Field(i)
			fill(v.Field(i), strings.Split(field.Tag.Get("json"), ",")[0], parents...)
		}
	default:
		panic("extension: the golden does not render " + v.Type().String())
	}
}

func filled(d declaration) any {
	v := reflect.New(d.shape).Elem()
	fill(v, d.shape.Name())
	return v.Interface()
}

func sortedNames(in map[string]declaration) []string {
	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func checkGolden(t *testing.T, file string, value any) {
	t.Helper()
	got, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", file)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Errorf("the contract moved; if that is meant, run `go test ./extension -update` and review the diff of %s", path)
	}
}

func everyComponent() []Node {
	var nodes []Node
	for _, name := range sortedNames(components) {
		child := Separator.New(NoProps{})
		nodes = append(nodes, Node{Type: name, Key: "key", Props: filled(components[name]), Children: []Node{child}})
	}
	return nodes
}

func everyMessage() []Message {
	var all []Message
	for _, name := range sortedNames(messages) {
		all = append(all, Message{Version: ProtocolVersion, Kind: name, Data: filled(messages[name])})
	}
	return all
}

func TestComponentsMatchGolden(t *testing.T) {
	checkGolden(t, "components.golden.json", everyComponent())
}
func TestMessagesMatchGolden(t *testing.T) { checkGolden(t, "messages.golden.json", everyMessage()) }
func TestEnumsMatchGolden(t *testing.T)    { checkGolden(t, "enums.golden.json", EnumValues()) }

func TestManifestMatchesGolden(t *testing.T) {
	checkGolden(t, "manifest.golden.json", filled(declarationOf[Manifest]()))
}

func TestEveryComponentRoundTrips(t *testing.T) {
	for _, want := range everyComponent() {
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got Node
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%s: %v", want.Type, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s came back as %+v, want %+v", want.Type, got, want)
		}
	}
}

func TestEveryMessageRoundTrips(t *testing.T) {
	for _, want := range everyMessage() {
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got Message
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%s: %v", want.Kind, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s came back as %+v, want %+v", want.Kind, got, want)
		}
	}
}

func TestBuildersWriteTheDeclaredShape(t *testing.T) {
	msg := Render.New(RenderData{Surface: "main", Root: Box.New(BoxProps{Gap: 4},
		Button.New(ButtonProps{Text: "Run", OnPress: "run"}).WithKey("run"))})
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"v":1,"kind":"render","data":{"surface":"main","root":{"type":"box","props":{"gap":4},` +
		`"children":[{"type":"button","key":"run","props":{"text":"Run","onPress":"run"}}]}}}`
	if string(raw) != want {
		t.Errorf("got  %s\nwant %s", raw, want)
	}
}

func TestRefusesWhatThisBuildDoesNotSpeak(t *testing.T) {
	for name, tc := range map[string]struct {
		json   string
		target any
		want   string
	}{
		"unknown component":   {`{"type":"hologram","props":{}}`, &Node{}, `unknown component "hologram"`},
		"unknown nested":      {`{"type":"box","props":{},"children":[{"type":"hologram"}]}`, &Node{}, `unknown component "hologram"`},
		"props of wrong type": {`{"type":"progress","props":{"percent":"ten"}}`, &Node{}, `component "progress"`},
		"unknown message":     {`{"v":1,"kind":"teleport","data":{}}`, &Message{}, `unknown message "teleport"`},
		"newer protocol":      {`{"v":2,"kind":"toast","data":{}}`, &Message{}, "protocol version 2"},
	} {
		t.Run(name, func(t *testing.T) {
			err := json.Unmarshal([]byte(tc.json), tc.target)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got error %v, want one naming %q", err, tc.want)
			}
		})
	}
}
