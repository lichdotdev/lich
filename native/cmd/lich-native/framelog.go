package main

import (
	"os"

	"github.com/lichdotdev/lich/native/internal/framelog"
	"github.com/lichdotdev/lich/native/ui"
)

// frameLogEnv names the file lich-native writes one JSON line per drawn frame
// to, with what asked for the frame and where its time went; unset, no log is
// kept. native/cmd/framestat summarizes one.
const frameLogEnv = "LICH_NATIVE_FRAME_LOG"

// causeEcho is output that arrived within echoWindow of a write to the PTY.
const causeEcho = "echo"

// frameCauses names the functions that invalidate for the causes people ask
// about; any other function is logged as "other:" and its name.
var frameCauses = map[string]string{
	"terminals.onOutput":      "pty",
	"terminals.onReplay":      "pty",
	"model.follow":            "event",
	"model.decodeFrames":      "screencast",
	"model.applyBrowserFrame": "screencast",
}

// openFrameLog opens the log frameLogEnv names, or returns nil, the disabled
// log, when it is unset.
func openFrameLog(beat *ui.LoopBeat) (*framelog.Log, error) {
	path := os.Getenv(frameLogEnv)
	if path == "" {
		return nil, nil
	}
	return framelog.Open(path, beat.Pending, frameCauses)
}
