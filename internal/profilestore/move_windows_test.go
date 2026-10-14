package profilestore

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCrossDeviceKnowsWindowsVolumes(t *testing.T) {
	err := &os.LinkError{Op: "rename", Old: `C:\a`, New: `D:\b`, Err: windows.ERROR_NOT_SAME_DEVICE}
	if !crossDevice(err.Err) {
		t.Error("ERROR_NOT_SAME_DEVICE was not read as crossing volumes, so the copy never runs")
	}
}
