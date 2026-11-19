package pagescript

import (
	"strings"
	"testing"
)

func TestLocatePassesSelectorsAndRevealAsJSON(t *testing.T) {
	script, err := Locate([]string{`a[title="x"]`}, "#top")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(script, `(["a[title=\"x\"]"], "#top")`) {
		t.Fatalf("locate call ends %q, want the selectors and reveal as JSON", script[len(script)-40:])
	}
}

func TestElementAtCallsTheScriptAtThePoint(t *testing.T) {
	if got := ElementAt(12.5, 40); !strings.HasPrefix(got, elementScript) || !strings.HasSuffix(got, "(12.5, 40)") {
		t.Fatalf("element call ends %q, want the script at 12.5, 40", got[len(got)-20:])
	}
}
