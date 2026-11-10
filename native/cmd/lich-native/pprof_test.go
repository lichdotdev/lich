package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/internal/framelog"
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

func TestPprofServesTheFrameCount(t *testing.T) {
	ln, err := pprofListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go servePprof(ln)
	frames.Add(3)
	resp, err := http.Get("http://" + ln.Addr().String() + "/debug/vars")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var vars struct{ Frames int64 }
	if err := json.NewDecoder(resp.Body).Decode(&vars); err != nil {
		t.Fatal(err)
	}
	if vars.Frames < 3 {
		t.Fatalf("frames = %d, want the 3 drawn", vars.Frames)
	}
}

func TestPprofServesTheFrameCauses(t *testing.T) {
	ln, err := pprofListener("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go servePprof(ln)
	framelog.Causes.Add("tested", 2)
	resp, err := http.Get("http://" + ln.Addr().String() + "/debug/vars")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var vars struct {
		FrameCauses map[string]int64 `json:"frame_causes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vars); err != nil {
		t.Fatal(err)
	}
	if vars.FrameCauses["tested"] < 2 {
		t.Fatalf("frame_causes = %v, want the 2 counted", vars.FrameCauses)
	}
}
