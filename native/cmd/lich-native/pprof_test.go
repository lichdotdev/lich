package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPprofListenerRefusesAnAddressOffTheMachine(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:6060", ":6060", "192.168.0.10:6060", "example.com:6060", "6060"} {
		if ln, err := pprofListener(addr); err == nil {
			ln.Close()
			t.Errorf("%q was accepted", addr)
		}
	}
}

func TestPprofServesTheHeapOnLoopback(t *testing.T) {
	ln, err := pprofListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go servePprof(ln)
	resp, err := http.Get("http://" + ln.Addr().String() + "/debug/pprof/heap?debug=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "heap profile") {
		t.Fatalf("status %d, body starts %.80q", resp.StatusCode, body)
	}
}
