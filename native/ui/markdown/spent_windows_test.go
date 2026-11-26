package markdown

import "time"

var processStart = time.Now()

// spent is the wall clock here: a Windows process's CPU times tick at
// 15.6 ms, as coarse as the budget, and the CI job runs one package's tests
// at a time.
func spent() time.Duration { return time.Since(processStart) }
