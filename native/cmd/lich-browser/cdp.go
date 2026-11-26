package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lichdotdev/lich/browserwire"
)

// The WebView2 engine drives its pages over the DevTools protocol where
// WebView2 has no API of its own: eval as the console runs it, input the page
// sees as trusted, the requests Settle waits on, the file chooser. This is
// that protocol's side of the wire, kept apart from WebView2 so it is tested
// everywhere.

// cdpError is a failed protocol call's answer as an error.
func cdpError(answer string) error {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(answer), &e) == nil && e.Message != "" {
		return errors.New(e.Message)
	}
	return fmt.Errorf("devtools protocol: %s", answer)
}

// evaluate is Runtime.evaluate's params for a script run as the DevTools
// console runs it: REPL mode has its value, top-level await and a let or
// const declared again. Its class declarations are made redeclarable as
// WebKit's are, which REPL mode allows only across scripts.
func evaluate(script string) map[string]any {
	return map[string]any{"expression": redeclarable(script), "replMode": true, "awaitPromise": true, "returnByValue": true}
}

// evalAnswer is Runtime.evaluate's answer as the wire's: the value as JSON,
// empty for undefined, or the throw as the page words it.
func evalAnswer(raw json.RawMessage) (string, error) {
	var r struct {
		Result struct {
			Type         string          `json:"type"`
			Value        json.RawMessage `json:"value"`
			Unserialized string          `json:"unserializableValue"`
		} `json:"result"`
		Exception *struct {
			Text      string `json:"text"`
			Exception struct {
				Description string          `json:"description"`
				Value       json.RawMessage `json:"value"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	if e := r.Exception; e != nil {
		if d := e.Exception.Description; d != "" {
			first, _, _ := strings.Cut(d, "\n")
			return "", errors.New(first)
		}
		if len(e.Exception.Value) > 0 {
			return "", errors.New("Uncaught " + string(e.Exception.Value))
		}
		return "", errors.New(e.Text)
	}
	switch {
	case r.Result.Type == "undefined":
		return "", nil
	case r.Result.Unserialized != "":
		// NaN, Infinity, -0 and BigInt have no JSON; JSON.stringify
		// writes null.
		return "null", nil
	case len(r.Result.Value) == 0:
		return "null", nil
	}
	return compactJSON(r.Result.Value)
}

func compactJSON(raw json.RawMessage) (string, error) {
	var b strings.Builder
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// cdpModifiers is the protocol's modifier bits for the wire's.
func cdpModifiers(m browserwire.Mods) int {
	var bits int
	for _, b := range []struct {
		mod browserwire.Mods
		bit int
	}{{browserwire.ModAlt, 1}, {browserwire.ModCtrl, 2}, {browserwire.ModMeta, 4}, {browserwire.ModShift, 8}} {
		if m&b.mod != 0 {
			bits |= b.bit
		}
	}
	return bits
}

var (
	cdpMouseTypes = map[string]string{
		browserwire.MouseMove: "mouseMoved", browserwire.MouseDown: "mousePressed",
		browserwire.MouseUp: "mouseReleased", browserwire.MouseWheel: "mouseWheel",
	}
	// cdpButtons are the protocol's names for the DOM's buttons, and the
	// bit each sets in MouseEvent.buttons.
	cdpButtons   = []string{"left", "middle", "right", "back", "forward"}
	cdpButtonBit = []int{1, 4, 2, 8, 16}
)

// mouseEvent is Input.dispatchMouseEvent's params for o, held the DOM
// buttons held before it, and the buttons held after it.
func mouseEvent(o browserwire.MouseParams, held int) (map[string]any, int, error) {
	kind, ok := cdpMouseTypes[o.Kind]
	if !ok {
		return nil, held, fmt.Errorf("no mouse event %q", o.Kind)
	}
	e := map[string]any{"type": kind, "x": o.X, "y": o.Y, "modifiers": cdpModifiers(o.Mods)}
	switch o.Kind {
	case browserwire.MouseDown, browserwire.MouseUp:
		b := o.Button
		if b < 0 || b >= len(cdpButtons) {
			b = 0
		}
		if o.Kind == browserwire.MouseDown {
			held |= cdpButtonBit[b]
		} else {
			held &^= cdpButtonBit[b]
		}
		e["button"], e["clickCount"] = cdpButtons[b], max(o.Clicks, 1)
	case browserwire.MouseWheel:
		e["deltaX"], e["deltaY"] = o.DX, o.DY
	}
	e["buttons"] = held
	return e, held, nil
}

// virtualKeys are Windows' virtual-key codes for the DOM's key names that
// are no character, which Chromium acts on (a Backspace with none deletes
// nothing).
var virtualKeys = map[string]int{
	"Enter": 0x0d, "Tab": 0x09, "Backspace": 0x08, "Escape": 0x1b, "Delete": 0x2e, "Insert": 0x2d,
	"ArrowLeft": 0x25, "ArrowUp": 0x26, "ArrowRight": 0x27, "ArrowDown": 0x28,
	"Home": 0x24, "End": 0x23, "PageUp": 0x21, "PageDown": 0x22,
	"Shift": 0x10, "Control": 0x11, "Alt": 0x12, "Meta": 0x5b, "CapsLock": 0x14, "ContextMenu": 0x5d,
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73, "F5": 0x74, "F6": 0x75,
	"F7": 0x76, "F8": 0x77, "F9": 0x78, "F10": 0x79, "F11": 0x7a, "F12": 0x7b,
}

// keyText is what a named key types, as Chromium's keyboard does.
var keyText = map[string]string{"Enter": "\r"}

// keyEvent is Input.dispatchKeyEvent's params for o.
func keyEvent(o browserwire.KeyParams) (map[string]any, error) {
	vk, named := virtualKeys[o.Key]
	text := keyText[o.Key]
	if !named {
		r, size := utf8.DecodeRuneInString(o.Key)
		if r == utf8.RuneError || size != len(o.Key) {
			return nil, fmt.Errorf("no key %q", o.Key)
		}
		vk, text = charVirtualKey(r), o.Key
	}
	if o.Mods&(browserwire.ModCtrl|browserwire.ModAlt|browserwire.ModMeta) != 0 {
		text = ""
	}
	e := map[string]any{"key": o.Key, "code": o.Code, "windowsVirtualKeyCode": vk, "modifiers": cdpModifiers(o.Mods)}
	switch o.Kind {
	case browserwire.KeyDown:
		e["type"] = "rawKeyDown"
		if text != "" {
			e["type"], e["text"], e["unmodifiedText"] = "keyDown", text, text
		}
	case browserwire.KeyUp:
		e["type"] = "keyUp"
	default:
		return nil, fmt.Errorf("no key event %q", o.Kind)
	}
	return e, nil
}

// charVirtualKey is the virtual key of a letter, digit or space, zero for
// another character, which its text alone types.
func charVirtualKey(r rune) int {
	switch {
	case r >= 'a' && r <= 'z':
		return int(r - 'a' + 'A')
	case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ':
		return int(r)
	}
	return 0
}

// cdpRequestTypes are the protocol's resource types the wire names.
var cdpRequestTypes = map[string]string{
	"Document": browserwire.RequestDocument, "Stylesheet": browserwire.RequestStylesheet,
	"Image": browserwire.RequestImage, "Media": browserwire.RequestMedia, "Font": browserwire.RequestFont,
	"Script": browserwire.RequestScript, "XHR": browserwire.RequestXHR, "Fetch": browserwire.RequestFetch,
}

func requestKind(cdpType string) string {
	if k, ok := cdpRequestTypes[cdpType]; ok {
		return k
	}
	return browserwire.RequestOther
}

// navigationErrors are WebView2's COREWEBVIEW2_WEB_ERROR_STATUS, in its
// order, worded for the agent reading why a load failed.
var navigationErrors = [...]string{
	"the load failed for a reason WebView2 does not know",
	"the server's certificate names another host",
	"the server's certificate has expired",
	"the client certificate has errors",
	"the server's certificate was revoked",
	"the server's certificate is not valid",
	"the server is unreachable",
	"the server timed out",
	"the server's response is no valid HTTP",
	"the connection was aborted",
	"the connection was reset",
	"the network is disconnected",
	"no connection could be made",
	"the host name did not resolve",
	"the load was canceled",
	"a redirect failed",
	"an unexpected error",
	"the server asks for credentials",
	"the proxy asks for credentials",
}

func navigationError(status int) string {
	if status < 0 || status >= len(navigationErrors) {
		return navigationErrors[0]
	}
	return navigationErrors[status]
}

// pageMessage is a message the page script posted, [handler, text].
func pageMessage(raw string) (handler, text string, ok bool) {
	var m []string
	if json.Unmarshal([]byte(raw), &m) != nil || len(m) != 2 {
		return "", "", false
	}
	return m[0], m[1], true
}
