package browser

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// answering is an evalFunc whose page answers every script with answer.
func answering(answer string) evalFunc {
	return func(_ context.Context, _ string, out any) error {
		return json.Unmarshal([]byte(answer), out)
	}
}

func TestResolveTargetRefusesWhatAClickWouldMiss(t *testing.T) {
	for _, tc := range []struct {
		answer, wantErr string
	}{
		{`null`, "no element matches"},
		{`{"disabled":true}`, "disabled"},
		{`{"x":1,"y":2,"covered":"div.modal"}`, "covered by <div.modal>"},
		{`{"x":1,"y":2,"resized":true,"w":800,"h":600}`, "was resized"},
		{`{"x":900,"y":2,"outside":true,"w":800,"h":600}`, "outside the 800x600 viewport"},
		{`{"x":1,"y":2}`, ""},
	} {
		pt, err := resolveTarget(t.Context(), answering(tc.answer), Target{Selector: "#a"})
		switch {
		case tc.wantErr == "" && (err != nil || pt != point{X: 1, Y: 2}):
			t.Errorf("%s: %v, %v: want 1,2", tc.answer, pt, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("%s: %v, want %q", tc.answer, err, tc.wantErr)
		}
	}
}

func TestSettleQuietWaitsOutTheFetchThenTheDocument(t *testing.T) {
	fetches, quiets := 2, 0
	inFlight := func(browserTime) bool { fetches--; return fetches > 0 }
	quiet := func(context.Context, time.Duration) (bool, error) { quiets++; return quiets > 1, nil }
	if err := settleQuiet(t.Context(), inFlight, quiet, browserTime{}); err != nil {
		t.Fatal(err)
	}
	if quiets != 2 {
		t.Fatalf("%d quiet reads, want the document read until it went quiet", quiets)
	}
}
