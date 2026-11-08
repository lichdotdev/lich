package browser

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/lichdotdev/lich/api"
)

func TestEvalNeedsABrowserAndAScript(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "1"}); !errors.Is(err, errNoBrowser) {
		t.Fatalf("eval with no browser: %v", err)
	}
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "  \n"}); !errors.Is(err, errNoScript) {
		t.Fatalf("blank script: %v", err)
	}
	if got := launch.pages[0].evals; len(got) != 0 {
		t.Fatalf("a blank script reached the page: %q", got)
	}
}

func TestEvalAnswersTheValueCutTo2000Runes(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	p.evalOut = strings.Repeat("é", 2500)
	got, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "document.title"})
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat("é", 2000) + "…"; got != want {
		t.Fatalf("answer of %d runes, want 2000 and an ellipsis", len([]rune(got)))
	}
	if len(p.evals) != 1 || p.evals[0] != "document.title" {
		t.Fatalf("page ran %q", p.evals)
	}
	p.evalOut = strings.Repeat("a", 2000)
	if got, _ := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "x"}); got != p.evalOut {
		t.Fatal("an answer of exactly 2000 runes was cut")
	}
}

func TestEvalThatThrowsKeepsTheBrowser(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("the script threw: Uncaught Error: boom")
	launch.pages[0].evalErr = boom
	if _, err := s.Eval(t.Context(), api.BrowserEvalOptions{ID: "s", Script: "throw 1"}); !errors.Is(err, boom) {
		t.Fatalf("eval error = %v", err)
	}
	if _, err := s.Info("s", ""); err != nil {
		t.Fatalf("the browser went with the script's error: %v", err)
	}
}

func TestEvalValueReadsEachKindOfResult(t *testing.T) {
	cases := []struct {
		name string
		obj  *runtime.RemoteObject
		want string
	}{
		{"nothing", nil, "undefined"},
		{"undefined", &runtime.RemoteObject{Type: runtime.TypeUndefined}, "undefined"},
		{"string unquoted", &runtime.RemoteObject{Type: runtime.TypeString, Value: json.RawMessage(`"a \"b\"\nc"`)}, "a \"b\"\nc"},
		{"number", &runtime.RemoteObject{Type: runtime.TypeNumber, Value: json.RawMessage(`42`)}, "42"},
		{"object as JSON", &runtime.RemoteObject{Type: runtime.TypeObject, Value: json.RawMessage(`{"a":[1,2]}`)}, `{"a":[1,2]}`},
		{"null", &runtime.RemoteObject{Type: runtime.TypeObject, Value: json.RawMessage(`null`)}, "null"},
		{"NaN", &runtime.RemoteObject{Type: runtime.TypeNumber, UnserializableValue: "NaN"}, "NaN"},
		{"description", &runtime.RemoteObject{Type: runtime.TypeFunction, Description: "function f() {}"}, "function f() {}"},
		{"bare type", &runtime.RemoteObject{Type: runtime.TypeSymbol}, "symbol"},
	}
	for _, c := range cases {
		if got := evalValue(c.obj); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
