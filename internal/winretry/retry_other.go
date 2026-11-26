//go:build !windows

// Package winretry runs a file operation that Windows can refuse for a moment
// after the process holding the file has exited.
package winretry

// Do runs op: outside Windows nothing holds a file once its process exited.
func Do(op func() error) error { return op() }
