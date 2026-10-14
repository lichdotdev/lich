//go:build !windows

package profilestore

import (
	"syscall"
	"testing"
)

func TestCrossDeviceKnowsFilesystems(t *testing.T) {
	if !crossDevice(syscall.EXDEV) {
		t.Error("EXDEV was not read as crossing filesystems, so the copy never runs")
	}
	if crossDevice(syscall.ENOENT) {
		t.Error("a missing file was read as crossing filesystems")
	}
}
