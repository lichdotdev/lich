package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/themes"
)

// TestHandleThemesCoversEveryThemesMethod fails on a themes method api
// declares and no route answers, which a client would only meet as a 404. A
// body that is no options object stops a handled call at its decoding (400),
// before the service runs; an unhandled one is unknown (404).
func TestHandleThemesCoversEveryThemesMethod(t *testing.T) {
	d := rpc.New()
	handleThemes(d, themes.NewInDir(t.TempDir()))
	seen := 0
	for name := range api.Methods() {
		if !strings.HasPrefix(name, "themes.") {
			continue
		}
		seen++
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rpc/"+name, strings.NewReader(`["not options"]`)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400 from its route, got %d %q", name, rec.Code, rec.Body.String())
		}
	}
	if seen == 0 {
		t.Fatal("api declares no themes method: the scan no longer sees them")
	}
}
