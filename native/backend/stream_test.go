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

type wsFrame struct {
	kind websocket.MessageType
	data string
}

// A backend older than the window skips text frames and decodes binary ones
// as it always did, so typing has to survive both being true.
func TestSendTypedPrecedesAnOrdinaryFrameWithAText(t *testing.T) {
	frames := make(chan wsFrame, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		for {
			kind, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			frames <- wsFrame{kind, string(data)}
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
	if err := stream.SendTyped(ctx, "s", []byte{0}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []wsFrame{
		{websocket.MessageBinary, "\x01sr"},
		{websocket.MessageText, "s"},
		{websocket.MessageBinary, "\x01s\x00"},
	} {
		select {
		case got := <-frames:
			if got != want {
				t.Fatalf("frame %+v, want %+v", got, want)
			}
		case <-ctx.Done():
			t.Fatalf("no frame %+v", want)
		}
	}
}
