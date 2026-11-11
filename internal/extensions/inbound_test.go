package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func encodeLine(t *testing.T, msg extension.Message) []byte {
	t.Helper()
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func render(root extension.Node) extension.Message {
	return extension.Render.New(extension.RenderData{Surface: "main", Root: root})
}

func box(children ...extension.Node) extension.Node {
	return extension.Box.New(extension.BoxProps{}, children...)
}

func chain(depth int, wrap func(extension.Node) extension.Node) extension.Node {
	node := box()
	for range depth - 1 {
		node = wrap(node)
	}
	return node
}

// wrappers are the ways one node holds another: every one has to count.
var wrappers = map[string]func(extension.Node) extension.Node{
	"children": func(n extension.Node) extension.Node { return box(n) },
	"tooltip content": func(n extension.Node) extension.Node {
		return extension.Tooltip.New(extension.TooltipProps{Content: &n})
	},
	"panel trigger": func(n extension.Node) extension.Node {
		return extension.DropdownPanel.New(extension.DropdownPanelProps{Trigger: n})
	},
	"dialog actions": func(n extension.Node) extension.Node {
		return extension.Dialog.New(extension.DialogProps{Actions: []extension.Node{n}})
	},
	"menu entry lead": func(n extension.Node) extension.Node {
		return extension.DropdownMenu.New(extension.DropdownMenuProps{Entries: []extension.MenuEntry{{Lead: &n}}})
	},
	"submenu entry lead": func(n extension.Node) extension.Node {
		sub := []extension.MenuEntry{{Lead: &n}}
		return extension.DropdownMenu.New(extension.DropdownMenuProps{Entries: []extension.MenuEntry{{Sub: sub}}})
	},
}

func TestTreeDepthLimit(t *testing.T) {
	for name, wrap := range wrappers {
		t.Run(name, func(t *testing.T) {
			if _, err := vetLine(encodeLine(t, render(chain(maxTreeDepth, wrap)))); err != nil {
				t.Errorf("a tree at the limit was refused: %v", err)
			}
			_, err := vetLine(encodeLine(t, render(chain(maxTreeDepth+1, wrap))))
			if err == nil || !strings.Contains(err.Error(), "deeper") {
				t.Errorf("a tree over the limit: err = %v", err)
			}
		})
	}
}

func TestTreeNodeLimit(t *testing.T) {
	children := make([]extension.Node, maxTreeNodes-1)
	for i := range children {
		children[i] = box()
	}
	if _, err := vetLine(encodeLine(t, render(box(children...)))); err != nil {
		t.Errorf("a tree at the limit was refused: %v", err)
	}
	over := extension.Dialog.New(extension.DialogProps{Actions: append(children, box())})
	_, err := vetLine(encodeLine(t, render(over)))
	if err == nil || !strings.Contains(err.Error(), "over") {
		t.Errorf("nodes held in a prop escaped the count: err = %v", err)
	}
}

func TestRefusedKinds(t *testing.T) {
	for _, line := range [][]byte{
		encodeLine(t, extension.FireEvent.New(extension.EventData{Handler: "x"})),
		encodeLine(t, extension.SurfaceOpened.New(extension.SurfaceData{Surface: "main"})),
		encodeLine(t, extension.SurfaceClosed.New(extension.SurfaceData{Surface: "main"})),
	} {
		_, err := vetLine(line)
		if err == nil || !strings.Contains(err.Error(), "may not send") {
			t.Errorf("%s: err = %v", line, err)
		}
	}
}

func TestAcceptedKinds(t *testing.T) {
	for _, msg := range []extension.Message{
		render(box()),
		extension.ShowToast.New(extension.ToastData{Title: "t"}),
		extension.DismissToast.New(extension.DismissToastData{ID: "t"}),
		extension.OpenExternal.New(extension.OpenExternalData{URL: "https://example.com/a?b=c"}),
		extension.Clipboard.New(extension.ClipboardData{Text: "t"}),
	} {
		if _, err := vetLine(encodeLine(t, msg)); err != nil {
			t.Errorf("%s: %v", msg.Kind, err)
		}
	}
}

func TestOpenExternalURLs(t *testing.T) {
	for raw, ok := range map[string]bool{
		"http://example.com":      true,
		"https://example.com/x":   true,
		"HTTPS://example.com":     true,
		"":                        false,
		"https:":                  false,
		"https:///nohost":         false,
		"example.com":             false,
		"javascript:alert(1)":     false,
		"file:///etc/passwd":      false,
		"ms-settings:":            false,
		"ssh://example.com":       false,
		" https://example.com":    false,
		"https://exa\nmple.com/x": false,
	} {
		_, err := vetLine(encodeLine(t, extension.OpenExternal.New(extension.OpenExternalData{URL: raw})))
		if (err == nil) != ok {
			t.Errorf("%q: err = %v, accepted should be %v", raw, err, ok)
		}
	}
}

func TestMalformedLines(t *testing.T) {
	unknownComponent := `{"v":1,"kind":"render","data":{"surface":"main","root":{"type":"nope"}}}`
	wrongVersion := `{"v":99,"kind":"toast","data":{"title":"t"}}`
	for _, line := range []string{"", "not json", `{"v":1,"kind":"nope"}`, unknownComponent, wrongVersion} {
		if _, err := vetLine([]byte(line)); err == nil {
			t.Errorf("%q was accepted", line)
		}
	}
}

// Decoding costs depth times size, so a deep line is turned away unread.
func TestJSONNestingIsRefusedBeforeDecoding(t *testing.T) {
	line := strings.Repeat(`{"a":[`, maxJSONNesting)
	_, err := vetLine([]byte(line))
	if err == nil || !strings.Contains(err.Error(), "nested") {
		t.Errorf("err = %v", err)
	}
}

func TestJSONNestingIgnoresStrings(t *testing.T) {
	cases := map[string]int{
		`{"a":"[[[{{{"}`:      1,
		`{"a":"\"[["}`:        1,
		`{"a":"\\","b":[[]]}`: 3,
		`[{}, {"a":[]}]`:      3,
	}
	for line, want := range cases {
		if got := jsonNesting([]byte(line)); got != want {
			t.Errorf("%s: nesting %d, want %d", line, got, want)
		}
	}
}

func TestLineThatGrowsPastTheLimitIsRefused(t *testing.T) {
	line := encodeLine(t, extension.ShowToast.New(extension.ToastData{Title: "x"}))
	huge := strings.Repeat("<", maxLineBytes/2)
	line = bytes.Replace(line, []byte(`"title":"x"`), []byte(`"title":"`+huge+`"`), 1)
	if len(line) > maxLineBytes {
		t.Fatalf("the fixture is %d bytes, over the line limit already", len(line))
	}
	_, err := vetLine(line)
	if err == nil || !strings.Contains(err.Error(), "encodes to") {
		t.Errorf("err = %v", err)
	}
}

func TestAcceptedLineLosesWhatWasNotChecked(t *testing.T) {
	raw, err := vetLine([]byte(`{"v":1,"kind":"toast","extra":"x","data":{"title":"t","junk":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "extra") || strings.Contains(string(raw), "junk") {
		t.Errorf("forwarded unchecked fields: %s", raw)
	}
}

func TestFramerSplitsAcrossWrites(t *testing.T) {
	var got []string
	f := newFramer(16, func(line []byte) { got = append(got, string(line)) }, func() { t.Error("too long") })
	for _, part := range []string{"ab", "c\nde", "\n\nf"} {
		if n, err := f.Write([]byte(part)); n != len(part) || err != nil {
			t.Errorf("Write(%q) = %d, %v", part, n, err)
		}
	}
	if want := []string{"abc", "de", ""}; !slices.Equal(got, want) {
		t.Errorf("lines %q, want %q", got, want)
	}
}

func TestFramerSurvivesAnEndlessLine(t *testing.T) {
	var got []string
	tooLong := 0
	f := newFramer(8, func(line []byte) { got = append(got, string(line)) }, func() { tooLong++ })
	chunk := bytes.Repeat([]byte("x"), 1024)
	for range 1024 {
		_, _ = f.Write(chunk)
		if len(f.buf) > f.max {
			t.Fatalf("buffered %d bytes past the limit of %d", len(f.buf), f.max)
		}
	}
	_, _ = f.Write([]byte("tail\nok\n"))
	if tooLong != 1 || !slices.Equal(got, []string{"ok"}) {
		t.Errorf("tooLong = %d, lines %q", tooLong, got)
	}
}

func TestLineAtTheLimitFitsTheFramer(t *testing.T) {
	var got int
	f := newFramer(8, func(line []byte) { got = len(line) }, func() { t.Error("refused a line at the limit") })
	_, _ = f.Write([]byte("12345678\n"))
	if got != 8 {
		t.Errorf("line of %d bytes", got)
	}
}

// One real process writes every kind of bad line: each is refused, none costs
// it its life, and what is good still arrives in order.
func TestBadLinesAreRefusedWithoutKillingTheExtension(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeScript)
	deep := encodeLine(t, render(chain(maxTreeDepth+1, wrappers["children"])))
	var script bytes.Buffer
	for _, line := range [][]byte{
		[]byte(toastLine("first")),
		bytes.Repeat([]byte("x"), maxLineBytes+1),
		deep,
		[]byte("not json"),
		encodeLine(t, extension.FireEvent.New(extension.EventData{Handler: "x"})),
		encodeLine(t, extension.OpenExternal.New(extension.OpenExternalData{URL: "file:///etc/passwd"})),
		[]byte(toastLine("last")),
	} {
		script.Write(line)
		script.WriteByte('\n')
	}
	writeFile(t, filepath.Join(root, scriptFile), script.String())
	s, r := boot(t, dir)

	waitUntil(t, "the last good line", func() bool { return slices.Contains(r.titles(t), "last") })
	if got, want := r.titles(t), []string{"first", "last"}; !slices.Equal(got, want) {
		t.Errorf("delivered %q, want %q", got, want)
	}
	if runCount(root) != 1 {
		t.Errorf("ran %d times; a bad line must not cost a restart", runCount(root))
	}
	if st := r.lastStatus(); st.Status != api.ExtensionRunning {
		t.Errorf("status = %+v", st)
	}
	if _, err := s.Send(context.Background(), sendOptions(t, "ext", extension.SurfaceOpened.New(extension.SurfaceData{Surface: "main"}))); err != nil {
		t.Errorf("the extension no longer takes messages: %v", err)
	}
	waitUntil(t, "the echo", func() bool { return slices.Contains(r.titles(t), "got surface.opened") })
}

func TestMessageEventCarriesExtensionID(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeScript)
	writeFile(t, filepath.Join(root, scriptFile), toastLine("hello")+"\n")
	_, r := boot(t, dir)
	waitUntil(t, "the toast", func() bool { return len(r.titles(t)) > 0 })

	r.mu.Lock()
	defer r.mu.Unlock()
	if got := r.messages[0].ExtensionID; got != "ext" {
		t.Errorf("event names extension %q", got)
	}
	if !json.Valid(r.messages[0].Message) {
		t.Errorf("event carries invalid JSON: %s", r.messages[0].Message)
	}
}

func sendOptions(t *testing.T, id string, msg extension.Message) api.ExtensionSendOptions {
	t.Helper()
	return api.ExtensionSendOptions{ID: id, Message: encodeLine(t, msg)}
}

func TestEncodeOutboundEndsInOneNewline(t *testing.T) {
	line, err := encodeOutbound(encodeLine(t, extension.FireEvent.New(extension.EventData{Handler: "go", Text: "a\nb"})))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(line, []byte("\n")) != 1 || !bytes.HasSuffix(line, []byte("\n")) {
		t.Errorf("not one line: %q", line)
	}
}

func TestEncodeOutboundRefusals(t *testing.T) {
	for _, raw := range []string{
		"", "not json",
		fmt.Sprintf(`{"v":%d,"kind":"toast","data":{"title":"t"}}`, extension.ProtocolVersion),
		fmt.Sprintf(`{"v":%d,"kind":"openExternal","data":{"url":"https://a.b"}}`, extension.ProtocolVersion),
	} {
		if _, err := encodeOutbound(json.RawMessage(raw)); err == nil {
			t.Errorf("%q was accepted", raw)
		}
	}
}
