package rpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

var (
	echo   = api.Method[pair, pair]{Name: "typed.Echo"}
	silent = api.Method[pair, api.None]{Name: "typed.Silent"}
	traced = api.Method[api.None, string]{Name: "typed.Traced"}
)

func newTypedHandler() *Handler {
	h := New()
	Handle(h, echo, func(_ context.Context, p pair) (pair, error) {
		if p.A == "fail" {
			return pair{}, errors.New("boom")
		}
		return p, nil
	})
	Handle(h, silent, func(context.Context, pair) (api.None, error) { return api.None{}, nil })
	Handle(h, traced, func(ctx context.Context, _ api.None) (string, error) {
		tag, _ := ctx.Value(ctxKey{}).(string)
		return tag, nil
	})
	return h
}

func TestHandleDecodesTheOptionsObjectByName(t *testing.T) {
	rec := call(t, newTypedHandler(), "typed.Echo", `[{"b":7,"a":"x","unknown":true}]`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"a":"x","b":7}` {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

// An empty envelope is the zero options: what a client from before the
// method took any options sends.
func TestHandleReadsAnEmptyEnvelopeAsZeroOptions(t *testing.T) {
	for _, body := range []string{`[]`, ``} {
		rec := call(t, newTypedHandler(), "typed.Echo", body)
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"a":"","b":0}` {
			t.Errorf("body %q: got %d %q", body, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleAnswersNoneAsNull(t *testing.T) {
	rec := call(t, newTypedHandler(), "typed.Silent", `[{"a":"x"}]`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandleRefusesWhatIsNotOneOptionsObject(t *testing.T) {
	for _, body := range []string{`["x", 2]`, `["x"]`, `{"a":"x"}`, `[{"a":"x"},{"a":"y"}]`} {
		if rec := call(t, newTypedHandler(), "typed.Echo", body); rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: want 400, got %d %q", body, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleReportsTheServiceError(t *testing.T) {
	rec := call(t, newTypedHandler(), "typed.Echo", `[{"a":"fail"}]`)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandleHandsOverTheRequestContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/rpc/typed.Traced", strings.NewReader(`[]`))
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, "req-7"))
	rec := httptest.NewRecorder()
	newTypedHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `"req-7"` {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestHandleRefusesASecondHandlerForOneMethod(t *testing.T) {
	h := newTypedHandler()
	defer func() {
		if recover() == nil {
			t.Error("a second Handle of typed.Echo did not panic")
		}
	}()
	Handle(h, echo, func(_ context.Context, p pair) (pair, error) { return p, nil })
}
