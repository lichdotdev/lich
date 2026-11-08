package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The backend reads the leading zero byte as a person typing
// (internal/terminal/transport.go), so only SendTyped may write one.
func TestSendTypedMarksTheFrameAndSendDoesNot(t *testing.T) {
	frames := make(chan []byte, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			frames <- data
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, _, err := New(Runtime{Port: port, Token: "tok"}).Terminal(ctx, func(string, []byte) {}, func(string, []byte) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(ctx, "s", []byte("r")); err != nil {
		t.Fatal(err)
	}
	if err := stream.SendTyped(ctx, "s", []byte("k")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\x01sr", "\x00\x01sk"} {
		select {
		case got := <-frames:
			if string(got) != want {
				t.Fatalf("frame %q, want %q", got, want)
			}
		case <-ctx.Done():
			t.Fatalf("no frame %q", want)
		}
	}
}
