package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
)

// pprofEnv names the address lich-native serves its runtime profiles on, for
// capturing the heap or CPU of a real window; unset, nothing listens.
const pprofEnv = "LICH_NATIVE_PPROF"

// pprofListener listens on addr, which must be a loopback address: a heap
// profile carries whatever the window holds in memory.
func pprofListener(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("%s=%q: %w", pprofEnv, addr, err)
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("%s=%q: want a loopback address, such as 127.0.0.1:6060", pprofEnv, addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pprofEnv, err)
	}
	return ln, nil
}

// servePprof answers net/http/pprof's endpoints on ln until it closes.
func servePprof(ln net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	log.Printf("pprof: serving http://%s/debug/pprof/", ln.Addr())
	log.Printf("pprof: %v", http.Serve(ln, mux))
}
