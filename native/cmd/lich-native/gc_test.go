package main

import (
	"runtime/debug"
	"testing"
)

func TestTuneGC(t *testing.T) {
	t.Cleanup(func() { debug.SetGCPercent(debug.SetGCPercent(100)) })
	tests := []struct {
		name string
		gogc string
		want int
	}{
		{"unset takes the window's own target", "", gcPercent},
		{"GOGC wins", "25", 77},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			debug.SetGCPercent(77)
			tuneGC(func(k string) string {
				if k == "GOGC" {
					return tt.gogc
				}
				return ""
			})
			if got := debug.SetGCPercent(77); got != tt.want {
				t.Errorf("GC percent = %d, want %d", got, tt.want)
			}
		})
	}
}
