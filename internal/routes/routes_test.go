package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/spawn"
	"github.com/lichdotdev/lich/internal/themes"
)

func TestHandleThemesCoversEveryThemesMethod(t *testing.T) {
	d := rpc.New()
	Themes(d, themes.NewInDir(t.TempDir()))
	assertHandled(t, d, "themes.")
}

// The services below are never reached: a handled call stops at its decoding.
func TestHandleRelayCoversEveryRelayMethod(t *testing.T) {
	d := rpc.New()
	Relay(d, nil)
	assertHandled(t, d, "relay.")
}

func TestHandleSystemCoversEverySystemMethod(t *testing.T) {
	d := rpc.New()
	System(d, nil)
	assertHandled(t, d, "system.")
}

func TestHandleTrayCoversEveryTrayMethod(t *testing.T) {
	d := rpc.New()
	Tray(d, nil)
	assertHandled(t, d, "tray.")
}

func TestHandleStoreCoversEveryStoreMethod(t *testing.T) {
	d := rpc.New()
	Store(d, nil)
	assertHandled(t, d, "store.")
}

func TestHandleAppServicesCoverEveryMethod(t *testing.T) {
	for prefix, route := range map[string]func(*rpc.Handler){
		"drop.":        func(d *rpc.Handler) { Drop(d, nil) },
		"patchnotes.":  func(d *rpc.Handler) { PatchNotes(d, nil) },
		"appupdate.":   func(d *rpc.Handler) { AppUpdate(d, nil) },
		"agentplugin.": func(d *rpc.Handler) { AgentPlugin(d, nil) },
		"providers.":   func(d *rpc.Handler) { Providers(d, nil) },
	} {
		d := rpc.New()
		route(d)
		assertHandled(t, d, prefix)
	}
}

// assertHandled fails on a method under prefix that api declares and no route
// of d answers, which a client would only meet as a 404. A body that is no
// options object stops a handled call at its decoding (400), before the
// service runs; an unhandled one is unknown (404).
func assertHandled(t *testing.T, d *rpc.Handler, prefix string) {
	t.Helper()
	seen := 0
	for name := range api.Methods() {
		if !strings.HasPrefix(name, prefix) {
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
		t.Fatalf("api declares no %s method: the scan no longer sees them", prefix)
	}
}

func TestHandleSpawnCoversEverySpawnMethod(t *testing.T) {
	d := rpc.New()
	Spawn(d, spawn.New(nil, nil, nil, nil))
	assertHandled(t, d, "spawn.")
}
