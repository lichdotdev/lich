package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type pair struct {
	A string `json:"a"`
	B int    `json:"b"`
}

type ctxKey struct{}

func call(t *testing.T, h *Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc/"+method, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAMethodNoRouteAnswersIsUnknown(t *testing.T) {
	for _, name := range []string{"typed.Nope", "nope.Echo", "malformed"} {
		if rec := call(t, newTypedHandler(), name, `[]`); rec.Code != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", name, rec.Code)
		}
	}
}

// Only a POST is a call, and no CORS header is answered: a page in a browser
// holding the token cannot read a reply cross-origin.
func TestOnlyPostIsACall(t *testing.T) {
	h := newTypedHandler()
	for _, method := range []string{http.MethodOptions, http.MethodGet} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/rpc/typed.Echo", nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("%s: %d %v", method, rec.Code, rec.Header())
		}
	}
}
