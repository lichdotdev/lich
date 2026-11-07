// Command framestat summarizes a LICH_NATIVE_FRAME_LOG file: frames per second
// per cause, and each phase's p50 and p95.
//
//	framestat [-last 30s] frames.jsonl
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lichdotdev/lich/native/internal/framelog"
)

func main() {
	last := flag.Duration("last", 0, "only the last `window` of the log (default all)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: framestat [-last 30s] frames.jsonl")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "framestat:", err)
		os.Exit(1)
	}
	defer f.Close()
	s, err := framelog.Read(f, *last)
	if err != nil {
		fmt.Fprintln(os.Stderr, "framestat:", err)
		os.Exit(1)
	}
	s.Write(os.Stdout)
}
