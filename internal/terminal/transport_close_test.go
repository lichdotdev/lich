package terminal

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
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

// A fork/exec anywhere in the process holds a copy of the listener's fd until
// its exec; lich forks constantly (PTYs, git, gh), so close must free the port
// even then. Without the fix about 4% of rebinds fail here.
func TestCloseFreesThePortWhileTheProcessForks(t *testing.T) {
	const rebinds = 200
	stop := make(chan struct{})
	var forks sync.WaitGroup
	for range 4 {
		forks.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					_ = exec.Command(os.Args[0], "-test.run=^$").Run()
				}
			}
		})
	}
	defer func() { close(stop); forks.Wait() }()

	for i := range rebinds {
		tr := newRestartTransport(t)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ping", tr.port)) // Serve is up
		if err != nil {
			t.Fatalf("ping: %v", err)
		}
		_ = resp.Body.Close()
		if err := tr.close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", tr.port))
		if err != nil {
			t.Fatalf("rebind %d after close: %v", i, err)
		}
		_ = l.Close()
	}
}
