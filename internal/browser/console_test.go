package browser

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
	"github.com/lichdotdev/lich/api"
)

func TestReadNamesTheConsoleErrorsSinceTheLastAnswerOnce(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onConsoleError("Uncaught TypeError: x is undefined")
	p.onConsoleError("boom")
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Uncaught TypeError: x is undefined", "boom"}
	if !reflect.DeepEqual(info.ConsoleErrors, want) || info.ConsoleErrorsMore != 0 {
		t.Fatalf("read reported %q (+%d), want %q", info.ConsoleErrors, info.ConsoleErrorsMore, want)
	}
	again, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.ConsoleErrors) != 0 {
		t.Fatalf("a second read repeated %q", again.ConsoleErrors)
	}
}

func TestClickAndPressAnswerWithTheConsoleErrorsTheyCaused(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.onClick = func() { p.onConsoleError("from click") }
	clicked, err := s.ClickAt(t.Context(), api.BrowserClickOptions{ID: "s", BrowserTarget: api.BrowserTarget{Selector: "#b"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(clicked.ConsoleErrors, []string{"from click"}) {
		t.Fatalf("click reported %q", clicked.ConsoleErrors)
	}
	p.onConsoleError("before press")
	pressed, err := s.PressKey(t.Context(), api.BrowserPressOptions{ID: "s", Key: "Enter"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pressed.ConsoleErrors, []string{"before press"}) {
		t.Fatalf("press reported %q", pressed.ConsoleErrors)
	}
}

func TestConsoleErrorsPastTheCapAreCountedNotKept(t *testing.T) {
	s, launch, _ := viewService(t)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	for i := range consoleErrorMax + 3 {
		p.onConsoleError(fmt.Sprintf("e%d", i))
	}
	info, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.ConsoleErrors) != consoleErrorMax || info.ConsoleErrors[0] != "e0" || info.ConsoleErrorsMore != 3 {
		t.Fatalf("read reported %q (+%d), want the first %d and 3 more", info.ConsoleErrors, info.ConsoleErrorsMore, consoleErrorMax)
	}
	again, err := s.Read(t.Context(), api.BrowserInfoOptions{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if again.ConsoleErrorsMore != 0 {
		t.Fatalf("the count of more errors was reported again: %d", again.ConsoleErrorsMore)
	}
}

func TestConsoleTextIsOneLineCutToItsLimit(t *testing.T) {
	long := strings.Repeat("é", consoleTextMax+10)
	got := consoleLine(long)
	if n := len([]rune(got)); n != consoleTextMax+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("cut to %d runes (%q...), want %d and an ellipsis", n, got[:8], consoleTextMax+1)
	}
	if got := consoleLine("  a\n\tb  c "); got != "a b c" {
		t.Fatalf("got %q, want the whitespace folded", got)
	}
}

func TestConsoleAPIErrorJoinsItsArguments(t *testing.T) {
	ev := &runtime.EventConsoleAPICalled{Type: runtime.APITypeError, Args: []*runtime.RemoteObject{
		{Type: runtime.TypeString, Value: []byte(`"failed: save"`)},
		{Type: runtime.TypeNumber, Value: []byte(`42`)},
		{Type: runtime.TypeObject, Description: "Error: nope\n    at f (app.js:1:2)"},
		{Type: runtime.TypeNumber, UnserializableValue: "NaN"},
		{Type: runtime.TypeUndefined},
	}}
	if got, want := consoleCallText(ev), "failed: save 42 Error: nope NaN undefined"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExceptionTextNamesTheErrorAndWhereItWasThrown(t *testing.T) {
	d := &runtime.ExceptionDetails{
		Text: "Uncaught", URL: "http://x/app.js", LineNumber: 11,
		Exception: &runtime.RemoteObject{Description: "TypeError: a is null\n    at g (app.js:12:3)"},
	}
	if got, want := exceptionText(d), "Uncaught TypeError: a is null (http://x/app.js:12)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	thrown := &runtime.ExceptionDetails{Text: "Uncaught", Exception: &runtime.RemoteObject{Value: []byte(`"plain"`)}}
	if got := exceptionText(thrown); got != "Uncaught plain" {
		t.Fatalf("a thrown string read %q", got)
	}
	if got := exceptionText(&runtime.ExceptionDetails{Text: "Uncaught SyntaxError"}); got != "Uncaught SyntaxError" {
		t.Fatalf("an exception without its object read %q", got)
	}
}

func TestLogEntryNamesTheResourceThatFailed(t *testing.T) {
	e := &log.Entry{Level: log.LevelError, Text: "Failed to load resource: 404 (Not Found)", URL: "http://x/a.png"}
	if got, want := logEntryText(e), "Failed to load resource: 404 (Not Found) http://x/a.png"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := logEntryText(&log.Entry{Text: "CSP"}); got != "CSP" {
		t.Fatalf("got %q", got)
	}
}
