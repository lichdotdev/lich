//go:build unix

package markdown

import (
	"syscall"
	"time"
)

// spent is the CPU time this process has used: a loaded runner gives the
// CPU to other packages' tests, which a wall clock would count as ours.
func spent() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		panic(err)
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
