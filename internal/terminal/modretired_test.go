package terminal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A lich-plugin older than its mod's removal still polls and posts /mod/*. Each
// answers 404 behind every mount main.go makes, and 404 is what each of those
// mods reads as a lich without the endpoint and stops on (lich-plugin 0.21.1:
// mod-control.js stops polling, mod-usage.js and mod-status.js stop sending,
// worker-answer.js drops its report), where 401 or 403 would have it retry.
func TestARetiredModEndpointIsNotFound(t *testing.T) {
	tr, _, _ := scopedTransport(t)
	for _, path := range []string{"/mod/commands", "/mod/acks", "/mod/usage", "/mod/answer", "/mod/status"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			rec := httptest.NewRecorder()
			tr.mux.ServeHTTP(rec, httptest.NewRequest(method, path+"?token="+tr.token+"&session_id=s1", strings.NewReader("{}")))
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s = %d, want 404", method, path, rec.Code)
			}
		}
	}
}
