package terminal

import (
	"fmt"
	"net"
	"net/http"
	"testing"
)

// A quitting lich keeps running for the services' stop grace after it closes
// the transport; a relaunch in that window must find the port free.
func TestCloseFreesThePortWhileAClientIsConnected(t *testing.T) {
	t.Parallel()
	tr := newRestartTransport(t)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", tr.port))
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	_ = resp.Body.Close() // the keep-alive connection stays open

	if err := tr.close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", tr.port))
	if err != nil {
		t.Fatalf("rebind after close: %v", err)
	}
	_ = l.Close()
}
