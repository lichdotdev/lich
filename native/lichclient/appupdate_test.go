package lichclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func restartBackend(t *testing.T, status int) (*Client, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path + " " + r.URL.Query().Get("token")
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return New(Runtime{Port: port, Token: "tok"}), &got
}

func TestRestartPostsWithTheToken(t *testing.T) {
	c, got := restartBackend(t, http.StatusNoContent)
	if err := c.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *got != "POST /restart tok" {
		t.Errorf("request = %q, want POST /restart with the token", *got)
	}
}

func TestRestartFailsOnAnythingButNoContent(t *testing.T) {
	c, _ := restartBackend(t, http.StatusServiceUnavailable)
	if err := c.Restart(context.Background()); err == nil {
		t.Error("a 503 restart returned no error")
	}
}
