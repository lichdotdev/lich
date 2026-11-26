package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/browserwire"
)

func TestEvalAnswerAsTheWireDoes(t *testing.T) {
	for _, tc := range []struct{ name, answer, want, err string }{
		{"a string", `{"result":{"type":"string","value":"Hi"}}`, `"Hi"`, ""},
		{"an object, compact", `{"result":{"type":"object","value":{"a": [1, 2]}}}`, `{"a":[1,2]}`, ""},
		{"undefined", `{"result":{"type":"undefined"}}`, "", ""},
		{"NaN", `{"result":{"type":"number","unserializableValue":"NaN"}}`, "null", ""},
		{"a throw, its first line", `{"result":{"type":"object"},"exceptionDetails":{"text":"Uncaught","exception":{"description":"TypeError: nope\n    at <anonymous>:1:7"}}}`, "", "TypeError: nope"},
		{"a thrown string", `{"result":{},"exceptionDetails":{"text":"Uncaught","exception":{"value":"x"}}}`, "", `Uncaught "x"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evalAnswer(json.RawMessage(tc.answer))
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("error %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("evaluated = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestEvalNextAwaitsAPromiseAndReadsAnObject(t *testing.T) {
	for _, tc := range []struct{ name, answer, method string }{
		{"a promise", `{"result":{"type":"object","subtype":"promise","objectId":"7"}}`, "Runtime.awaitPromise"},
		{"an object", `{"result":{"type":"object","objectId":"7"}}`, "Runtime.callFunctionOn"},
		{"a number", `{"result":{"type":"number","value":2}}`, ""},
		{"a throw", `{"result":{"type":"object","objectId":"7"},"exceptionDetails":{"text":"Uncaught"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method, params := evalNext(json.RawMessage(tc.answer))
			if method != tc.method || (method != "" && params["returnByValue"] != true) {
				t.Fatalf("evalNext = %q %v, want %q by value", method, params, tc.method)
			}
		})
	}
}

func TestEvaluateMakesClassesRedeclarable(t *testing.T) {
	if got := evaluate("class K {}; K")["expression"]; got != "var K = class K {}; K" {
		t.Fatalf("expression %q", got)
	}
}

func TestMouseEventTracksTheButtonsHeld(t *testing.T) {
	down, held, err := mouseEvent(browserwire.MouseParams{Kind: browserwire.MouseDown, X: 5, Y: 6, Button: 2, Mods: browserwire.ModShift}, 0)
	if err != nil || held != 2 || down["button"] != "right" || down["buttons"] != 2 || down["clickCount"] != 1 || down["modifiers"] != 8 {
		t.Fatalf("down %v, held %d, %v", down, held, err)
	}
	up, held, _ := mouseEvent(browserwire.MouseParams{Kind: browserwire.MouseUp, Button: 2, Clicks: 2}, held)
	if held != 0 || up["buttons"] != 0 || up["clickCount"] != 2 {
		t.Fatalf("up %v, held %d", up, held)
	}
	wheel, _, _ := mouseEvent(browserwire.MouseParams{Kind: browserwire.MouseWheel, DY: 200}, 0)
	if wheel["type"] != "mouseWheel" || wheel["deltaY"] != 200.0 {
		t.Fatalf("wheel %v", wheel)
	}
	if _, _, err := mouseEvent(browserwire.MouseParams{Kind: "drag"}, 0); err == nil {
		t.Fatal("a kind the wire has not took")
	}
}

func TestKeyEventTypesWhatTheKeyTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		o    browserwire.KeyParams
		want map[string]any
	}{
		{"a letter", browserwire.KeyParams{Kind: browserwire.KeyDown, Key: "a", Code: "KeyA"},
			map[string]any{"type": "keyDown", "key": "a", "code": "KeyA", "windowsVirtualKeyCode": 0x41, "modifiers": 0, "text": "a", "unmodifiedText": "a"}},
		{"Enter", browserwire.KeyParams{Kind: browserwire.KeyDown, Key: "Enter"},
			map[string]any{"type": "keyDown", "key": "Enter", "code": "", "windowsVirtualKeyCode": 0x0d, "modifiers": 0, "text": "\r", "unmodifiedText": "\r"}},
		{"Backspace", browserwire.KeyParams{Kind: browserwire.KeyDown, Key: "Backspace"},
			map[string]any{"type": "rawKeyDown", "key": "Backspace", "code": "", "windowsVirtualKeyCode": 0x08, "modifiers": 0}},
		{"a shortcut types nothing", browserwire.KeyParams{Kind: browserwire.KeyDown, Key: "a", Mods: browserwire.ModCtrl},
			map[string]any{"type": "rawKeyDown", "key": "a", "code": "", "windowsVirtualKeyCode": 0x41, "modifiers": 2}},
		{"a release", browserwire.KeyParams{Kind: browserwire.KeyUp, Key: "Tab"},
			map[string]any{"type": "keyUp", "key": "Tab", "code": "", "windowsVirtualKeyCode": 0x09, "modifiers": 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := keyEvent(tc.o)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("keyEvent = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	if _, err := keyEvent(browserwire.KeyParams{Kind: browserwire.KeyDown, Key: "Hyper"}); err == nil {
		t.Fatal("a key with no name the wire knows took")
	}
}

func TestRequestKindAndNavigationErrorCoverWhatTheyDoNotKnow(t *testing.T) {
	if requestKind("XHR") != browserwire.RequestXHR || requestKind("WebSocket") != browserwire.RequestOther {
		t.Fatal("request kinds")
	}
	if navigationError(13) != "the host name did not resolve" || navigationError(99) != navigationErrors[0] {
		t.Fatal("navigation errors")
	}
}

func TestPageMessageIsAHandlerAndItsText(t *testing.T) {
	if h, text, ok := pageMessage(`["lich","boom"]`); !ok || h != "lich" || text != "boom" {
		t.Fatalf("got %q %q %v", h, text, ok)
	}
	if _, _, ok := pageMessage(`"boom"`); ok {
		t.Fatal("a message of no handler read")
	}
	if cdpError(`{"code":-32000,"message":"No node"}`).Error() != "No node" {
		t.Fatal("cdpError")
	}
}
