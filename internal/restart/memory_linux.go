package restart

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
)

// readMemory is pid's memory now, false once its status is gone or unreadable.
func readMemory(pid int) (memory, bool) {
	status, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return memory{}, false
	}
	return parseMemory(status)
}

// parseMemory reads VmRSS and VmHWM out of a /proc/<pid>/status; ok is false
// when the status carries neither, as a zombie's does.
func parseMemory(status []byte) (memory, bool) {
	var m memory
	lines := bufio.NewScanner(bytes.NewReader(status))
	for lines.Scan() {
		key, value, found := strings.Cut(lines.Text(), ":")
		if !found {
			continue
		}
		switch key {
		case "VmRSS":
			m.rss = strings.TrimSpace(value)
		case "VmHWM":
			m.hwm = strings.TrimSpace(value)
		}
	}
	return m, m.rss != "" || m.hwm != ""
}
