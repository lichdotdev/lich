package browser

import "golang.org/x/sys/windows"

// errCrossDevice is what rename answers for a file moving across volumes.
var errCrossDevice error = windows.ERROR_NOT_SAME_DEVICE
