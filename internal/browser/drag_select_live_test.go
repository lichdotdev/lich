package browser

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

const dragText = "plain text to select here"

func TestChromeDragSelectsText(t *testing.T) {
	p, srv := livePages(t, map[string]string{"/": `<body style="margin:0"><p id="p" style="margin:0;font:20px monospace">` + dragText + `</p>`})
	ctx := liveOpen(t, p, srv.URL+"/")
	steps := []api.BrowserMouseOptions{
		{Type: "move", X: 5, Y: 10},
		{Type: "press", X: 5, Y: 10, Button: "left", Clicks: 1},
		{Type: "move", X: 60, Y: 10, Button: "left"},
		{Type: "move", X: 120, Y: 10, Button: "left"},
		{Type: "release", X: 120, Y: 10, Button: "left", Clicks: 1},
	}
	for _, m := range steps {
		if err := p.Mouse(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	got, err := p.Selection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// How many letters 115px covers is the machine's monospace font's to say.
	if got == "" || !strings.HasPrefix(dragText, got) {
		t.Fatalf("drag selected %q, want the start of %q", got, dragText)
	}
}
