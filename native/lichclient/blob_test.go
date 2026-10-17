package lichclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// blobBackend answers /blob the way project.ServeBlob does: the bytes, or a
// status with the reason worded for the page.
func blobBackend(t *testing.T, status int, body string) (*Client, *url.Values) {
	t.Helper()
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blob" || r.URL.Query().Get("token") != "tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		got = r.URL.Query()
		if status != http.StatusOK {
			http.Error(w, body, status)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return New(Runtime{Port: port, Token: "tok"}), &got
}

func TestBlobSendsTheSideAndReturnsItsBytes(t *testing.T) {
	c, got := blobBackend(t, http.StatusOK, "\x89PNG")
	data, err := c.Blob(context.Background(), "/repo", "a b/shot.png", "HEAD")
	if err != nil || string(data) != "\x89PNG" {
		t.Fatalf("got %q, %v", data, err)
	}
	if got.Get("path") != "/repo" || got.Get("rel") != "a b/shot.png" || got.Get("ref") != "HEAD" {
		t.Errorf("query %v", *got)
	}
}

func TestBlobMissingSideIsErrNoBlob(t *testing.T) {
	c, _ := blobBackend(t, http.StatusNotFound, "not found")
	if _, err := c.Blob(context.Background(), "/repo", "new.png", "HEAD"); !errors.Is(err, ErrNoBlob) {
		t.Fatalf("err = %v, want ErrNoBlob", err)
	}
}

func TestBlobRefusalCarriesTheBackendSentence(t *testing.T) {
	c, _ := blobBackend(t, http.StatusRequestEntityTooLarge, "62.4 MB, above the 20 MB preview limit")
	_, err := c.Blob(context.Background(), "/repo", "big.png", "")
	if got := ErrorText(err); got != "62.4 MB, above the 20 MB preview limit" {
		t.Fatalf("ErrorText = %q", got)
	}
}
