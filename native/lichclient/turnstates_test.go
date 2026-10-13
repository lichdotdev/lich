package lichclient

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestTurnStatesDecodesTheSnapshot(t *testing.T) {
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		if method != "terminal.TurnStates" || len(args) != 0 {
			t.Errorf("got %s(%s), want terminal.TurnStates()", method, rawArgs(args))
		}
		return json.RawMessage(`[
			{"id":"s1","state":"busy","tool":"Bash","detail":"go test","since":1000,"usage":{"id":"s1","percent":42}},
			{"id":"s2","state":"waiting","reason":"permission","since":2000},
			{"id":"s3","usage":{"id":"s3","percent":7}}]`), http.StatusOK
	})
	got, err := c.TurnStates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %+v, want 3 sessions", got)
	}
	if s := got[0]; s.ID != "s1" || s.State != "busy" || s.Tool != "Bash" || s.Detail != "go test" || s.Since != 1000 || len(s.Usage) == 0 {
		t.Errorf("s1 = %+v", s)
	}
	if s := got[1]; s.State != "waiting" || s.Reason != "permission" || s.Usage != nil {
		t.Errorf("s2 = %+v", s)
	}
	if s := got[2]; s.State != "" || len(s.Usage) == 0 {
		t.Errorf("s3 = %+v", s)
	}
}
