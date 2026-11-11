package extension

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func filledOf(t reflect.Type) any {
	v := reflect.New(t).Elem()
	fill(v, t.Name())
	return v.Interface()
}

func everyMethod() []map[string]any {
	var all []map[string]any
	for _, name := range MethodNames() {
		decl := methods[name]
		all = append(all, map[string]any{
			"method": name, "permission": decl.permission,
			"params": filledOf(decl.params), "result": filledOf(decl.result),
		})
	}
	return all
}

func everyEvent() []map[string]any {
	var all []map[string]any
	for _, name := range sortedKeys(events) {
		decl := events[name]
		all = append(all, map[string]any{"name": name, "permission": decl.permission, "data": filledOf(decl.data)})
	}
	return all
}

func TestMethodsMatchGolden(t *testing.T) { checkGolden(t, "methods.golden.json", everyMethod()) }
func TestEventsMatchGolden(t *testing.T)  { checkGolden(t, "events.golden.json", everyEvent()) }

func TestEveryPermissionIsKnown(t *testing.T) {
	for name, decl := range methods {
		if decl.permission != "" && !permissionKnown(decl.permission) {
			t.Errorf("method %s needs %q, which Permissions does not list", name, decl.permission)
		}
	}
	for name, decl := range events {
		if !permissionKnown(decl.permission) {
			t.Errorf("event %s needs %q, which Permissions does not list", name, decl.permission)
		}
	}
}

func permissionKnown(p Permission) bool {
	for _, known := range Permissions {
		if known == p {
			return true
		}
	}
	return false
}

func roundTrip(t *testing.T, msg Message) Message {
	t.Helper()
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var got Message
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestACallAndItsResultRoundTripTyped(t *testing.T) {
	want := SendParams{Session: "api-3", Prompt: "run the tests", WaitSeconds: 5}
	call := roundTrip(t, SendToSession.Call("7", want)).Data.(CallData)
	if call.ID != "7" || call.Method != "sessions.send" {
		t.Fatalf("call came back as %+v", call)
	}
	params, err := SendToSession.Params(call)
	if err != nil || params != want {
		t.Fatalf("params %+v, %v; want %+v", params, err, want)
	}

	answer := SendResult{Ticket: "t1", Target: "api-3", Status: "answered", Answer: "green"}
	res := roundTrip(t, SendToSession.Answer("7", answer)).Data.(ResultData)
	got, err := SendToSession.Result(res)
	if err != nil || got != answer || res.ID != "7" {
		t.Fatalf("result %+v, %v under %q; want %+v", got, err, res.ID, answer)
	}
}

func TestAnErrorResultIsAGoError(t *testing.T) {
	_, err := ListSessions.Result(ResultData{ID: "1", Error: `needs the permission "sessions:read"`})
	if err == nil || !strings.Contains(err.Error(), "sessions.list") || !strings.Contains(err.Error(), "sessions:read") {
		t.Errorf("got %v, want an error naming the method and the reason", err)
	}
}

func TestParamsThatDoNotDecodeAreRefused(t *testing.T) {
	_, err := SendToSession.Params(CallData{Method: "sessions.send", Params: json.RawMessage(`{"session":7}`)})
	if err == nil || !strings.Contains(err.Error(), "sessions.send") {
		t.Errorf("got %v, want an error naming the method", err)
	}
}

func TestAnEventDecodesOnlyAsItself(t *testing.T) {
	want := SessionStatusData{ID: "s1", State: "busy", Tool: "Bash"}
	ev := roundTrip(t, SessionStatus.New(want)).Data.(LichEventData)
	got, err := SessionStatus.Decode(ev)
	if err != nil || got != want {
		t.Fatalf("decoded %+v, %v; want %+v", got, err, want)
	}
	if _, err := SessionStatus.Decode(LichEventData{Name: "other"}); err == nil {
		t.Error("an event of another name decoded as session.status")
	}
}
